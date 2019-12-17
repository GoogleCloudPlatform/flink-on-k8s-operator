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
)

func TestTimeConverter(t *testing.T) {
	var tc = &TimeConverter{}

	var str1 = "2019-10-23T05:10:36Z"
	var tm1 = tc.FromString(str1)
	var str2 = tc.ToString(tm1)
	assert.Assert(t, str1 == str2)

	var str3 = "2019-10-24T09:57:18+09:00"
	var tm2 = tc.FromString(str3)
	var str4 = tc.ToString(tm2)
	assert.Assert(t, str3 == str4)
}

func TestShouldRestartJob(t *testing.T) {
	var restartOnFailure = v1alpha1.JobRestartPolicyFromSavepointOnFailure
	var jobStatus1 = v1alpha1.JobStatus{
		State:             v1alpha1.JobState.Failed,
		SavepointLocation: "gs://my-bucket/savepoint-123",
	}
	var restart1 = shouldRestartJob(&restartOnFailure, &jobStatus1)
	assert.Equal(t, restart1, true)

	var jobStatus2 = v1alpha1.JobStatus{
		State: v1alpha1.JobState.Failed,
	}
	var restart2 = shouldRestartJob(&restartOnFailure, &jobStatus2)
	assert.Equal(t, restart2, false)

	var neverRestart = v1alpha1.JobRestartPolicyNever
	var jobStatus3 = v1alpha1.JobStatus{
		State:             v1alpha1.JobState.Failed,
		SavepointLocation: "gs://my-bucket/savepoint-123",
	}
	var restart3 = shouldRestartJob(&neverRestart, &jobStatus3)
	assert.Equal(t, restart3, false)
}
