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
	"github.com/googlecloudplatform/flink-operator/api/v1beta1"
	"github.com/googlecloudplatform/flink-operator/controllers/flinkclient"
	"github.com/googlecloudplatform/flink-operator/controllers/history"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	ControlRetries    = "retries"
	ControlMaxRetries = "3"

	RevisionNameLabel = "flinkoperator.k8s.io/revision-name"

	SavepointRetryIntervalSeconds = 10
)

type UpdateState string
type JobSubmitState string

const (
	UpdateStatePreparing  UpdateState = "Preparing"
	UpdateStateInProgress UpdateState = "InProgress"
	UpdateStateFinished   UpdateState = "Finished"

	JobDeployStateInProgress = "InProgress"
	JobDeployStateSucceeded  = "Succeeded"
	JobDeployStateFailed     = "Failed"
	JobDeployStateUnknown    = "Unknown"
)

type objectForPatch struct {
	Metadata objectMetaForPatch `json:"metadata"`
}

// objectMetaForPatch define object meta struct for patch operation
type objectMetaForPatch struct {
	Annotations map[string]interface{} `json:"annotations"`
}

func getFlinkAPIBaseURL(cluster *v1beta1.FlinkCluster) string {
	clusterDomain := os.Getenv("CLUSTER_DOMAIN")
	if clusterDomain == "" {
		clusterDomain = "cluster.local"
	}

	return fmt.Sprintf(
		"http://%s.%s.svc.%s:%d",
		getJobManagerServiceName(cluster.ObjectMeta.Name),
		cluster.ObjectMeta.Namespace,
		clusterDomain,
		*cluster.Spec.JobManager.Ports.UI)
}

// Gets JobManager ingress name
func getConfigMapName(clusterName string) string {
	return clusterName + "-configmap"
}

