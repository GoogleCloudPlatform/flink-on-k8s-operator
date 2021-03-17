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

package flinkclient

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-logr/logr"
)

const (
	savepointStateInProgress = "IN_PROGRESS"
	savepointStateCompleted  = "COMPLETED"
)

// FlinkClient - Flink API client.
type FlinkClient struct {
	Log        logr.Logger
	HTTPClient HTTPClient
}

// JobStatus defines Flink job status.
type JobStatus struct {
	ID     string
	Status string
}

// JobStatusList defines Flink job status list.
type JobStatusList struct {
	Jobs []JobStatus
}

// SavepointTriggerID defines trigger ID of an async savepoint operation.
type SavepointTriggerID struct {
	RequestID string `json:"request-id"`
}

// SavepointFailureCause defines the cause of savepoint failure.
type SavepointFailureCause struct {
	ExceptionClass string `json:"class"`
	StackTrace     string `json:"stack-trace"`
}

// SavepointStateID - enum("IN_PROGRESS", "COMPLETED").
type SavepointStateID struct {
	ID string `json:"id"`
}

// SavepointStatus defines savepoint status of a job.
type SavepointStatus struct {
	// Flink job ID.
	JobID string
	// Savepoint operation trigger ID.
	TriggerID string
	// Completed or not.
	Completed bool
	// Savepoint location URI, non-empty when savepoint succeeded.
	Location string
	// Cause of the failure, non-empyt when savepoint failed
	FailureCause SavepointFailureCause
}

func (s *SavepointStatus) IsSuccessful() bool {
	return s.Completed && s.FailureCause.StackTrace == ""
}

func (s *SavepointStatus) IsFailed() bool {
	return s.Completed && s.FailureCause.StackTrace != ""
}

// GetJobStatusList gets Flink job status list.
func (c *FlinkClient) GetJobStatusList(
	apiBaseURL string, jobStatusList *JobStatusList) error {
	return c.HTTPClient.Get(apiBaseURL+"/jobs", jobStatusList)
}

// StopJob stops a job.
func (c *FlinkClient) StopJob(
	apiBaseURL string, jobID string) error {
	var resp = struct{}{}
	return c.HTTPClient.Patch(
		fmt.Sprintf("%s/jobs/%s?mode=cancel", apiBaseURL, jobID), []byte{}, &resp)
}

// TriggerSavepoint triggers an async savepoint operation.
func (c *FlinkClient) TriggerSavepoint(
	apiBaseURL string, jobID string, dir string, cancel bool) (SavepointTriggerID, error) {
	var url = fmt.Sprintf("%s/jobs/%s/savepoints", apiBaseURL, jobID)
	var jsonStr = fmt.Sprintf(`{
		"target-directory" : "%s",
		"cancel-job" : %v
	}`, dir, cancel)
	var triggerID = SavepointTriggerID{}
	var err = c.HTTPClient.Post(url, []byte(jsonStr), &triggerID)
	return triggerID, err
}

// TakeSavepoint takes savepoint, blocks until it succeeds or fails.
func (c *FlinkClient) TakeSavepoint(
	apiBaseURL string, jobID string, dir string) (SavepointStatus, error) {
	var triggerID = SavepointTriggerID{}
	var status = SavepointStatus{JobID: jobID}
	var err error

	triggerID, err = c.TriggerSavepoint(apiBaseURL, jobID, dir, false)
	if err != nil {
		return SavepointStatus{}, err
	}

	for i := 0; i < 12; i++ {
		status, err = c.GetSavepointStatus(apiBaseURL, jobID, triggerID.RequestID)
		if err == nil && status.Completed {
			return status, nil
		}
		time.Sleep(5 * time.Second)
	}

	return status, err
}

// GetSavepointStatus returns savepoint status.
//
// Flink API response examples:
//
// 1) success:
//
// {
//    "status":{"id":"COMPLETED"},
//    "operation":{
//      "location":"file:/tmp/savepoint-ad4025-dd46c1bd1c80"
//    }
// }
//
// 2) failure:
//
// {
//    "status":{"id":"COMPLETED"},
//    "operation":{
//      "failure-cause":{
//        "class": "java.util.concurrent.CompletionException",
//        "stack-trace": "..."
//      }
//    }
// }
func (c *FlinkClient) GetSavepointStatus(
	apiBaseURL string, jobID string, triggerID string) (SavepointStatus, error) {
	var url = fmt.Sprintf(
		"%s/jobs/%s/savepoints/%s", apiBaseURL, jobID, triggerID)
	var status = SavepointStatus{JobID: jobID, TriggerID: triggerID}
	var rootJSON map[string]*json.RawMessage
	var stateID SavepointStateID
	var opJSON map[string]*json.RawMessage
	var err = c.HTTPClient.Get(url, &rootJSON)
	if err != nil {
		return status, err
	}
	c.Log.Info("Savepoint status json", "json", rootJSON)
	if state, ok := rootJSON["status"]; ok && state != nil {
		err = json.Unmarshal(*state, &stateID)
		if err != nil {
			return status, err
		}
		if stateID.ID == savepointStateCompleted {
			status.Completed = true
		} else {
			status.Completed = false
		}
	}
	if op, ok := rootJSON["operation"]; ok && op != nil {
		err = json.Unmarshal(*op, &opJSON)
		if err != nil {
			return status, err
		}
		// Success
		if location, ok := opJSON["location"]; ok && location != nil {
			err = json.Unmarshal(*location, &status.Location)
			if err != nil {
				return status, err
			}
		}
		// Failure
		if failureCause, ok := opJSON["failure-cause"]; ok && failureCause != nil {
			err = json.Unmarshal(*failureCause, &status.FailureCause)
			if err != nil {
				return status, err
			}
		}
	}
	return status, err
}
