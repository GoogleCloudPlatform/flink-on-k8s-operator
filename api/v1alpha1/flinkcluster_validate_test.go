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
	"testing"

	"gotest.tools/assert"
)

// Tests updating status is allowed.
func TestUpdateStatusAllowed(t *testing.T) {
	var oldCluster = FlinkCluster{Status: FlinkClusterStatus{State: "NoReady"}}
	var newCluster = FlinkCluster{Status: FlinkClusterStatus{State: "Running"}}
	var err = validateUpdate(&oldCluster, &newCluster)
	assert.NilError(t, err, "updating status failed unexpectedly")
}

// Tests updating spec is not allowed.
func TestUpdateSpecNotAllowed(t *testing.T) {
	var oldCluster = FlinkCluster{Spec: FlinkClusterSpec{Image: ImageSpec{Name: "flink:1.8.1"}}}
	var newCluster = FlinkCluster{Spec: FlinkClusterSpec{Image: ImageSpec{Name: "flink:1.9.0"}}}
	var err = validateUpdate(&oldCluster, &newCluster)
	var expectedErr = "updating FlinkCluster spec is not allowed," +
		" please delete the resouce and recreate"
	assert.Equal(t, err.Error(), expectedErr)
}
