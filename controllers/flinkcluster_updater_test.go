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

	flinkoperatorv1alpha1 "github.com/googlecloudplatform/flink-operator/api/v1alpha1"
	"gotest.tools/assert"
	appsv1 "k8s.io/api/apps/v1"
)

func TestGetDeploymentStateNotReady(t *testing.T) {
	var replicas int32 = 3
	var deployment = appsv1.Deployment{
		Spec:   appsv1.DeploymentSpec{Replicas: &replicas},
		Status: appsv1.DeploymentStatus{AvailableReplicas: 2},
	}
	var state = getDeploymentState(&deployment)
	assert.Assert(
		t, state == flinkoperatorv1alpha1.ClusterComponentState.NotReady)
}

func TestGetDeploymentStateReady(t *testing.T) {
	var replicas int32 = 3
	var deployment = appsv1.Deployment{
		Spec:   appsv1.DeploymentSpec{Replicas: &replicas},
		Status: appsv1.DeploymentStatus{AvailableReplicas: 3},
	}
	var state = getDeploymentState(&deployment)
	assert.Assert(t, state == flinkoperatorv1alpha1.ClusterComponentState.Ready)
}
