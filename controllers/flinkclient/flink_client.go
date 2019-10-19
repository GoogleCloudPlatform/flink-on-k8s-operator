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
	State        string
	FailureCause SavepointFailureCause
}

// GetJobStatusList gets Flink job status list.
func (c *FlinkClient) GetJobStatusList(
	apiBaseURL string, jobStatusList *JobStatusList) error {
	return c.HTTPClient.Get(apiBaseURL+"/jobs", jobStatusList)
}

// TriggerSavepoint triggers an async savepoint operation.
func (c *FlinkClient) TriggerSavepoint(
	apiBaseURL string, jobID string, dir string) (SavepointTriggerID, error) {
	var url = fmt.Sprintf("%s/jobs/%s/savepoints", apiBaseURL, jobID)
	var jsonStr = fmt.Sprintf(`{
		"target-directory" : "%s",
		"cancel-job" : false
	}`, dir)
	var triggerID = SavepointTriggerID{}
	var err = c.HTTPClient.Post(url, []byte(jsonStr), &triggerID)
	return triggerID, err
}

// GetSavepointStatus returns savepoint status.
func (c *FlinkClient) GetSavepointStatus(
	apiBaseURL string, jobID string, triggerID string) (SavepointStatus, error) {
	var url = fmt.Sprintf(
		"%s/jobs/%s/savepoints/%s", apiBaseURL, jobID, triggerID)
	var status = SavepointStatus{}
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
		status.State = stateID.ID
	}
	if op, ok := rootJSON["operation"]; ok && op != nil {
		err = json.Unmarshal(*op, &opJSON)
		if err != nil {
			return status, err
		}
		if failureCause, ok := opJSON["failure-cause"]; ok && failureCause != nil {
			err = json.Unmarshal(*failureCause, &status.FailureCause)
			if err != nil {
				return status, err
			}
		}
	}
	return status, err
}

// TakeSavepoint takes savepoint, blocks until it suceeds or fails.
func (c *FlinkClient) TakeSavepoint(
	apiBaseURL string, jobID string, dir string) (
	SavepointTriggerID, SavepointStatus, error) {
	var triggerID = SavepointTriggerID{}
	var status = SavepointStatus{}
	var err error

	triggerID, err = c.TriggerSavepoint(apiBaseURL, jobID, dir)
	if err != nil {
		return triggerID, SavepointStatus{}, err
	}

	for i := 0; i < 10; i = i + 1 {
		status, err = c.GetSavepointStatus(apiBaseURL, jobID, triggerID.RequestID)
		if err == nil && status.State == savepointStateCompleted {
			return triggerID, status, nil
		}
		time.Sleep(5 * time.Second)
	}

	return triggerID, status, err
}
