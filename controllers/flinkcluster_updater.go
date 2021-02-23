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

// Updater which updates the status of a cluster based on the status of its
// components.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/googlecloudplatform/flink-operator/controllers/flinkclient"
	"k8s.io/apimachinery/pkg/types"
	"reflect"
	"time"

	"github.com/go-logr/logr"
	v1beta1 "github.com/googlecloudplatform/flink-operator/api/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ClusterStatusUpdater updates the status of the FlinkCluster CR.
type ClusterStatusUpdater struct {
	k8sClient client.Client
	context   context.Context
	log       logr.Logger
	recorder  record.EventRecorder
	observed  ObservedClusterState
}

// Compares the current status recorded in the cluster's status field and the
// new status derived from the status of the components, updates the cluster
// status if it is changed, returns the new status.
func (updater *ClusterStatusUpdater) updateStatusIfChanged() (
	bool, error) {
	if updater.observed.cluster == nil {
		updater.log.Info("The cluster has been deleted, no status to update")
		return false, nil
	}

	// Current status recorded in the cluster's status field.
	var oldStatus = v1beta1.FlinkClusterStatus{}
	updater.observed.cluster.Status.DeepCopyInto(&oldStatus)
	oldStatus.LastUpdateTime = ""

	// New status derived from the cluster's components.
	var newStatus = updater.deriveClusterStatus(
		&updater.observed.cluster.Status, &updater.observed)

	// Compare
	var changed = updater.isStatusChanged(oldStatus, newStatus)

	// Update
	if changed {
		updater.log.Info(
			"Status changed",
			"old",
			updater.observed.cluster.Status,
			"new", newStatus)
		updater.createStatusChangeEvents(oldStatus, newStatus)
		var tc = &TimeConverter{}
		newStatus.LastUpdateTime = tc.ToString(time.Now())
		return true, updater.updateClusterStatus(newStatus)
	}

	updater.log.Info("No status change", "state", oldStatus.State)
	return false, nil
}

func (updater *ClusterStatusUpdater) createStatusChangeEvents(
	oldStatus v1beta1.FlinkClusterStatus,
	newStatus v1beta1.FlinkClusterStatus) {
	if oldStatus.Components.JobManagerStatefulSet.State !=
		newStatus.Components.JobManagerStatefulSet.State {
		updater.createStatusChangeEvent(
			"JobManager StatefulSet",
			oldStatus.Components.JobManagerStatefulSet.State,
			newStatus.Components.JobManagerStatefulSet.State)
	}

	// ConfigMap.
	if oldStatus.Components.ConfigMap.State !=
		newStatus.Components.ConfigMap.State {
		updater.createStatusChangeEvent(
			"ConfigMap",
			oldStatus.Components.ConfigMap.State,
			newStatus.Components.ConfigMap.State)
	}

	// JobManager service.
	if oldStatus.Components.JobManagerService.State !=
		newStatus.Components.JobManagerService.State {
		updater.createStatusChangeEvent(
			"JobManager service",
			oldStatus.Components.JobManagerService.State,
			newStatus.Components.JobManagerService.State)
	}

	// JobManager ingress.
	if oldStatus.Components.JobManagerIngress == nil && newStatus.Components.JobManagerIngress != nil {
		updater.createStatusChangeEvent(
			"JobManager ingress", "",
			newStatus.Components.JobManagerIngress.State)
	}
	if oldStatus.Components.JobManagerIngress != nil && newStatus.Components.JobManagerIngress != nil &&
		oldStatus.Components.JobManagerIngress.State != newStatus.Components.JobManagerIngress.State {
		updater.createStatusChangeEvent(
			"JobManager ingress",
			oldStatus.Components.JobManagerIngress.State,
			newStatus.Components.JobManagerIngress.State)
	}

	// TaskManager.
	if oldStatus.Components.TaskManagerStatefulSet.State !=
		newStatus.Components.TaskManagerStatefulSet.State {
		updater.createStatusChangeEvent(
			"TaskManager StatefulSet",
			oldStatus.Components.TaskManagerStatefulSet.State,
			newStatus.Components.TaskManagerStatefulSet.State)
	}

	// Job.
	if oldStatus.Components.Job == nil && newStatus.Components.Job != nil {
		updater.createStatusChangeEvent(
			"Job", "", newStatus.Components.Job.State)
	}
	if oldStatus.Components.Job != nil && newStatus.Components.Job != nil &&
		oldStatus.Components.Job.State != newStatus.Components.Job.State {
		updater.createStatusChangeEvent(
			"Job",
			oldStatus.Components.Job.State,
			newStatus.Components.Job.State)
	}

	// Cluster.
	if oldStatus.State != newStatus.State {
		updater.createStatusChangeEvent("Cluster", oldStatus.State, newStatus.State)
	}

	// Savepoint.
	if newStatus.Savepoint != nil && !reflect.DeepEqual(oldStatus.Savepoint, newStatus.Savepoint) {
		eventType, eventReason, eventMessage := getSavepointEvent(*newStatus.Savepoint)
		updater.recorder.Event(updater.observed.cluster, eventType, eventReason, eventMessage)
	}

	// Control.
	if newStatus.Control != nil && !reflect.DeepEqual(oldStatus.Control, newStatus.Control) {
		eventType, eventReason, eventMessage := getControlEvent(*newStatus.Control)
		updater.recorder.Event(updater.observed.cluster, eventType, eventReason, eventMessage)
	}
}

