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

// ClusterComponentState defines states for a cluster component.
var ClusterComponentState = struct {
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
	Running   string
	Succeeded string
	Failed    string
	Unknown   string
}{
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

	// Flink image pull policy.
	PullPolicy *string `json:"pullPolicy,omitempty"`
}

// JobManagerPorts defines ports of JobManager.
type JobManagerPorts struct {
	// RPC port.
	RPC *int32 `json:"rpc,omitempty"`

	// Blob port.
	Blob *int32 `json:"blob,omitempty"`

	// Query port.
	Query *int32 `json:"query,omitempty"`

	// UI port.
	UI *int32 `json:"ui,omitempty"`
}

// JobManagerSpec defines properties of JobManager.
type JobManagerSpec struct {
	// The number of replicas.
	Replicas *int32 `json:"replicas,omitempty"`

	// Access scope, enum("Cluster", "VPC", "External").
	AccessScope string `json:"accessScope"`

	// Ports.
	Ports JobManagerPorts `json:"ports,omitempty"`

	// Compute resources required by each JobManager container.
	// If omitted, a default value will be used.
	// Cannot be updated.
	// More info: https://kubernetes.io/docs/concepts/configuration/manage-compute-resources-container/
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`
}

// TaskManagerPorts defines ports of TaskManager.
type TaskManagerPorts struct {
	// Data port.
	Data *int32 `json:"data,omitempty"`

	// RPC port.
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

	// Allow non-restored state.
	AllowNonRestoredState *bool `json:"allowNonRestoredState,omitempty"`

	// Job parallelism.
	Parallelism *int32 `json:"parallelism,omitempty"`

	// No logging output to STDOUT.
	NoLoggingToStdout *bool `json:"noLoggingToStdout,omitempty"`

	// Restart policy, "OnFailure" or "Never".
	RestartPolicy string `json:"restartPolicy"`

	// TODO(dagang): support volumes and volumeMounts.
}

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// FlinkClusterSpec defines the desired state of FlinkCluster
type FlinkClusterSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Flink image spec for the cluster's components.
	ImageSpec ImageSpec `json:"image"`

	// Flink JobManager spec.
	JobManagerSpec JobManagerSpec `json:"jobManager"`

	// Flink TaskManager spec.
	TaskManagerSpec TaskManagerSpec `json:"taskManager"`

	// Optional job spec. If specified, this cluster is an ephemeral Job
	// Cluster, which will be automatically terminated after the job finishes;
	// otherwise, it is a long-running Session Cluster.
	JobSpec *JobSpec `json:"job,omitempty"`
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
	// The state of JobManager deployment.
	JobManagerDeployment FlinkClusterComponentState `json:"jobManagerDeployment"`

	// The state of JobManager service.
	JobManagerService FlinkClusterComponentState `json:"jobManagerService"`

	// The state of TaskManager deployment.
	TaskManagerDeployment FlinkClusterComponentState `json:"taskManagerDeployment"`

	// The status of the job, available only when JobSpec is provided.
	Job *JobStatus `json:"job,omitempty"`
}

// JobStatus defines the status of a job.
type JobStatus struct {
	// The name of the job resource.
	Name string `json:"name"`

	// The state of the job.
	State string `json:"state"`
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
