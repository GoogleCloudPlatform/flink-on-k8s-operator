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
	"fmt"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"strconv"
	"time"

	v1beta1 "github.com/googlecloudplatform/flink-operator/api/v1beta1"
	batchv1 "k8s.io/api/batch/v1"
)

const (
	ControlSavepointState       = "savepointState"
	ControlSavepointTriggerID   = "SavepointTriggerID"
	ControlSavepointTriggerTime = "SavepointTriggerTime"
	ControlJobID                = "jobID"
	ControlRetries              = "retries"

	ControlMaxRetries = "3"

	SavepointStateProgressing   = "Progressing"
	SavepointStateTriggerFailed = "TriggerFailed"
	SavepointStateFailed        = "Failed"
	SavepointStateSucceeded     = "Succeeded"

	SavepointTimeoutSec = 60
)

var FlinkAPIRetryBackoff = wait.Backoff{
	Duration: 1 * time.Second,
	Factor:   2,
	Steps:    4,
}

func getFlinkAPIBaseURL(cluster *v1beta1.FlinkCluster) string {
	return fmt.Sprintf(
		"http://%s.%s.svc.cluster.local:%d",
		getJobManagerServiceName(cluster.ObjectMeta.Name),
		cluster.ObjectMeta.Namespace,
		*cluster.Spec.JobManager.Ports.UI)
}

// Gets JobManager ingress name
func getConfigMapName(clusterName string) string {
	return clusterName + "-configmap"
}

// Gets JobManager deployment name
func getJobManagerDeploymentName(clusterName string) string {
	return clusterName + "-jobmanager"
}

// Gets JobManager service name
func getJobManagerServiceName(clusterName string) string {
	return clusterName + "-jobmanager"
}

// Gets JobManager ingress name
func getJobManagerIngressName(clusterName string) string {
	return clusterName + "-jobmanager"
}

// Gets TaskManager name
func getTaskManagerDeploymentName(clusterName string) string {
	return clusterName + "-taskmanager"
}

// Gets Job name
func getJobName(clusterName string) string {
	return clusterName + "-job"
}

// TimeConverter converts between time.Time and string.
type TimeConverter struct{}

// FromString converts string to time.Time.
func (tc *TimeConverter) FromString(timeStr string) time.Time {
	timestamp, err := time.Parse(
		time.RFC3339, timeStr)
	if err != nil {
		panic(fmt.Sprintf("Failed to parse time string: %s", timeStr))
	}
	return timestamp
}

// ToString converts time.Time to string.
func (tc *TimeConverter) ToString(timestamp time.Time) string {
	return timestamp.Format(time.RFC3339)
}

// setTimestamp sets the current timestamp to the target.
func setTimestamp(target *string) {
	var tc = &TimeConverter{}
	var now = time.Now()
	*target = tc.ToString(now)
}

// shouldRestartJob returns true if the controller should restart the failed
// job.
func shouldRestartJob(
	restartPolicy *v1beta1.JobRestartPolicy,
	jobStatus *v1beta1.JobStatus) bool {
	return restartPolicy != nil &&
		*restartPolicy == v1beta1.JobRestartPolicyFromSavepointOnFailure &&
		jobStatus != nil &&
		jobStatus.State == v1beta1.JobStateFailed &&
		len(jobStatus.SavepointLocation) > 0
}

func getFromSavepoint(jobSpec batchv1.JobSpec) string {
	var jobArgs = jobSpec.Template.Spec.Containers[0].Args
	for i, arg := range jobArgs {
		if arg == "--fromSavepoint" && i < len(jobArgs)-1 {
			return jobArgs[i+1]
		}
	}
	return ""
}

func getRetryCount(data map[string]string) (string, error) {
	var err error
	var retries, ok = data["retries"]
	if ok {
		retryCount, err := strconv.Atoi(retries)
		if err == nil {
			retryCount++
			retries = strconv.Itoa(retryCount)
		}
	} else {
		retries = "1"
	}
	return retries, err
}

func getNewUserControlStatus(controlName string) *v1beta1.FlinkClusterControlStatus {
	var controlStatus = new(v1beta1.FlinkClusterControlStatus)
	controlStatus.Name = controlName
	controlStatus.State = v1beta1.ControlStateProgressing
	setTimestamp(&controlStatus.UpdateTime)
	return controlStatus
}

