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

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ClusterState defines states for a cluster.
var ClusterState = struct {
	Creating    string
	Running     string
	Reconciling string
	Stopping    string
	Stopped     string
}{
	Creating:    "Creating",
	Running:     "Running",
	Reconciling: "Reconciling",
	Stopping:    "Stopping",
	Stopped:     "Stopped",
}

// ComponentState defines states for a cluster component.
var ComponentState = struct {
	NotReady string
	Ready    string
	Deleted  string
}{
	NotReady: "NotReady",
	Ready:    "Ready",
	Deleted:  "Deleted",
}

// JobState defines states for a Flink job.
var JobState = struct {
	Pending   string
	Running   string
	Succeeded string
	Failed    string
	Unknown   string
}{
	Pending:   "Pending",
	Running:   "Running",
	Succeeded: "Succeeded",
	Failed:    "Failed",
	Unknown:   "Unknown",
}

// JobRestartPolicy defines the policy for job restart.
var JobRestartPolicy = struct {
	OnFailure string
	Never     string
}{
	OnFailure: "OnFailure",
	Never:     "Never",
}

// AccessScope defines the access scope of JobManager service.
var AccessScope = struct {
	Cluster  string
	VPC      string
	External string
}{
	Cluster:  "Cluster",
	VPC:      "VPC",
	External: "External",
}

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

	// Volumes in the JobManager pod.
	Volumes []corev1.Volume `json:"volumes,omitempty"`

	// Volume mounts in the JobManager container.
	Mounts []corev1.VolumeMount `json:"mounts,omitempty"`

	// Selector which must match a node's labels for the JobManager pod to be
	// scheduled on that node.
	// More info: https://kubernetes.io/docs/concepts/configuration/assign-pod-node/
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`
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

	// Volumes in the TaskManager pods.
	Volumes []corev1.Volume `json:"volumes,omitempty"`

	// Volume mounts in the TaskManager containers.
	Mounts []corev1.VolumeMount `json:"mounts,omitempty"`

	// Selector which must match a node's labels for the TaskManager pod to be
	// scheduled on that node.
	// More info: https://kubernetes.io/docs/concepts/configuration/assign-pod-node/
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// Sidecar containers running alongside with the TaskManager container in the
	// pod.
	Sidecars []corev1.Container `json:"sidecars,omitempty"`
}

// JobSpec defines properties of a Flink job.
type JobSpec struct {
	// JAR file of the job.
	JarFile string `json:"jarFile"`

	// Fully qualified Java class name of the job.
	ClassName *string `json:"className,omitempty"`

	// Args of the job.
	Args []string `json:"args,omitempty"`

	// Savepoint where to restore the job from (e.g., gs://my-savepoint/1234).
	Savepoint *string `json:"savepoint,omitempty"`

	// Allow non-restored state, default: false.
	AllowNonRestoredState *bool `json:"allowNonRestoredState,omitempty"`

	// Automatically take a savepoint to the savepoints dir every n seconds.
	AutoSavepointSeconds *int32 `json:"autoSavepointSeconds,omitempty"`

	// Savepoints dir where to store automatically taken savepoints.
	SavepointsDir *string `json:"savepointsDir,omitempty"`

	// Job parallelism, default: 1.
	Parallelism *int32 `json:"parallelism,omitempty"`

	// No logging output to STDOUT, default: false.
	NoLoggingToStdout *bool `json:"noLoggingToStdout,omitempty"`

	// Restart policy, "OnFailure" or "Never", default: "OnFailure".
	RestartPolicy *corev1.RestartPolicy `json:"restartPolicy"`

	// Volumes in the Job pod.
	Volumes []corev1.Volume `json:"volumes,omitempty"`

	// Volume mounts in the Job container.
	Mounts []corev1.VolumeMount `json:"mounts,omitempty"`
}

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// FlinkClusterSpec defines the desired state of FlinkCluster
type FlinkClusterSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Flink image spec for the cluster's components.
	Image ImageSpec `json:"image"`

	// Flink JobManager spec.
	JobManager JobManagerSpec `json:"jobManager"`

	// Flink TaskManager spec.
	TaskManager TaskManagerSpec `json:"taskManager"`

	// Optional job spec. If specified, this cluster is an ephemeral Job
	// Cluster, which will be automatically terminated after the job finishes;
	// otherwise, it is a long-running Session Cluster.
	Job *JobSpec `json:"job,omitempty"`

	// Flink properties which are appened to flink-conf.yaml of the image.
	FlinkProperties map[string]string `json:"flinkProperties,omitempty"`

	// Environment variables shared by all JobManager, TaskManager and job
	// containers.
	EnvVars []corev1.EnvVar `json:"envVars,omitempty"`
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
	JobManagerService FlinkClusterComponentState `json:"jobManagerService"`

	// The state of JobManager ingress.
	JobManagerIngress *JobManagerIngressStatus `json:"jobManagerIngress,omitempty"`

	// The state of TaskManager deployment.
	TaskManagerDeployment FlinkClusterComponentState `json:"taskManagerDeployment"`

	// The status of the job, available only when JobSpec is provided.
	Job *JobStatus `json:"job,omitempty"`
}

// JobStatus defines the status of a job.
type JobStatus struct {
	// The name of the Kubernetes job resource.
	Name string `json:"name"`

	// The ID of the Flink job.
	ID string `json:"id"`

	// The state of the Kubernetes job.
	State string `json:"state"`

	// Savepoint location.
	SavepointLocation string `json:"savepointLocation,omitempty"`

	// Last savepoint trigger ID.
	LastSavepointTriggerID string `json:"lastSavepointTriggerID,omitempty"`

	// Last successful or failed savepoint operation timestamp.
	LastSavepointTime string `json:"lastSavepointTime,omitempty"`
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

// FlinkClusterStatus defines the observed state of FlinkCluster
type FlinkClusterStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// The overall state of the Flink cluster.
	State string `json:"state"`

	// The status of the components.
	Components FlinkClusterComponentsStatus `json:"components"`

	// Last update timestamp for this status.
	LastUpdateTime string `json:"lastUpdateTime,omitempty"`
}

// +kubebuilder:object:root=true

// FlinkCluster is the Schema for the flinkclusters API
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