func (updater *ClusterStatusUpdater) createStatusChangeEvent(
	name string, oldStatus string, newStatus string) {
	if len(oldStatus) == 0 {
		updater.recorder.Event(
			updater.observed.cluster,
			"Normal",
			"StatusUpdate",
			fmt.Sprintf("%v status: %v", name, newStatus))
	} else {
		updater.recorder.Event(
			updater.observed.cluster,
			"Normal",
			"StatusUpdate",
			fmt.Sprintf(
				"%v status changed: %v -> %v", name, oldStatus, newStatus))
	}
}

func (updater *ClusterStatusUpdater) deriveClusterStatus(
	recorded *v1beta1.FlinkClusterStatus,
	observed *ObservedClusterState) v1beta1.FlinkClusterStatus {
	var status = v1beta1.FlinkClusterStatus{}
	var runningComponents = 0
	// jmStatefulSet, jmService, tmStatefulSet.
	var totalComponents = 3
	var updateState = getUpdateState(*observed)
	var isClusterUpdating = !isClusterUpdateToDate(*observed) && updateState == UpdateStateInProgress
	var isJobUpdating = recorded.Components.Job != nil && recorded.Components.Job.State == v1beta1.JobStateUpdating

	// ConfigMap.
	var observedConfigMap = observed.configMap
	if !isComponentUpdated(observedConfigMap, *observed.cluster) && isJobUpdating {
		recorded.Components.ConfigMap.DeepCopyInto(&status.Components.ConfigMap)
		status.Components.ConfigMap.State = v1beta1.ComponentStateUpdating
	} else if observedConfigMap != nil {
		status.Components.ConfigMap.Name = observedConfigMap.ObjectMeta.Name
		status.Components.ConfigMap.State = v1beta1.ComponentStateReady
	} else if recorded.Components.ConfigMap.Name != "" {
		status.Components.ConfigMap =
			v1beta1.FlinkClusterComponentState{
				Name:  recorded.Components.ConfigMap.Name,
				State: v1beta1.ComponentStateDeleted,
			}
	}

	// JobManager StatefulSet.
	var observedJmStatefulSet = observed.jmStatefulSet
	if !isComponentUpdated(observedJmStatefulSet, *observed.cluster) && isJobUpdating {
		recorded.Components.JobManagerStatefulSet.DeepCopyInto(&status.Components.JobManagerStatefulSet)
		status.Components.JobManagerStatefulSet.State = v1beta1.ComponentStateUpdating
	} else if observedJmStatefulSet != nil {
		status.Components.JobManagerStatefulSet.Name = observedJmStatefulSet.ObjectMeta.Name
		status.Components.JobManagerStatefulSet.State = getStatefulSetState(observedJmStatefulSet)
		if status.Components.JobManagerStatefulSet.State == v1beta1.ComponentStateReady {
			runningComponents++
		}
	} else if recorded.Components.JobManagerStatefulSet.Name != "" {
		status.Components.JobManagerStatefulSet =
			v1beta1.FlinkClusterComponentState{
				Name:  recorded.Components.JobManagerStatefulSet.Name,
				State: v1beta1.ComponentStateDeleted,
			}
	}

	// JobManager service.
	var observedJmService = observed.jmService
	if !isComponentUpdated(observedJmService, *observed.cluster) && isJobUpdating {
		recorded.Components.JobManagerService.DeepCopyInto(&status.Components.JobManagerService)
		status.Components.JobManagerService.State = v1beta1.ComponentStateUpdating
	} else if observedJmService != nil {
		var state string
		var nodePort int32
		switch observedJmService.Spec.Type {
		case corev1.ServiceTypeClusterIP:
			if observedJmService.Spec.ClusterIP != "" {
				state = v1beta1.ComponentStateReady
				runningComponents++
			} else {
				state = v1beta1.ComponentStateNotReady
			}
		case corev1.ServiceTypeLoadBalancer:
			if len(observedJmService.Status.LoadBalancer.Ingress) > 0 {
				state = v1beta1.ComponentStateReady
				runningComponents++
			} else {
				state = v1beta1.ComponentStateNotReady
			}
		case corev1.ServiceTypeNodePort:
			if len(observedJmService.Spec.Ports) > 0 {
				state = v1beta1.ComponentStateReady
				runningComponents++
				for _, port := range observedJmService.Spec.Ports {
					if port.Name == "ui" {
						nodePort = port.NodePort
					}
				}
			} else {
				state = v1beta1.ComponentStateNotReady
			}
		}

		status.Components.JobManagerService =
			v1beta1.JobManagerServiceStatus{
				Name:     observedJmService.ObjectMeta.Name,
				State:    state,
				NodePort: nodePort,
			}
	} else if recorded.Components.JobManagerService.Name != "" {
		status.Components.JobManagerService =
			v1beta1.JobManagerServiceStatus{
				Name:  recorded.Components.JobManagerService.Name,
				State: v1beta1.ComponentStateDeleted,
			}
	}

	// (Optional) JobManager ingress.
	var observedJmIngress = observed.jmIngress
	if !isComponentUpdated(observedJmIngress, *observed.cluster) && isJobUpdating {
		status.Components.JobManagerIngress = &v1beta1.JobManagerIngressStatus{}
		recorded.Components.JobManagerIngress.DeepCopyInto(status.Components.JobManagerIngress)
		status.Components.JobManagerIngress.State = v1beta1.ComponentStateUpdating
	} else if observedJmIngress != nil {
		var state string
		var urls []string
		var useTLS bool
		var useHost bool
		var loadbalancerReady bool

		if len(observedJmIngress.Spec.TLS) > 0 {
			useTLS = true
		}

		if useTLS {
			for _, tls := range observedJmIngress.Spec.TLS {
				for _, host := range tls.Hosts {
					if host != "" {
						urls = append(urls, "https://"+host)
					}
				}
			}
		} else {
			for _, rule := range observedJmIngress.Spec.Rules {
				if rule.Host != "" {
					urls = append(urls, "http://"+rule.Host)
				}
			}
		}
		if len(urls) > 0 {
			useHost = true
		}

		// Check loadbalancer is ready.
		if len(observedJmIngress.Status.LoadBalancer.Ingress) > 0 {
			var addr string
			for _, ingress := range observedJmIngress.Status.LoadBalancer.Ingress {
				// Get loadbalancer address.
				if ingress.Hostname != "" {
					addr = ingress.Hostname
				} else if ingress.IP != "" {
					addr = ingress.IP
				}
				// If ingress spec does not have host, get ip or hostname of loadbalancer.
				if !useHost && addr != "" {
					if useTLS {
						urls = append(urls, "https://"+addr)
					} else {
						urls = append(urls, "http://"+addr)
					}
				}
			}
			// If any ready LB found, state is ready.
			if addr != "" {
				loadbalancerReady = true
			}
		}

		// Jobmanager ingress state become ready when LB for ingress is specified.
		if loadbalancerReady {
			state = v1beta1.ComponentStateReady
		} else {
			state = v1beta1.ComponentStateNotReady
		}

		status.Components.JobManagerIngress =
			&v1beta1.JobManagerIngressStatus{
				Name:  observedJmIngress.ObjectMeta.Name,
				State: state,
				URLs:  urls,
			}
	} else if recorded.Components.JobManagerIngress != nil &&
		recorded.Components.JobManagerIngress.Name != "" {
		status.Components.JobManagerIngress =
			&v1beta1.JobManagerIngressStatus{
				Name:  recorded.Components.JobManagerIngress.Name,
				State: v1beta1.ComponentStateDeleted,
			}
	}

	// TaskManager StatefulSet.
	var observedTmStatefulSet = observed.tmStatefulSet
	if !isComponentUpdated(observedTmStatefulSet, *observed.cluster) && isJobUpdating {
		recorded.Components.TaskManagerStatefulSet.DeepCopyInto(&status.Components.TaskManagerStatefulSet)
		status.Components.TaskManagerStatefulSet.State = v1beta1.ComponentStateUpdating
	} else if observedTmStatefulSet != nil {
		status.Components.TaskManagerStatefulSet.Name =
			observedTmStatefulSet.ObjectMeta.Name
		status.Components.TaskManagerStatefulSet.State =
			getStatefulSetState(observedTmStatefulSet)
		if status.Components.TaskManagerStatefulSet.State ==
			v1beta1.ComponentStateReady {
			runningComponents++
		}
	} else if recorded.Components.TaskManagerStatefulSet.Name != "" {
		status.Components.TaskManagerStatefulSet =
			v1beta1.FlinkClusterComponentState{
				Name:  recorded.Components.TaskManagerStatefulSet.Name,
				State: v1beta1.ComponentStateDeleted,
			}
	}

	// (Optional) Job.
	// Update job status.
	status.Components.Job = updater.getJobStatus()

	// Derive the new cluster state.
	switch recorded.State {
	case "", v1beta1.ClusterStateCreating:
		if runningComponents < totalComponents {
			status.State = v1beta1.ClusterStateCreating
		} else {
			status.State = v1beta1.ClusterStateRunning
		}
	case v1beta1.ClusterStateUpdating:
		if isClusterUpdating {
			status.State = v1beta1.ClusterStateUpdating
		} else if runningComponents < totalComponents {
			if isUpdateTriggered(*recorded) {
				status.State = v1beta1.ClusterStateUpdating
			} else {
				status.State = v1beta1.ClusterStateReconciling
			}
		} else {
			status.State = v1beta1.ClusterStateRunning
		}
	case v1beta1.ClusterStateRunning,
		v1beta1.ClusterStateReconciling:
		var jobStatus = status.Components.Job
		if isClusterUpdating {
			status.State = v1beta1.ClusterStateUpdating
		} else if isJobStopped(jobStatus) {
			var policy = observed.cluster.Spec.Job.CleanupPolicy
			if jobStatus.State == v1beta1.JobStateSucceeded &&
				policy.AfterJobSucceeds != v1beta1.CleanupActionKeepCluster {
				status.State = v1beta1.ClusterStateStopping
			} else if jobStatus.State == v1beta1.JobStateFailed &&
				policy.AfterJobFails != v1beta1.CleanupActionKeepCluster {
				status.State = v1beta1.ClusterStateStopping
			} else if jobStatus.State == v1beta1.JobStateCancelled &&
				policy.AfterJobCancelled != v1beta1.CleanupActionKeepCluster {
				status.State = v1beta1.ClusterStateStopping
			} else {
				status.State = v1beta1.ClusterStateRunning
			}
		} else if runningComponents < totalComponents {
			status.State = v1beta1.ClusterStateReconciling
		} else {
			status.State = v1beta1.ClusterStateRunning
		}
	case v1beta1.ClusterStateStopping,
		v1beta1.ClusterStatePartiallyStopped:
		if isClusterUpdating {
			status.State = v1beta1.ClusterStateUpdating
		} else if runningComponents == 0 {
			status.State = v1beta1.ClusterStateStopped
		} else if runningComponents < totalComponents {
			status.State = v1beta1.ClusterStatePartiallyStopped
		} else {
			status.State = v1beta1.ClusterStateStopping
		}
	case v1beta1.ClusterStateStopped:
		if isUpdateTriggered(*recorded) {
			status.State = v1beta1.ClusterStateUpdating
		} else {
			status.State = v1beta1.ClusterStateStopped
		}
	default:
		panic(fmt.Sprintf("Unknown cluster state: %v", recorded.State))
	}

	// Update savepoint status if it is in progress or requested.
	status.Savepoint = updater.getUpdatedSavepointStatus(
		observed.savepoint,
		recorded.Savepoint,
		recorded.Components.Job,
		updater.getFlinkJobID())

	// User requested control
	status.Control = getControlStatus(observed.cluster, &status, recorded)

	// Update finished
	if updateState == UpdateStateFinished {
		status.CurrentRevision = observed.cluster.Status.NextRevision
		updater.log.Info("Finished update.")
	}

	// Update revision status
	status.NextRevision = getRevisionWithNameNumber(observed.revisionStatus.nextRevision)
	if status.CurrentRevision == "" {
		if recorded.CurrentRevision == "" {
			status.CurrentRevision = getRevisionWithNameNumber(observed.revisionStatus.currentRevision)
		} else {
			status.CurrentRevision = recorded.CurrentRevision
		}
	}
	if observed.revisionStatus.collisionCount != 0 {
		status.CollisionCount = new(int32)
		*status.CollisionCount = observed.revisionStatus.collisionCount
	}

	return status
}

