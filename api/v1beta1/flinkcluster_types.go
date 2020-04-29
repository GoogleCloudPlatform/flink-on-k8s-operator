/*
Copyright 2019 Google LLC.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1beta1

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ClusterState defines states for a cluster.
const (
	ClusterStateCreating         = "Creating"
	ClusterStateRunning          = "Running"
	ClusterStateReconciling      = "Reconciling"
	ClusterStateStopping         = "Stopping"
	ClusterStatePartiallyStopped = "PartiallyStopped"
	ClusterStateStopped          = "Stopped"
)

// ComponentState defines states for a cluster component.
const (
	ComponentStateNotReady = "NotReady"
	ComponentStateReady    = "Ready"
	ComponentStateDeleted  = "Deleted"
)

// JobState defines states for a Flink job.
const (
	JobStatePending   = "Pending"
	JobStateRunning   = "Running"
	JobStateSucceeded = "Succeeded"
	JobStateFailed    = "Failed"
	JobStateCancelled = "Cancelled"
	JobStateUnknown   = "Unknown"
)

// AccessScope defines the access scope of JobManager service.
const (
	AccessScopeCluster  = "Cluster"
	AccessScopeVPC      = "VPC"
	AccessScopeExternal = "External"
	AccessScopeNodePort = "NodePort"
)

// JobRestartPolicy defines the restart policy when a job fails.
type JobRestartPolicy = string

const (
	// JobRestartPolicyNever - never restarts a failed job.
	JobRestartPolicyNever = "Never"

	// JobRestartPolicyFromSavepointOnFailure - restart the job from the latest
	// savepoint if available, otherwise do not restart.
	JobRestartPolicyFromSavepointOnFailure = "FromSavepointOnFailure"
)

// User requested control
const (
	// control annotation key
	ControlAnnotation = "flinkclusters.flinkoperator.k8s.io/user-control"

	// control name
	ControlNameCancel    = "job-cancel"
	ControlNameSavepoint = "savepoint"

	// control state
	ControlStateProgressing = "Progressing"
	ControlStateSucceeded   = "Succeeded"
	ControlStateFailed      = "Failed"
)

// ImageSpec defines Flink image of JobManager and TaskManager containers.
type ImageSpec struct {
	// Flink image name.
	Name string `json:"name"`

	// Image pull policy. One of Always, Never, IfNotPresent. Defaults to Always
	// if :latest tag is specified, or IfNotPresent otherwise.
	PullPolicy corev1.PullPolicy `json:"pullPolicy,omitempty"`

	// Secrets for image pull.
	PullSecrets []corev1.LocalObjectReference `json:"pullSecrets,omitempty"`
}

// JobManagerPorts defines ports of JobManager.
type JobManagerPorts struct {
	// RPC port, default: 6123.
	RPC *int32 `json:"rpc,omitempty"`

	// Blob port, default: 6124.
	Blob *int32 `json:"blob,omitempty"`

	// Query port, default: 6125.
	Query *int32 `json:"query,omitempty"`

	// UI port, default: 8081.
	UI *int32 `json:"ui,omitempty"`
}

// JobManagerIngressSpec defines ingress of JobManager
type JobManagerIngressSpec struct {
	// Ingress host format. ex) {{$clusterName}}.example.com
	HostFormat *string `json:"hostFormat,omitempty"`

	// Ingress annotations.
	Annotations map[string]string `json:"annotations,omitempty"`

	// TLS use.
	UseTLS *bool `json:"useTls,omitempty"`

	// TLS secret name.
	TLSSecretName *string `json:"tlsSecretName,omitempty"`
}

// JobManagerSpec defines properties of JobManager.
type JobManagerSpec struct {
	// The number of replicas.
	Replicas *int32 `json:"replicas,omitempty"`

	// Access scope, enum("Cluster", "VPC", "External").
	AccessScope string `json:"accessScope"`

	// (Optional) Ingress.
	Ingress *JobManagerIngressSpec `json:"ingress,omitempty"`

	// Ports.
	Ports JobManagerPorts `json:"ports,omitempty"`

	// Compute resources required by each JobManager container.
	// If omitted, a default value will be used.
	// Cannot be updated.
	// More info: https://kubernetes.io/docs/concepts/configuration/manage-compute-resources-container/
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`

	// TODO: Memory calculation would be change. Let's watch the issue FLINK-13980.

	// Percentage of off-heap memory in containers, as a safety margin to avoid OOM kill, default: 25
	MemoryOffHeapRatio *int32 `json:"memoryOffHeapRatio,omitempty"`

	// Minimum amount of off-heap memory in containers, as a safety margin to avoid OOM kill, default: 600M
	// You can express this value like 600M, 572Mi and 600e6
	// More info: https://kubernetes.io/docs/concepts/configuration/manage-compute-resources-container/#meaning-of-memory
	MemoryOffHeapMin resource.Quantity `json:"memoryOffHeapMin,omitempty"`

	// Volumes in the JobManager pod.
	Volumes []corev1.Volume `json:"volumes,omitempty"`

	// Volume mounts in the JobManager container.
	VolumeMounts []corev1.VolumeMount `json:"volumeMounts,omitempty"`

	// Selector which must match a node's labels for the JobManager pod to be
	// scheduled on that node.
	// More info: https://kubernetes.io/docs/concepts/configuration/assign-pod-node/
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// Sidecar containers running alongside with the JobManager container in the
	// pod.
	Sidecars []corev1.Container `json:"sidecars,omitempty"`
}

// TaskManagerPorts defines ports of TaskManager.
type TaskManagerPorts struct {
	// Data port, default: 6121.
	Data *int32 `json:"data,omitempty"`

	// RPC port, default: 6122.
	RPC *int32 `json:"rpc,omitempty"`

	// Query port.
	Query *int32 `json:"query,omitempty"`
}

// TaskManagerSpec defines properties of TaskManager.
type TaskManagerSpec struct {
	// The number of replicas.
	Replicas int32 `json:"replicas"`

	// Ports.
	Ports TaskManagerPorts `json:"ports,omitempty"`

	// Compute resources required by each TaskManager container.
	// If omitted, a default value will be used.
	// Cannot be updated.
	// More info: https://kubernetes.io/docs/concepts/configuration/manage-compute-resources-container/
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`

	// TODO: Memory calculation would be change. Let's watch the issue FLINK-13980.

	// Percentage of off-heap memory in containers, as a safety margin to avoid OOM kill, default: 25
	MemoryOffHeapRatio *int32 `json:"memoryOffHeapRatio,omitempty"`

	// Minimum amount of off-heap memory in containers, as a safety margin to avoid OOM kill, default: 600M
	// You can express this value like 600M, 572Mi and 600e6
	// More info: https://kubernetes.io/docs/concepts/configuration/manage-compute-resources-container/#meaning-of-memory
	MemoryOffHeapMin resource.Quantity `json:"memoryOffHeapMin,omitempty"`

	// Volumes in the TaskManager pods.
	// More info: https://kubernetes.io/docs/concepts/storage/volumes/
	Volumes []corev1.Volume `json:"volumes,omitempty"`

	// Volume mounts in the TaskManager containers.
	// More info: https://kubernetes.io/docs/concepts/storage/volumes/
	VolumeMounts []corev1.VolumeMount `json:"volumeMounts,omitempty"`

	// Selector which must match a node's labels for the TaskManager pod to be
	// scheduled on that node.
	// More info: https://kubernetes.io/docs/concepts/configuration/assign-pod-node/
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// Sidecar containers running alongside with the TaskManager container in the
	// pod.
	Sidecars []corev1.Container `json:"sidecars,omitempty"`
}

// CleanupAction defines the action to take after job finishes.
type CleanupAction string

const (
	// CleanupActionKeepCluster - keep the entire cluster.
	CleanupActionKeepCluster = "KeepCluster"
	// CleanupActionDeleteCluster - delete the entire cluster.
	CleanupActionDeleteCluster = "DeleteCluster"
	// CleanupActionDeleteTaskManager - delete task manager, keep job manager.
	CleanupActionDeleteTaskManager = "DeleteTaskManager"
)

// CleanupPolicy defines the action to take after job finishes.
type CleanupPolicy struct {
	// Action to take after job succeeds.
	AfterJobSucceeds CleanupAction `json:"afterJobSucceeds,omitempty"`
	// Action to take after job fails.
	AfterJobFails CleanupAction `json:"afterJobFails,omitempty"`
	// Action to take after job is cancelled.
	AfterJobCancelled CleanupAction `json:"afterJobCancelled,omitempty"`
}

// JobSpec defines properties of a Flink job.
type JobSpec struct {
	// JAR file of the job.
	JarFile string `json:"jarFile"`

	// Fully qualified Java class name of the job.
	ClassName *string `json:"className,omitempty"`

	// Args of the job.
	Args []string `json:"args,omitempty"`

	// FromSavepoint where to restore the job from (e.g., gs://my-savepoint/1234).
	FromSavepoint *string `json:"fromSavepoint,omitempty"`

	// Allow non-restored state, default: false.
	AllowNonRestoredState *bool `json:"allowNonRestoredState,omitempty"`

	// Savepoints dir where to store savepoints of the job.
	SavepointsDir *string `json:"savepointsDir,omitempty"`

	// Automatically take a savepoint to the `savepointsDir` every n seconds.
	AutoSavepointSeconds *int32 `json:"autoSavepointSeconds,omitempty"`

	// Update this field to `jobStatus.savepointGeneration + 1` for a running job
	// cluster to trigger a new savepoint to `savepointsDir` on demand.
	SavepointGeneration int32 `json:"savepointGeneration,omitempty"`

	// Job parallelism, default: 1.
	Parallelism *int32 `json:"parallelism,omitempty"`

	// No logging output to STDOUT, default: false.
	NoLoggingToStdout *bool `json:"noLoggingToStdout,omitempty"`

	// Volumes in the Job pod.
	// More info: https://kubernetes.io/docs/concepts/storage/volumes/
	Volumes []corev1.Volume `json:"volumes,omitempty"`

	// Volume mounts in the Job container.
	// More info: https://kubernetes.io/docs/concepts/storage/volumes/
	VolumeMounts []corev1.VolumeMount `json:"volumeMounts,omitempty"`

	// Init containers of the Job pod. A typical use case could be using an init
	// container to download a remote job jar to a local path which is
	// referenced by the `jarFile` property.
	// More info: https://kubernetes.io/docs/concepts/workloads/pods/init-containers/
	InitContainers []corev1.Container `json:"initContainers,omitempty"`

	// Restart policy when the job fails, "Never" or "FromSavepointOnFailure",
	// default: "Never".
	//
	// "Never" means the operator will never try to restart a failed job, manual
	// cleanup and restart is required.
	//
	// "FromSavepointOnFailure" means the operator will try to restart the failed
	// job from the savepoint recorded in the job status if available; otherwise,
	// the job will stay in failed state. This option is usually used together
	// with `autoSavepointSeconds` and `savepointsDir`.
	RestartPolicy *JobRestartPolicy `json:"restartPolicy"`

	// The action to take after job finishes.
	CleanupPolicy *CleanupPolicy `json:"cleanupPolicy,omitempty"`

	// Request the job to be cancelled. Only applies to running jobs. If
	// `savePointsDir` is provided, a savepoint will be taken before stopping the
	// job.
	CancelRequested *bool `json:"cancelRequested,omitempty"`
}

// NativeSessionClusterJobSpec defines properties of a Native Flink session cluster.
// The properties in NativeSessionClusterJobSpec comes from
// https://ci.apache.org/projects/flink/flink-docs-release-1.10/ops/config.html#kubernetes
type NativeSessionClusterJobSpec struct {
	// kubernetes.cluster-id. The cluster id used for identifying the unique flink cluster.
	// We use the Name of flinkCluster.ObjectMeta.Name
	FlinkClusterID string `json:"flinkClusterID,omitempty"`

	// kubernetes.config.file. The kubernetes config file will be used to create the client.
	// The default is located at ~/.kube/config. The sericeaccount in the pod also works.
	KubeConfig *string `json:"kubeConfig,omitempty"`

	// kubernetes.container-start-command-template. Template for the kubernetes jobmanager
	// and taskmanager container start invocation.
	// Default: "%java% %classpath% %jvmmem% %jvmopts% %logging% %class% %args% %redirects%"
	ContainerStartCommandTemplate *string `json:"containerStartCommandTemplate,omitempty"`

	// kubernetes.entry.path. The entrypoint script of kubernetes in the image. It will be used as command for jobmanager and taskmanager container.
	// Default: "/opt/flink/bin/kubernetes-entry.sh"
	EntryPath *string `json:"entryPath,omitempty"`

	// kubernetes.flink.conf.dir. The flink conf directory that will be mounted in pod.
	// The flink-conf.yaml, log4j.properties, logback.xml in this path will be overwritten from config map.
	// Default: "/opt/flink/conf"
	CongfigDir *string `json:"congfigDir,omitempty"`

	// kubernetes.flink.log.dir. The directory that logs of jobmanager and taskmanager be saved in the pod.
	// Default: "/opt/flink/log".
	LogDir *string `json:"logDir,omitempty"`

	// kubernetes.jobmanager.cpu. The number of cpu used by job manager.
	// Default: 1.0
	CPUJobManager *int32 `json:"CPUJobManager,omitempty"`

	// kubernetes.jobmanager.service-account. Service account that is used by jobmanager within kubernetes cluster.
	// The job manager uses this service account when requesting taskmanager pods from the API server.
	// Default: "default"
	FlinkClusterSA *string `json:"flinkClusterSA,omitempty"`

	// kubernetes.rest-service.exposed.type. It could be ClusterIP/NodePort/LoadBalancer(default).
	// When set to ClusterIP, the rest service will not be created.
	// Default: "LoadBalancer"
	FlinkRestServiceType *string `json:"flinkRestServiceType,omitempty"`

	// kubernetes.service.create-timeout. Timeout used for creating the service.
	// The timeout value requires a time-unit specifier (ms/s/min/h/d).
	// Default: "1 min"
	FlinkServiceCreateTimeout *string `json:"flinkServiceCreateTimeout,omitempty"`

	// kubernetes.taskmanager.cpu. The number of cpu used by task manager.
	// By default, the cpu is set to the number of slots per TaskManager.
	// Default: -1.0
	TaskManagerCPU *int32 `json:"taskManagerCPU,omitempty"`
}

// FlinkClusterSpec defines the desired state of FlinkCluster
type FlinkClusterSpec struct {
	// Flink image spec for the cluster's components.
	Image ImageSpec `json:"image"`

	// Flink JobManager spec.
	JobManager JobManagerSpec `json:"jobManager"`

	// Flink TaskManager spec.
	TaskManager TaskManagerSpec `json:"taskManager"`

	// (Optional) Job spec. If specified, this cluster is an ephemeral Job
	// Cluster, which will be automatically terminated after the job finishes;
	// otherwise, it is a long-running Session Cluster.
	Job *JobSpec `json:"job,omitempty"`

	// (Optional) Native Flink session spec. If specified,
	// this cluster is a Native Flink session(only jobmanager created in advanced.)
	NativeSessionClusterJob *NativeSessionClusterJobSpec `json:"nativeSessionClusterJob,omitempty"`

	// Environment variables shared by all JobManager, TaskManager and job
	// containers.
	EnvVars []corev1.EnvVar `json:"envVars,omitempty"`

	// Flink properties which are appened to flink-conf.yaml.
	FlinkProperties map[string]string `json:"flinkProperties,omitempty"`

	// Config for Hadoop.
	HadoopConfig *HadoopConfig `json:"hadoopConfig,omitempty"`

	// Config for GCP.
	GCPConfig *GCPConfig `json:"gcpConfig,omitempty"`
}

// HadoopConfig defines configs for Hadoop.
type HadoopConfig struct {
	// The name of the ConfigMap which contains the Hadoop config files.
	// The ConfigMap must be in the same namespace as the FlinkCluster.
	ConfigMapName string `json:"configMapName,omitempty"`

	// The path where to mount the Volume of the ConfigMap.
	MountPath string `json:"mountPath,omitempty"`
}

// GCPConfig defines configs for GCP.
type GCPConfig struct {
	// GCP service account.
	ServiceAccount *GCPServiceAccount `json:"serviceAccount,omitempty"`
}

// GCPServiceAccount defines the config about GCP service account.
type GCPServiceAccount struct {
	// The name of the Secret holding the GCP service account key file.
	// The Secret must be in the same namespace as the FlinkCluster.
	SecretName string `json:"secretName,omitempty"`

	// The name of the service account key file.
	KeyFile string `json:"keyFile,omitempty"`

	// The path where to mount the Volume of the Secret.
	MountPath string `json:"mountPath,omitempty"`
}

// FlinkClusterComponentState defines the observed state of a component
// of a FlinkCluster.
type FlinkClusterComponentState struct {
	// The resource name of the component.
	Name string `json:"name"`

	// The state of the component.
	State string `json:"state"`
}

// FlinkClusterComponentsStatus defines the observed status of the
// components of a FlinkCluster.
type FlinkClusterComponentsStatus struct {
	// The state of configMap.
	ConfigMap FlinkClusterComponentState `json:"configMap"`

	// The state of JobManager deployment.
	JobManagerDeployment FlinkClusterComponentState `json:"jobManagerDeployment"`

	// The state of JobManager service.
	JobManagerService JobManagerServiceStatus `json:"jobManagerService"`

	// The state of JobManager ingress.
	JobManagerIngress *JobManagerIngressStatus `json:"jobManagerIngress,omitempty"`

	// The state of TaskManager deployment.
	TaskManagerDeployment FlinkClusterComponentState `json:"taskManagerDeployment"`

	// The status of the job, available only when JobSpec is provided.
	Job *JobStatus `json:"job,omitempty"`
}

// Control state
type FlinkClusterControlState struct {
	// Control name
	Name string `json:"name"`

	// Control data
	Details map[string]string `json:"details,omitempty"`

	// State
	State string `json:"state"`

	// Message
	Message string `json:"message,omitempty"`

	// State update time
	UpdateTime string `json:"updateTime"`
}

// JobStatus defines the status of a job.
type JobStatus struct {
	// The name of the Kubernetes job resource.
	Name string `json:"name"`

	// The ID of the Flink job.
	ID string `json:"id"`

	// The state of the Kubernetes job.
	State string `json:"state"`

	// The actual savepoint from which this job started.
	// In case of restart, it might be different from the savepoint in the job
	// spec.
	FromSavepoint string `json:"fromSavepoint,omitempty"`

	// The generation of the savepoint in `savepointsDir` taken by the operator.
	// The value starts from 0 when there is no savepoint and increases by 1 for
	// each successful savepoint.
	SavepointGeneration int32 `json:"savepointGeneration,omitempty"`

	// Savepoint location.
	SavepointLocation string `json:"savepointLocation,omitempty"`

	// Last savepoint trigger ID.
	LastSavepointTriggerID string `json:"lastSavepointTriggerID,omitempty"`

	// Last successful or failed savepoint operation timestamp.
	LastSavepointTime string `json:"lastSavepointTime,omitempty"`

	// The number of restarts.
	RestartCount int32 `json:"restartCount,omitempty"`
}

// JobManagerIngressStatus defines the status of a JobManager ingress.
type JobManagerIngressStatus struct {
	// The name of the Kubernetes ingress resource.
	Name string `json:"name"`

	// The state of the component.
	State string `json:"state"`

	// The URLs of ingress.
	URLs []string `json:"urls,omitempty"`
}

// JobManagerServiceStatus defines the observed state of FlinkCluster
type JobManagerServiceStatus struct {
	// The name of the Kubernetes jobManager service.
	Name string `json:"name"`

	// The state of the component.
	State string `json:"state"`

	// (Optional) The node port, present when `accessScope` is `NodePort`.
	NodePort int32 `json:"nodePort,omitempty"`
}

// FlinkClusterStatus defines the observed state of FlinkCluster
type FlinkClusterStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// The overall state of the Flink cluster.
	State string `json:"state"`

	// The status of the components.
	Components FlinkClusterComponentsStatus `json:"components"`

	// The status of control requested by user
	Control *FlinkClusterControlState `json:"control,omitempty"`

	// Last update timestamp for this status.
	LastUpdateTime string `json:"lastUpdateTime,omitempty"`
}

// +kubebuilder:object:root=true

// FlinkCluster is the Schema for the flinkclusters API
// +kubebuilder:subresource:status
type FlinkCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   FlinkClusterSpec   `json:"spec"`
	Status FlinkClusterStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// FlinkClusterList contains a list of FlinkCluster
type FlinkClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []FlinkCluster `json:"items"`
}

func init() {
	SchemeBuilder.Register(&FlinkCluster{}, &FlinkClusterList{})
}
