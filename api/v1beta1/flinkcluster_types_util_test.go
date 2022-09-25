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
	"gotest.tools/assert"
	"testing"
	"time"
)

func TestIsSavepointUpToDate(t *testing.T) {
	var tc = &TimeConverter{}
	var savepointTime = time.Now()
	var jobEndTime = savepointTime.Add(time.Second * 100)
	var maxStateAgeToRestoreSeconds = int32(300)

	// When maxStateAgeToRestoreSeconds is not provided
	var jobSpec = JobSpec{
		MaxStateAgeToRestoreSeconds: nil,
	}
	var jobStatus = JobStatus{
		SavepointTime:     tc.ToString(savepointTime),
		SavepointLocation: "gs://my-bucket/savepoint-123",
	}
	var update = jobStatus.IsSavepointUpToDate(&jobSpec, jobEndTime)
	assert.Equal(t, update, false)

	// Old savepoint
	savepointTime = time.Now()
	jobEndTime = savepointTime.Add(time.Second * 500)
	jobSpec = JobSpec{
		MaxStateAgeToRestoreSeconds: &maxStateAgeToRestoreSeconds,
	}
	jobStatus = JobStatus{
		SavepointTime:     tc.ToString(savepointTime),
		SavepointLocation: "gs://my-bucket/savepoint-123",
		EndTime:           tc.ToString(jobEndTime),
	}
	update = jobStatus.IsSavepointUpToDate(&jobSpec, jobEndTime)
	assert.Equal(t, update, false)

	// Fails without savepointLocation
	savepointTime = time.Now()
	jobEndTime = savepointTime.Add(time.Second * 100)
	jobSpec = JobSpec{
		MaxStateAgeToRestoreSeconds: &maxStateAgeToRestoreSeconds,
	}
	jobStatus = JobStatus{
		SavepointTime: tc.ToString(savepointTime),
		EndTime:       tc.ToString(jobEndTime),
	}
	update = jobStatus.IsSavepointUpToDate(&jobSpec, jobEndTime)
	assert.Equal(t, update, false)

	// Up-to-date savepoint
	jobEndTime = savepointTime.Add(time.Second * 100)
	jobSpec = JobSpec{
		MaxStateAgeToRestoreSeconds: &maxStateAgeToRestoreSeconds,
	}
	jobStatus = JobStatus{
		SavepointTime:     tc.ToString(savepointTime),
		SavepointLocation: "gs://my-bucket/savepoint-123",
	}
	update = jobStatus.IsSavepointUpToDate(&jobSpec, jobEndTime)
	assert.Equal(t, update, true)

	// A savepoint of the final job state.
	jobSpec = JobSpec{
		MaxStateAgeToRestoreSeconds: &maxStateAgeToRestoreSeconds,
	}
	jobStatus = JobStatus{
		FinalSavepoint: true,
	}
	update = jobStatus.IsSavepointUpToDate(&jobSpec, time.Time{})
	assert.Equal(t, update, true)
}

func TestShouldRestartJob(t *testing.T) {
	var tc = &TimeConverter{}
	var restartOnFailure = JobRestartPolicyFromSavepointOnFailure
	var neverRestart = JobRestartPolicyNever
	var maxStateAgeToRestoreSeconds = int32(300) // 5 min

	// Restart with savepoint up to date
	var savepointTime = time.Now()
	var endTime = savepointTime.Add(time.Second * 60) // savepointTime + 1 min
	var jobSpec = JobSpec{
		RestartPolicy:               &restartOnFailure,
		MaxStateAgeToRestoreSeconds: &maxStateAgeToRestoreSeconds,
	}
	var jobStatus = JobStatus{
		State:             JobStateFailed,
		SavepointLocation: "gs://my-bucket/savepoint-123",
		SavepointTime:     tc.ToString(savepointTime),
		EndTime:           tc.ToString(endTime),
	}
	var restart = jobStatus.ShouldRestart(&jobSpec)
	assert.Equal(t, restart, true)

	// Not restart without savepoint
	jobSpec = JobSpec{
		RestartPolicy:               &restartOnFailure,
		MaxStateAgeToRestoreSeconds: &maxStateAgeToRestoreSeconds,
	}
	jobStatus = JobStatus{
		State:   JobStateFailed,
		EndTime: tc.ToString(endTime),
	}
	restart = jobStatus.ShouldRestart(&jobSpec)
	assert.Equal(t, restart, false)

	// Not restart with restartPolicy Never
	jobSpec = JobSpec{
		RestartPolicy:               &neverRestart,
		MaxStateAgeToRestoreSeconds: &maxStateAgeToRestoreSeconds,
	}
	jobStatus = JobStatus{
		State:             JobStateFailed,
		SavepointLocation: "gs://my-bucket/savepoint-123",
		SavepointTime:     tc.ToString(savepointTime),
		EndTime:           tc.ToString(endTime),
	}
	restart = jobStatus.ShouldRestart(&jobSpec)
	assert.Equal(t, restart, false)

	// Not restart with old savepoint
	endTime = savepointTime.Add(time.Second * 300) // savepointTime + 5 min
	jobSpec = JobSpec{
		RestartPolicy:               &neverRestart,
		MaxStateAgeToRestoreSeconds: &maxStateAgeToRestoreSeconds,
	}
	jobStatus = JobStatus{
		State:             JobStateFailed,
		SavepointLocation: "gs://my-bucket/savepoint-123",
		SavepointTime:     tc.ToString(savepointTime),
		EndTime:           tc.ToString(endTime),
	}
	restart = jobStatus.ShouldRestart(&jobSpec)
	assert.Equal(t, restart, false)
}