// Gets Flink job ID based on the observed state and the recorded state.
//
// It is possible that the recorded is not nil, but the observed is, due
// to transient error or being skiped as an optimization.
// If this returned nil, it is the state that job is not submitted or not identified yet.
func (updater *ClusterStatusUpdater) getFlinkJobID() *string {
	// Observed from active job manager
	var observedFlinkJob = updater.observed.flinkJobStatus.flinkJob
	if observedFlinkJob != nil && len(observedFlinkJob.ID) > 0 {
		return &observedFlinkJob.ID
	}

	// Observed from job submitter (when job manager is not ready yet)
	var observedJobSubmitLog = updater.observed.flinkJobSubmitLog
	if observedJobSubmitLog != nil && observedJobSubmitLog.JobID != "" {
		return &observedJobSubmitLog.JobID
	}

	// Recorded.
	var recordedJobStatus = updater.observed.cluster.Status.Components.Job
	if recordedJobStatus != nil && len(recordedJobStatus.ID) > 0 {
		return &recordedJobStatus.ID
	}

	return nil
}

func (updater *ClusterStatusUpdater) getJobStatus() *v1beta1.JobStatus {
	var observed = updater.observed
	var observedJob = updater.observed.job
	var observedFlinkJob = updater.observed.flinkJobStatus.flinkJob
	var observedCluster = updater.observed.cluster
	var observedSavepoint = updater.observed.savepoint
	var recordedJobStatus = updater.observed.cluster.Status.Components.Job
	var newJobStatus *v1beta1.JobStatus

	if recordedJobStatus == nil {
		return nil
	}
	newJobStatus = recordedJobStatus.DeepCopy()

	// Derive job state
	var jobState string
	switch {
	// When updating stopped job
	case isUpdateTriggered(observedCluster.Status) && isJobStopped(recordedJobStatus):
		jobState = v1beta1.JobStateUpdating
	// Already terminated state
	case isJobTerminated(observedCluster.Spec.Job.RestartPolicy, recordedJobStatus):
		jobState = recordedJobStatus.State
	// Derive state from the observed Flink job
	case observedFlinkJob != nil:
		jobState = getFlinkJobDeploymentState(observedFlinkJob.Status)
		if jobState == "" {
			updater.log.Error(errors.New("failed to get Flink job deployment state"), "observed flink job status", observedFlinkJob.Status)
			jobState = recordedJobStatus.State
		}
	// When Flink job not found
	case isFlinkAPIReady(observed):
		switch recordedJobStatus.State {
		case v1beta1.JobStateRunning:
			jobState = v1beta1.JobStateLost
		case v1beta1.JobStatePending:
			// Flink job is submitted but not confirmed via job manager yet
			var jobSubmitSucceeded = updater.getFlinkJobID() != nil
			// Flink job submit is in progress
			var jobSubmitInProgress = observedJob != nil &&
				observedJob.Status.Succeeded == 0 &&
				observedJob.Status.Failed == 0
			if jobSubmitSucceeded || jobSubmitInProgress {
				jobState = v1beta1.JobStatePending
				break
			}
			jobState = v1beta1.JobStateFailed
		default:
			jobState = recordedJobStatus.State
		}
	// When Flink API unavailable
	default:
		if recordedJobStatus.State == v1beta1.JobStatePending {
			var jobSubmitFailed = observedJob != nil && observedJob.Status.Failed > 0
			if jobSubmitFailed {
				jobState = v1beta1.JobStateFailed
				break
			}
		}
		jobState = recordedJobStatus.State
	}

	// Flink Job ID
	if jobState == v1beta1.JobStateUpdating {
		newJobStatus.ID = ""
	} else if observedFlinkJob != nil {
		newJobStatus.ID = observedFlinkJob.ID
	}

	// State
	newJobStatus.State = jobState

	// Flink job start time
	// TODO: It would be nice to set StartTime with the timestamp retrieved from the Flink job API like /jobs/{job-id}.
	if jobState == v1beta1.JobStateRunning && newJobStatus.StartTime == "" {
		setTimestamp(&newJobStatus.StartTime)
	}

	// Savepoint
	if newJobStatus != nil && observedSavepoint != nil && observedSavepoint.IsSuccessful() {
		newJobStatus.SavepointGeneration++
		newJobStatus.LastSavepointTriggerID = observedSavepoint.TriggerID
		newJobStatus.SavepointLocation = observedSavepoint.Location

		// TODO: LastSavepointTime should be set with the timestamp generated in job manager.
		// Currently savepoint complete timestamp is not included in savepoints API response.
		// Whereas checkpoint API returns the timestamp latest_ack_timestamp.
		// Note: https://ci.apache.org/projects/flink/flink-docs-stable/ops/rest_api.html#jobs-jobid-checkpoints-details-checkpointid
		setTimestamp(&newJobStatus.LastSavepointTime)
	}

	return newJobStatus
}

