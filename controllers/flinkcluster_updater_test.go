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

package controllers

import (
	"testing"

	v1alpha1 "github.com/googlecloudplatform/flink-operator/api/v1alpha1"
	"gotest.tools/assert"
	appsv1 "k8s.io/api/apps/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func TestGetDeploymentStateNotReady(t *testing.T) {
	var replicas int32 = 3
	var deployment = appsv1.Deployment{
		Spec:   appsv1.DeploymentSpec{Replicas: &replicas},
		Status: appsv1.DeploymentStatus{AvailableReplicas: 2},
	}
	var state = getDeploymentState(&deployment)
	assert.Assert(
		t, state == v1alpha1.ComponentState.NotReady)
}

func TestGetDeploymentStateReady(t *testing.T) {
	var replicas int32 = 3
	var deployment = appsv1.Deployment{
		Spec:   appsv1.DeploymentSpec{Replicas: &replicas},
		Status: appsv1.DeploymentStatus{AvailableReplicas: 3},
	}
	var state = getDeploymentState(&deployment)
	assert.Assert(t, state == v1alpha1.ComponentState.Ready)
}

func TestIsStatusChangedFalse(t *testing.T) {
	var oldStatus = v1alpha1.FlinkClusterStatus{}
	var newStatus = v1alpha1.FlinkClusterStatus{}
	var updater = &ClusterStatusUpdater{}
	assert.Assert(t, updater.isStatusChanged(oldStatus, newStatus) == false)
}

func TestIsStatusChangedTrue(t *testing.T) {
	var oldStatus = v1alpha1.FlinkClusterStatus{
		Components: v1alpha1.FlinkClusterComponentsStatus{
			JobManagerDeployment: v1alpha1.FlinkClusterComponentState{
				Name:  "my-jobmanager",
				State: "NotReady",
			},
			TaskManagerDeployment: v1alpha1.FlinkClusterComponentState{
				Name:  "my-taskmanager",
				State: "NotReady",
			},
			JobManagerService: v1alpha1.FlinkClusterComponentState{
				Name:  "my-jobmanager",
				State: "NotReady",
			},
			JobManagerIngress: &v1alpha1.JobManagerIngressStatus{
				Name:  "my-jobmanager",
				State: "NotReady",
			},
			Job: &v1alpha1.JobStatus{
				Name:  "my-job",
				State: "Pending",
			},
		},
		State: "Creating"}
	var newStatus = v1alpha1.FlinkClusterStatus{
		Components: v1alpha1.FlinkClusterComponentsStatus{
			JobManagerDeployment: v1alpha1.FlinkClusterComponentState{
				Name:  "my-jobmanager",
				State: "Ready",
			},
			TaskManagerDeployment: v1alpha1.FlinkClusterComponentState{
				Name:  "my-taskmanager",
				State: "Ready",
			},
			JobManagerService: v1alpha1.FlinkClusterComponentState{
				Name:  "my-jobmanager",
				State: "Ready",
			},
			JobManagerIngress: &v1alpha1.JobManagerIngressStatus{
				Name:  "my-jobmanager",
				State: "Ready",
				URLs:  []string{"http://my-jobmanager"},
			},
			Job: &v1alpha1.JobStatus{
				Name:  "my-job",
				State: "Running",
			},
		},
		State: "Creating"}
	var updater = &ClusterStatusUpdater{log: log.Log}
	assert.Assert(t, updater.isStatusChanged(oldStatus, newStatus))
}