// Gets JobManager StatefulSet name
func getJobManagerStatefulSetName(clusterName string) string {
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
func getTaskManagerStatefulSetName(clusterName string) string {
	return clusterName + "-taskmanager"
}

// Gets Job name
func getJobName(clusterName string) string {
	return clusterName + "-job-submitter"
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

func getTime(timeStr string) time.Time {
	var tc TimeConverter
	return tc.FromString(timeStr)
}

func isBlank(s *string) bool {
	return s == nil || strings.TrimSpace(*s) == ""
}

// Checks whether it is possible to take savepoint.
func canTakeSavepoint(cluster v1beta1.FlinkCluster) bool {
	var jobSpec = cluster.Spec.Job
	var savepointStatus = cluster.Status.Savepoint
	var job = cluster.Status.Components.Job
	return jobSpec != nil && jobSpec.SavepointsDir != nil &&
		!job.IsStopped() &&
		(savepointStatus == nil || savepointStatus.State != v1beta1.SavepointStateInProgress)
}

func shouldStopJob(cluster *v1beta1.FlinkCluster) bool {
	var userControl = cluster.Annotations[v1beta1.ControlAnnotation]
	var cancelRequested = cluster.Spec.Job.CancelRequested
	return userControl == v1beta1.ControlNameJobCancel ||
		(cancelRequested != nil && *cancelRequested)
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
	var patch []byte
	var err error

	// Ignore fields not related to rendering job resource.
	if cluster.Spec.Job != nil {
		clusterClone := cluster.DeepCopy()
		clusterClone.Spec.Job.CleanupPolicy = nil
		clusterClone.Spec.Job.RestartPolicy = nil
		clusterClone.Spec.Job.CancelRequested = nil
		clusterClone.Spec.Job.SavepointGeneration = 0
		patch, err = getPatch(clusterClone)
	} else {
		patch, err = getPatch(cluster)
	}
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

func getCurrentRevisionName(r *v1beta1.RevisionStatus) string {
	return r.CurrentRevision[:strings.LastIndex(r.CurrentRevision, "-")]
}

func getNextRevisionName(r *v1beta1.RevisionStatus) string {
	return r.NextRevision[:strings.LastIndex(r.NextRevision, "-")]
}

// Compose revision in FlinkClusterStatus with name and number of ControllerRevision
func getRevisionWithNameNumber(cr *appsv1.ControllerRevision) string {
	return fmt.Sprintf("%v-%v", cr.Name, cr.Revision)
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

// getNewControlRequest returns new requested control that is not in progress now.
func getNewControlRequest(cluster *v1beta1.FlinkCluster) string {
	var userControl = cluster.Annotations[v1beta1.ControlAnnotation]
	var recorded = cluster.Status
	if recorded.Control == nil || recorded.Control.State != v1beta1.ControlStateInProgress {
		return userControl
	}
	return ""
}

func getControlStatus(controlName string, state string) *v1beta1.FlinkClusterControlStatus {
	var controlStatus = new(v1beta1.FlinkClusterControlStatus)
	controlStatus.Name = controlName
	controlStatus.State = state
	setTimestamp(&controlStatus.UpdateTime)
	return controlStatus
}

func getControlEvent(status v1beta1.FlinkClusterControlStatus) (eventType string, eventReason string, eventMessage string) {
	var msg = status.Message
	if len(msg) > 100 {
		msg = msg[:100] + "..."
	}
	switch status.State {
	case v1beta1.ControlStateInProgress:
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
	if triggerReason == v1beta1.SavepointTriggerReasonJobCancel || triggerReason == v1beta1.SavepointTriggerReasonUpdate {
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

func isUserControlFinished(controlStatus *v1beta1.FlinkClusterControlStatus) bool {
	return controlStatus.State == v1beta1.ControlStateSucceeded ||
		controlStatus.State == v1beta1.ControlStateFailed
}

// Check time has passed
func hasTimeElapsed(timeToCheckStr string, now time.Time, intervalSec int) bool {
	tc := &TimeConverter{}
	timeToCheck := tc.FromString(timeToCheckStr)
	intervalPassedTime := timeToCheck.Add(time.Duration(int64(intervalSec) * int64(time.Second)))
	if now.After(intervalPassedTime) {
		return true
	}
	return false
}

// isComponentUpdated checks whether the component updated.
// If the component is observed as well as the next revision name in status.nextRevision and component's label `flinkoperator.k8s.io/hash` are equal, then it is updated already.
// If the component is not observed and it is required, then it is not updated yet.
// If the component is not observed and it is optional, but it is specified in the spec, then it is not updated yet.
func isComponentUpdated(component runtime.Object, cluster *v1beta1.FlinkCluster) bool {
	if !cluster.Status.Revision.IsUpdateTriggered() {
		return true
	}
	switch o := component.(type) {
	case *appsv1.Deployment:
		if o == nil {
			return false
		}
	case *appsv1.StatefulSet:
		if o == nil {
			return false
		}
	case *corev1.ConfigMap:
		if o == nil {
			return false
		}
	case *corev1.Service:
		if o == nil {
			return false
		}
	case *batchv1.Job:
		if o == nil {
			if cluster.Spec.Job != nil {
				return false
			}
			return true
		}
	case *extensionsv1beta1.Ingress:
		if o == nil {
			if cluster.Spec.JobManager.Ingress != nil {
				return false
			}
			return true
		}
	}

	var labels, err = meta.NewAccessor().Labels(component)
	var nextRevisionName = getNextRevisionName(&cluster.Status.Revision)
	if err != nil {
		return false
	}
	return labels[RevisionNameLabel] == nextRevisionName
}

func areComponentsUpdated(components []runtime.Object, cluster *v1beta1.FlinkCluster) bool {
	for _, c := range components {
		if !isComponentUpdated(c, cluster) {
			return false
		}
	}
	return true
}

func isUpdatedAll(observed ObservedClusterState) bool {
	components := []runtime.Object{
		observed.configMap,
		observed.jmStatefulSet,
		observed.tmStatefulSet,
		observed.jmService,
		observed.jmIngress,
		observed.flinkJobSubmitter.job,
	}
	return areComponentsUpdated(components, observed.cluster)
}

// isClusterUpdateToDate checks whether all cluster components are replaced to next revision.
func isClusterUpdateToDate(observed *ObservedClusterState) bool {
	if !observed.cluster.Status.Revision.IsUpdateTriggered() {
		return true
	}
	components := []runtime.Object{
		observed.configMap,
		observed.jmStatefulSet,
		observed.tmStatefulSet,
		observed.jmService,
	}
	return areComponentsUpdated(components, observed.cluster)
}

// isFlinkAPIReady checks whether cluster is ready to submit job.
func isFlinkAPIReady(list *flinkclient.JobStatusList) bool {
	// If the observed Flink job status list is not nil (e.g., emtpy list),
	// it means Flink REST API server is up and running. It is the source of
	// truth of whether we can submit a job.
	return list != nil
}

// jobStateFinalized returns true, if job state is saved so that it can be resumed later.
func finalSavepointRequested(jobID string, s *v1beta1.SavepointStatus) bool {
	return s != nil && s.JobID == jobID &&
		(s.TriggerReason == v1beta1.SavepointTriggerReasonUpdate ||
			s.TriggerReason == v1beta1.SavepointTriggerReasonJobCancel)
}

func getUpdateState(observed *ObservedClusterState) UpdateState {
	if observed.cluster == nil {
		return ""
	}
	var recorded = observed.cluster.Status
	var revision = recorded.Revision
	var job = recorded.Components.Job
	var jobSpec = observed.cluster.Spec.Job

	if !revision.IsUpdateTriggered() {
		return ""
	}
	switch {
	case !job.UpdateReady(jobSpec, observed.observeTime):
		return UpdateStatePreparing
	case !isClusterUpdateToDate(observed):
		return UpdateStateInProgress
	}
	return UpdateStateFinished
}

func shouldUpdateJob(observed *ObservedClusterState) bool {
	return observed.updateState == UpdateStateInProgress
}

func shouldUpdateCluster(observed *ObservedClusterState) bool {
	var job = observed.cluster.Status.Components.Job
	return !job.IsActive() && observed.updateState == UpdateStateInProgress
}

func getNonLiveHistory(revisions []*appsv1.ControllerRevision, historyLimit int) []*appsv1.ControllerRevision {

	history := append([]*appsv1.ControllerRevision{}, revisions...)
	nonLiveHistory := make([]*appsv1.ControllerRevision, 0)

	historyLen := len(history)
	if historyLen <= historyLimit {
		return nonLiveHistory
	}

	nonLiveHistory = append(nonLiveHistory, history[:(historyLen-historyLimit)]...)
	return nonLiveHistory
}

func getFlinkJobDeploymentState(flinkJobState string) string {
	switch flinkJobState {
	case "INITIALIZING", "CREATED", "RUNNING", "FAILING", "CANCELLING", "RESTARTING", "RECONCILING", "SUSPENDED":
		return v1beta1.JobStateRunning
	case "FINISHED":
		return v1beta1.JobStateSucceeded
	case "CANCELED":
		return v1beta1.JobStateCancelled
	case "FAILED":
		return v1beta1.JobStateFailed
	default:
		return ""
	}
}