func (updater *ClusterStatusUpdater) isStatusChanged(
	currentStatus v1beta1.FlinkClusterStatus,
	newStatus v1beta1.FlinkClusterStatus) bool {
	var changed = false
	if newStatus.State != currentStatus.State {
		changed = true
		updater.log.Info(
			"Cluster state changed",
			"current",
			currentStatus.State,
			"new",
			newStatus.State)
	}
	if !reflect.DeepEqual(newStatus.Control, currentStatus.Control) {
		updater.log.Info(
			"Control status changed", "current",
			currentStatus.Control,
			"new",
			newStatus.Control)
		changed = true
	}
	if newStatus.Components.ConfigMap !=
		currentStatus.Components.ConfigMap {
		updater.log.Info(
			"ConfigMap status changed",
			"current",
			currentStatus.Components.ConfigMap,
			"new",
			newStatus.Components.ConfigMap)
		changed = true
	}
	if newStatus.Components.JobManagerStatefulSet !=
		currentStatus.Components.JobManagerStatefulSet {
		updater.log.Info(
			"JobManager StatefulSet status changed",
			"current", currentStatus.Components.JobManagerStatefulSet,
			"new",
			newStatus.Components.JobManagerStatefulSet)
		changed = true
	}
	if newStatus.Components.JobManagerService !=
		currentStatus.Components.JobManagerService {
		updater.log.Info(
			"JobManager service status changed",
			"current",
			currentStatus.Components.JobManagerService,
			"new", newStatus.Components.JobManagerService)
		changed = true
	}
	if currentStatus.Components.JobManagerIngress == nil {
		if newStatus.Components.JobManagerIngress != nil {
			updater.log.Info(
				"JobManager ingress status changed",
				"current",
				"nil",
				"new", *newStatus.Components.JobManagerIngress)
			changed = true
		}
	} else {
		if newStatus.Components.JobManagerIngress.State != currentStatus.Components.JobManagerIngress.State {
			updater.log.Info(
				"JobManager ingress status changed",
				"current",
				*currentStatus.Components.JobManagerIngress,
				"new",
				*newStatus.Components.JobManagerIngress)
			changed = true
		}
	}
	if newStatus.Components.TaskManagerStatefulSet !=
		currentStatus.Components.TaskManagerStatefulSet {
		updater.log.Info(
			"TaskManager StatefulSet status changed",
			"current",
			currentStatus.Components.TaskManagerStatefulSet,
			"new",
			newStatus.Components.TaskManagerStatefulSet)
		changed = true
	}
	if currentStatus.Components.Job == nil {
		if newStatus.Components.Job != nil {
			updater.log.Info(
				"Job status changed",
				"current",
				"nil",
				"new",
				*newStatus.Components.Job)
			changed = true
		}
	} else {
		if newStatus.Components.Job != nil {
			var isEqual = reflect.DeepEqual(
				newStatus.Components.Job, currentStatus.Components.Job)
			if !isEqual {
				updater.log.Info(
					"Job status changed",
					"current",
					*currentStatus.Components.Job,
					"new",
					*newStatus.Components.Job)
				changed = true
			}
		} else {
			changed = true
		}
	}
	if !reflect.DeepEqual(newStatus.Savepoint, currentStatus.Savepoint) {
		updater.log.Info(
			"Savepoint status changed", "current",
			currentStatus.Savepoint,
			"new",
			newStatus.Savepoint)
		changed = true
	}
	if newStatus.CurrentRevision != currentStatus.CurrentRevision ||
		newStatus.NextRevision != currentStatus.NextRevision ||
		(newStatus.CollisionCount != nil && currentStatus.CollisionCount == nil) ||
		(currentStatus.CollisionCount != nil && *newStatus.CollisionCount != *currentStatus.CollisionCount) {
		updater.log.Info(
			"FlinkCluster revision status changed", "current",
			fmt.Sprintf("currentRevision: %v, nextRevision: %v, collisionCount: %v", currentStatus.CurrentRevision, currentStatus.NextRevision, currentStatus.CollisionCount),
			"new",
			fmt.Sprintf("currentRevision: %v, nextRevision: %v, collisionCount: %v", newStatus.CurrentRevision, newStatus.NextRevision, newStatus.CollisionCount))
		changed = true
	}
	return changed
}