func getSavepointStatus(jobID string, triggerID string, triggerSuccess bool) v1beta1.SavepointStatus {
	var savepointStatus = v1beta1.SavepointStatus{}
	var now string
	setTimestamp(&now)
	savepointStatus.JobID = jobID
	savepointStatus.TriggerID = triggerID
	savepointStatus.TriggerTime = now
	if triggerSuccess {
		savepointStatus.State = SavepointStateProgressing
	} else {
		savepointStatus.State = SavepointStateTriggerFailed
	}
	return savepointStatus
}

// Ported from master branch of k8s.io/client-go/util/retry/util.go
// We can replace this with future release of k8s.io/client-go
func retryOnError(backoff wait.Backoff, retriable func(error) bool, fn func() error) error {
	var lastErr error
	err := wait.ExponentialBackoff(backoff, func() (bool, error) {
		err := fn()
		switch {
		case err == nil:
			return true, nil
		case retriable(err):
			lastErr = err
			return false, nil
		default:
			return false, err
		}
	})
	if err == wait.ErrWaitTimeout {
		err = lastErr
	}
	return err
}

func savepointTimeout(s *v1beta1.SavepointStatus) bool {
	if s.TriggerTime == "" {
		return false
	}
	tc := &TimeConverter{}
	triggerTime := tc.FromString(s.TriggerTime)
	validTime := triggerTime.Add(time.Duration(int64(SavepointTimeoutSec) * int64(time.Second)))
	return time.Now().After(validTime)
}

func getControlEvent(status v1beta1.FlinkClusterControlStatus) (eventType string, eventReason string, eventMessage string) {
	switch status.State {
	case v1beta1.ControlStateProgressing:
		eventType = corev1.EventTypeNormal
		eventReason = "ControlRequested"
		eventMessage = fmt.Sprintf("Requested new user control %v", status.Name)
	case v1beta1.ControlStateSucceeded:
		eventType = corev1.EventTypeNormal
		eventReason = "ControlSucceeded"
		eventMessage = fmt.Sprintf("Succesfully completed user control %v", status.Name)
	case v1beta1.ControlStateFailed:
		eventType = corev1.EventTypeWarning
		eventReason = "ControlFailed"
		eventMessage = fmt.Sprintf("User control %v failed", status.Name)
	}
	return
}

func getSavepointEvent(status v1beta1.SavepointStatus) (eventType string, eventReason string, eventMessage string) {
	switch status.State {
	case SavepointStateTriggerFailed:
		eventType = corev1.EventTypeWarning
		eventReason = "SavepointFailed"
		eventMessage = fmt.Sprintf("Savepoint trigger failed: jobID %v.", status.JobID)
	case SavepointStateProgressing:
		if status.TriggerID == "" {
			break
		}
		eventType = corev1.EventTypeNormal
		eventReason = "SavepointTriggered"
		eventMessage = fmt.Sprintf("Triggered savepoint: jobID %v, triggerID %v.", status.JobID, status.TriggerID)
	case SavepointStateSucceeded:
		eventType = corev1.EventTypeNormal
		eventReason = "SavepointCreated"
		eventMessage = fmt.Sprintf("Successfully savepoint created: jobID %v, triggerID %v.", status.JobID, status.TriggerID)
	case SavepointStateFailed:
		eventType = corev1.EventTypeWarning
		eventReason = "SavepointFailed"
		eventMessage = fmt.Sprintf("Savepoint creation failed: %v", status.Message)
		if status.JobID != "" {
			eventMessage += fmt.Sprintf(", jobID %v",  status.JobID)
		}
		if status.TriggerID != "" {
			eventMessage += fmt.Sprintf(", triggerID %v",  status.TriggerID)
		}
	}
	return
}

func isJobActive(cluster *v1beta1.FlinkCluster) bool {
	var jobStatus = cluster.Status.Components.Job
	return !isJobStopped(cluster.Status.Components.Job) ||
		(jobStatus != nil &&
			jobStatus.State == v1beta1.JobStateFailed &&
			cluster.Spec.Job.RestartPolicy != nil &&
			*cluster.Spec.Job.RestartPolicy == v1beta1.JobRestartPolicyFromSavepointOnFailure &&
			len(jobStatus.SavepointLocation) > 0)
}

func isJobStopped(status *v1beta1.JobStatus) bool {
	return status != nil &&
		(status.State == v1beta1.JobStateSucceeded ||
			status.State == v1beta1.JobStateFailed ||
			status.State == v1beta1.JobStateCancelled)
}
