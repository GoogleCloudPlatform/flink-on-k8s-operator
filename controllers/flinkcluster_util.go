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
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/googlecloudplatform/flink-operator/controllers/history"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"strconv"
	"time"

	v1beta1 "github.com/googlecloudplatform/flink-operator/api/v1beta1"
	batchv1 "k8s.io/api/batch/v1"
)

const (
	ControlSavepointTriggerID = "SavepointTriggerID"
	ControlJobID              = "jobID"
	ControlRetries            = "retries"
	ControlMaxRetries         = "3"

	SavepointTimeoutSec = 60
)

type objectForPatch struct {
	Metadata objectMetaForPatch `json:"metadata"`
}

// objectMetaForPatch define object meta struct for patch operation
type objectMetaForPatch struct {
	Annotations map[string]interface{} `json:"annotations"`
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

func shouldUpdateJob(clusterStatus *v1beta1.FlinkClusterStatus) bool {
	return isJobUpdating(clusterStatus) &&
		clusterStatus.Savepoint.State == v1beta1.SavepointStateSucceeded
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

// newRevision generates FlinkClusterSpec patch and makes new child ControllerRevision resource with it.
func newRevision(cluster *v1beta1.FlinkCluster, revision int64, collisionCount *int32) (*appsv1.ControllerRevision, error) {
	patch, err := getPatch(cluster)
	if err != nil {
		return nil, err
	}
	cr, err := history.NewControllerRevision(cluster,
		controllerKind,
		cluster.ObjectMeta.Labels,
		runtime.RawExtension{Raw: patch},
		revision,
		collisionCount)
	if err != nil {
		return nil, err
	}
	if cr.ObjectMeta.Annotations == nil {
		cr.ObjectMeta.Annotations = make(map[string]string)
	}
	for key, value := range cluster.Annotations {
		cr.ObjectMeta.Annotations[key] = value
	}
	cr.SetNamespace(cluster.GetNamespace())
	cr.GetLabels()[history.ControllerRevisionManagedByLabel] = cluster.GetName()
	return cr, nil
}

func getPatch(cluster *v1beta1.FlinkCluster) ([]byte, error) {
	str := &bytes.Buffer{}
	err := unstructured.UnstructuredJSONScheme.Encode(cluster, str)

	if err != nil {
		return nil, err
	}
	var raw map[string]interface{}
	json.Unmarshal([]byte(str.Bytes()), &raw)
	objCopy := make(map[string]interface{})
	spec := raw["spec"].(map[string]interface{})
	objCopy["spec"] = spec
	spec["$patch"] = "replace"
	patch, err := json.Marshal(objCopy)
	return patch, err
}

func getNextRevisionNumber(revisions []*appsv1.ControllerRevision) int64 {
	count := len(revisions)
	if count <= 0 {
		return 1
	}
	return revisions[count-1].Revision + 1
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

func getNewSavepointStatus(jobID string, triggerID string, triggerReason string, message string, triggerSuccess bool) v1beta1.SavepointStatus {
	var savepointStatus = v1beta1.SavepointStatus{}
	var now string
	setTimestamp(&now)
	savepointStatus.JobID = jobID
	savepointStatus.TriggerID = triggerID
	savepointStatus.TriggerTime = now
	savepointStatus.TriggerReason = triggerReason
	savepointStatus.Message = message
	if triggerSuccess {
		savepointStatus.State = v1beta1.SavepointStateInProgress
	} else {
		savepointStatus.State = v1beta1.SavepointStateTriggerFailed
	}
	return savepointStatus
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
	var msg = status.Message
	if len(msg) > 100 {
		msg = msg[:100] + "..."
	}
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
		if status.Message != "" {
			eventMessage = fmt.Sprintf("User control %v failed: %v", status.Name, msg)
		} else {
			eventMessage = fmt.Sprintf("User control %v failed", status.Name)
		}
	}
	return
}

func getSavepointEvent(status v1beta1.SavepointStatus) (eventType string, eventReason string, eventMessage string) {
	var msg = status.Message
	if len(msg) > 100 {
		msg = msg[:100] + "..."
	}
	var triggerReason = status.TriggerReason
	if triggerReason == v1beta1.SavepointTriggerReasonJobCancel || triggerReason == v1beta1.SavepointTriggerReasonJobUpdate {
		triggerReason = "for " + triggerReason
	}
	switch status.State {
	case v1beta1.SavepointStateTriggerFailed:
		eventType = corev1.EventTypeWarning
		eventReason = "SavepointFailed"
		eventMessage = fmt.Sprintf("Failed to trigger savepoint %v: %v", triggerReason, msg)
	case v1beta1.SavepointStateInProgress:
		eventType = corev1.EventTypeNormal
		eventReason = "SavepointTriggered"
		eventMessage = fmt.Sprintf("Triggered savepoint %v: triggerID %v.", triggerReason, status.TriggerID)
	case v1beta1.SavepointStateSucceeded:
		eventType = corev1.EventTypeNormal
		eventReason = "SavepointCreated"
		eventMessage = fmt.Sprintf("Successfully savepoint created")
	case v1beta1.SavepointStateFailed:
		eventType = corev1.EventTypeWarning
		eventReason = "SavepointFailed"
		eventMessage = fmt.Sprintf("Savepoint creation failed: %v", msg)
	}
	return
}

func isJobStopped(status *v1beta1.JobStatus) bool {
	return status != nil &&
		(status.State == v1beta1.JobStateSucceeded ||
			status.State == v1beta1.JobStateFailed ||
			status.State == v1beta1.JobStateCancelled)
}

func isJobTerminated(restartPolicy *v1beta1.JobRestartPolicy, jobStatus *v1beta1.JobStatus) bool {
	return isJobStopped(jobStatus) && !shouldRestartJob(restartPolicy, jobStatus)
}

func isUserControlFinished(controlStatus *v1beta1.FlinkClusterControlStatus) bool {
	return controlStatus.State == v1beta1.ControlStateSucceeded ||
		controlStatus.State == v1beta1.ControlStateFailed
}

func isJobUpdating(status *v1beta1.FlinkClusterStatus) bool {
	return status.CurrentRevision != status.NextRevision
}