func (updater *ClusterStatusUpdater) updateClusterStatus(
	status v1beta1.FlinkClusterStatus) error {
	var cluster = v1beta1.FlinkCluster{}
	updater.observed.cluster.DeepCopyInto(&cluster)
	cluster.Status = status
	err := updater.k8sClient.Status().Update(updater.context, &cluster)
	// Clear control annotation after status update is complete.
	updater.clearControlAnnotation(status.Control)
	return err
}

// Clear finished or improper user control in annotations
func (updater *ClusterStatusUpdater) clearControlAnnotation(newControlStatus *v1beta1.FlinkClusterControlStatus) error {
	var userControl = updater.observed.cluster.Annotations[v1beta1.ControlAnnotation]
	if userControl == "" {
		return nil
	}
	if newControlStatus == nil || userControl != newControlStatus.Name || // refused control in updater
		(userControl == newControlStatus.Name && isUserControlFinished(newControlStatus)) /* finished control */ {
		// make annotation patch cleared
		annotationPatch := objectForPatch{
			Metadata: objectMetaForPatch{
				Annotations: map[string]interface{}{
					v1beta1.ControlAnnotation: nil,
				},
			},
		}
		patchBytes, err := json.Marshal(&annotationPatch)
		if err != nil {
			return err
		}
		return updater.k8sClient.Patch(updater.context, updater.observed.cluster, client.ConstantPatch(types.MergePatchType, patchBytes))
	}

	return nil
}

