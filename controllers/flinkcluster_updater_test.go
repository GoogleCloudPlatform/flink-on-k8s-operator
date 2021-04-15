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

	v1beta1 "github.com/googlecloudplatform/flink-operator/api/v1beta1"
	"gotest.tools/assert"
	appsv1 "k8s.io/api/apps/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func TestGetStatefulSetStateNotReady(t *testing.T) {
	var replicas int32 = 3
	var statefulSet = appsv1.StatefulSet{
		Spec:   appsv1.StatefulSetSpec{Replicas: &replicas},
		Status: appsv1.StatefulSetStatus{ReadyReplicas: 2},
	}
	var state = getStatefulSetState(&statefulSet)
	assert.Assert(
		t, state == v1beta1.ComponentStateNotReady)
}

func TestGetStatefulSetStateReady(t *testing.T) {
	var replicas int32 = 3
	var statefulSet = appsv1.StatefulSet{
		Spec:   appsv1.StatefulSetSpec{Replicas: &replicas},
		Status: appsv1.StatefulSetStatus{ReadyReplicas: 3},
	}
	var state = getStatefulSetState(&statefulSet)
	assert.Assert(t, state == v1beta1.ComponentStateReady)
}

func TestIsStatusChangedFalse(t *testing.T) {
	var oldStatus = v1beta1.FlinkClusterStatus{}
	var newStatus = v1beta1.FlinkClusterStatus{}
	var updater = &ClusterStatusUpdater{}
	assert.Assert(t, updater.isStatusChanged(oldStatus, newStatus) == false)
}

func TestIsStatusChangedTrue(t *testing.T) {
	var oldStatus = v1beta1.FlinkClusterStatus{
		Components: v1beta1.FlinkClusterComponentsStatus{
			JobManagerStatefulSet: v1beta1.FlinkClusterComponentState{
				Name:  "my-jobmanager",
				State: "NotReady",
			},
			TaskManagerStatefulSet: v1beta1.TaskManagerStatefulSetStatus{
				Name:  "my-taskmanager",
				State: "NotReady",
			},
			JobManagerService: v1beta1.JobManagerServiceStatus{
				Name:  "my-jobmanager",
				State: "NotReady",
			},
			JobManagerIngress: &v1beta1.JobManagerIngressStatus{
				Name:  "my-jobmanager",
				State: "NotReady",
			},
			Job: &v1beta1.JobStatus{
				Name:  "my-job",
				State: "Pending",
			},
		},
		State: "Creating"}
	var newStatus = v1beta1.FlinkClusterStatus{
		Components: v1beta1.FlinkClusterComponentsStatus{
			JobManagerStatefulSet: v1beta1.FlinkClusterComponentState{
				Name:  "my-jobmanager",
				State: "Ready",
			},
			TaskManagerStatefulSet: v1beta1.TaskManagerStatefulSetStatus{
				Name:  "my-taskmanager",
				State: "Ready",
			},
			JobManagerService: v1beta1.JobManagerServiceStatus{
				Name:  "my-jobmanager",
				State: "Ready",
			},
			JobManagerIngress: &v1beta1.JobManagerIngressStatus{
				Name:  "my-jobmanager",
				State: "Ready",
				URLs:  []string{"http://my-jobmanager"},
			},
			Job: &v1beta1.JobStatus{
				Name:  "my-job",
				State: "Running",
			},
		},
		State: "Creating"}
	var updater = &ClusterStatusUpdater{log: log.Log}
	assert.Assert(t, updater.isStatusChanged(oldStatus, newStatus))
}
