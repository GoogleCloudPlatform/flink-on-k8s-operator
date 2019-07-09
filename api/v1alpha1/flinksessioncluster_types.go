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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	SessionClusterStateReconciling = "Reconciling"
	SessionClusterStateRunning     = "Running"
)

var (
	SessionClusterComponentStatusNotReady = "Not Ready"
	SessionClusterComponentStatusReady    = "Ready"
)

// ImageSpec defines Flink image of JobManager and TaskManager containers.
type ImageSpec struct {
	// Flink image URI.
	URI *string `json:"uri,omitempty"`

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

	// Ports.
	Ports JobManagerPorts `json:"ports,omitempty"`
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
	Replicas *int32 `json:"replicas,omitempty"`

	// Ports.
	Ports TaskManagerPorts `json:"ports,omitempty"`
}

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// FlinkSessionClusterSpec defines the desired state of FlinkSessionCluster
type FlinkSessionClusterSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// The name of the Flink cluster.
	Name string `json:"name"`

	// Flink image spec.
	ImageSpec ImageSpec `json:"imageSpec"`

	// Flink JobManager spec.
	JobManagerSpec JobManagerSpec `json:"jobManagerSpec"`

	// Flink TaskManager spec.
	TaskManagerSpec TaskManagerSpec `json:"taskManagerSpec"`
}

// FlinkSessionClusterComponentState defines the observed state of a component
// of a FlinkSessionCluster.
type FlinkSessionClusterComponentState struct {
	// The resource name of the component.
	Name string `json:"name"`

	// The state of the component.
	State string `json:"state"`
}

// FlinkSessionClusterComponentsStatus defines the observed status of the
// components of a FlinkSessionCluster.
type FlinkSessionClusterComponentsStatus struct {
	// The state of JobManager deployment.
	JobManagerDeployment FlinkSessionClusterComponentState `json:"jobManagerDeployment"`

	// The state of JobManager service.
	JobManagerService FlinkSessionClusterComponentState `json:"jobManagerService"`

	// The state of TaskManager deployment.
	TaskManagerDeployment FlinkSessionClusterComponentState `json:"taskManagerDeployment"`
}

// FlinkSessionClusterStatus defines the observed state of FlinkSessionCluster
type FlinkSessionClusterStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// The overall state of the Flink cluster.
	State string `json:"state"`

	// The status of the components.
	Components FlinkSessionClusterComponentsStatus `json:"components"`

	// Last update timestamp for this status.
	LastUpdateTime string `json:"lastUpdateTime,omitempty"`
}

// +kubebuilder:object:root=true

// FlinkSessionCluster is the Schema for the flinksessionclusters API
type FlinkSessionCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   FlinkSessionClusterSpec   `json:"spec"`
	Status FlinkSessionClusterStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// FlinkSessionClusterList contains a list of FlinkSessionCluster
type FlinkSessionClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []FlinkSessionCluster `json:"items"`
}

func init() {
	SchemeBuilder.Register(&FlinkSessionCluster{}, &FlinkSessionClusterList{})
}