func (updater *ClusterStatusUpdater) getUpdatedSavepointStatus(
	observedSavepoint *Savepoint,
	recordedSavepointStatus *v1beta1.SavepointStatus,
	recordedJobStatus *v1beta1.JobStatus,
	flinkJobID *string) *v1beta1.SavepointStatus {
	if recordedSavepointStatus == nil {
		return nil
	}

	var savepointStatus = recordedSavepointStatus.DeepCopy()
	var errMsg string
	if savepointStatus.State == v1beta1.SavepointStateInProgress && observedSavepoint != nil {
		switch {
		case observedSavepoint.IsSuccessful():
			savepointStatus.State = v1beta1.SavepointStateSucceeded
		case observedSavepoint.IsFailed():
			savepointStatus.State = v1beta1.SavepointStateFailed
			errMsg = fmt.Sprintf("Savepoint error: %v", observedSavepoint.FailureCause.StackTrace)
		case observedSavepoint.savepointErr != nil:
			if err, ok := observedSavepoint.savepointErr.(*flinkclient.HTTPError); ok {
				savepointStatus.State = v1beta1.SavepointStateFailed
				errMsg = fmt.Sprintf("Failed to get savepoint status: %v", err)
			}
		}
	}
	if savepointStatus.State == v1beta1.SavepointStateInProgress {
		switch {
		case isJobStopped(recordedJobStatus):
			errMsg = "Flink job is stopped."
			savepointStatus.State = v1beta1.SavepointStateFailed
		case flinkJobID == nil:
			errMsg = "Flink job is not identified."
			savepointStatus.State = v1beta1.SavepointStateFailed
		case flinkJobID != nil && (recordedSavepointStatus.TriggerID != "" && *flinkJobID != recordedSavepointStatus.JobID):
			errMsg = "Savepoint triggered Flink job is lost."
			savepointStatus.State = v1beta1.SavepointStateFailed
		}
	}
	if errMsg != "" {
		if savepointStatus.TriggerReason == v1beta1.SavepointTriggerReasonUpdate {
			errMsg =
				"Failed to take savepoint for update. " +
					"The update process is being postponed until a savepoint is available. " + errMsg
		}
		if len(errMsg) > 1024 {
			errMsg = errMsg[:1024]
		}
		savepointStatus.Message = errMsg
	}
	return savepointStatus
}

func getControlStatus(cluster *v1beta1.FlinkCluster,
	new *v1beta1.FlinkClusterStatus,
	recorded *v1beta1.FlinkClusterStatus) *v1beta1.FlinkClusterControlStatus {
	var userControl = cluster.Annotations[v1beta1.ControlAnnotation]
	var controlStatus *v1beta1.FlinkClusterControlStatus
	var controlRequest = getNewUserControlRequest(cluster)

	// New control status
	if controlRequest != "" {
		controlStatus = getUserControlStatus(controlRequest, v1beta1.ControlStateRequested)
		return controlStatus
	}

	// Update control status in progress
	if recorded.Control != nil && userControl == recorded.Control.Name &&
		recorded.Control.State == v1beta1.ControlStateInProgress {
		controlStatus = recorded.Control.DeepCopy()
		switch recorded.Control.Name {
		case v1beta1.ControlNameJobCancel:
			if new.Components.Job.State == v1beta1.JobStateCancelled {
				controlStatus.State = v1beta1.ControlStateSucceeded
				setTimestamp(&controlStatus.UpdateTime)
			} else if isJobTerminated(cluster.Spec.Job.RestartPolicy, recorded.Components.Job) {
				controlStatus.Message = "Aborted job cancellation: job is terminated already."
				controlStatus.State = v1beta1.ControlStateFailed
				setTimestamp(&controlStatus.UpdateTime)
			} else if new.Savepoint != nil && new.Savepoint.State == v1beta1.SavepointStateFailed {
				controlStatus.Message = "Aborted job cancellation: failed to take savepoint."
				controlStatus.State = v1beta1.ControlStateFailed
				setTimestamp(&controlStatus.UpdateTime)
			} else if recorded.Control.Message != "" {
				controlStatus.State = v1beta1.ControlStateFailed
				setTimestamp(&controlStatus.UpdateTime)
			}
		case v1beta1.ControlNameSavepoint:
			if new.Savepoint != nil {
				if new.Savepoint.State == v1beta1.SavepointStateSucceeded {
					controlStatus.State = v1beta1.ControlStateSucceeded
					setTimestamp(&controlStatus.UpdateTime)
				} else if new.Savepoint.State == v1beta1.SavepointStateFailed || new.Savepoint.State == v1beta1.SavepointStateTriggerFailed {
					controlStatus.State = v1beta1.ControlStateFailed
					setTimestamp(&controlStatus.UpdateTime)
				}
			}
		}
		// Aborted by max retry reach
		var retries = controlStatus.Details[ControlRetries]
		if retries == ControlMaxRetries {
			controlStatus.Message = fmt.Sprintf("Aborted control %v. The maximum number of retries has been reached.", controlStatus.Name)
			controlStatus.State = v1beta1.ControlStateFailed
			setTimestamp(&controlStatus.UpdateTime)
		}
		return controlStatus
	}

	// Maintain control status if there is no change.
	if recorded.Control != nil && controlStatus == nil {
		controlStatus = recorded.Control.DeepCopy()
		return controlStatus
	}

	return nil
}

func getStatefulSetState(statefulSet *appsv1.StatefulSet) string {
	if statefulSet.Status.ReadyReplicas >= *statefulSet.Spec.Replicas {
		return v1beta1.ComponentStateReady
	}
	return v1beta1.ComponentStateNotReady
}
