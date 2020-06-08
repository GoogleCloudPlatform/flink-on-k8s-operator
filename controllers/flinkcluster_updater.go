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
	"fmt"
	"github.com/googlecloudplatform/flink-operator/controllers/history"
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
	history   history.Interface
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

	// Sync history and revision status
	err := updater.syncRevisionStatus(&newStatus)
	if err != nil {
		updater.log.Error(err, "Failed to sync flinkCluster history")
	}

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
	if oldStatus.Components.JobManagerDeployment.State !=
		newStatus.Components.JobManagerDeployment.State {
		updater.createStatusChangeEvent(
			"JobManager deployment",
			oldStatus.Components.JobManagerDeployment.State,
			newStatus.Components.JobManagerDeployment.State)
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
	if oldStatus.Components.TaskManagerDeployment.State !=
		newStatus.Components.TaskManagerDeployment.State {
		updater.createStatusChangeEvent(
			"TaskManager deployment",
			oldStatus.Components.TaskManagerDeployment.State,
			newStatus.Components.TaskManagerDeployment.State)
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
	// jmDeployment, jmService, tmDeployment.
	var totalComponents = 3
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

	// JobManager deployment.
	var observedJmDeployment = observed.jmDeployment
	if !isComponentUpdated(observedJmDeployment, *observed.cluster) && isJobUpdating {
		recorded.Components.JobManagerDeployment.DeepCopyInto(&status.Components.JobManagerDeployment)
		status.Components.JobManagerDeployment.State = v1beta1.ComponentStateUpdating
	} else if observedJmDeployment != nil {
		status.Components.JobManagerDeployment.Name = observedJmDeployment.ObjectMeta.Name
		status.Components.JobManagerDeployment.State = getDeploymentState(observedJmDeployment)
		if status.Components.JobManagerDeployment.State == v1beta1.ComponentStateReady {
			runningComponents++
		}
	} else if recorded.Components.JobManagerDeployment.Name != "" {
		status.Components.JobManagerDeployment =
			v1beta1.FlinkClusterComponentState{
				Name:  recorded.Components.JobManagerDeployment.Name,
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
		if observedJmService.Spec.Type == corev1.ServiceTypeClusterIP {
			if observedJmService.Spec.ClusterIP != "" {
				state = v1beta1.ComponentStateReady
				runningComponents++
			} else {
				state = v1beta1.ComponentStateNotReady
			}
		} else if observedJmService.Spec.Type == corev1.ServiceTypeLoadBalancer {
			if len(observedJmService.Status.LoadBalancer.Ingress) > 0 {
				state = v1beta1.ComponentStateReady
				runningComponents++
			} else {
				state = v1beta1.ComponentStateNotReady
			}
		} else if observedJmService.Spec.Type == corev1.ServiceTypeNodePort {
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

	// TaskManager deployment.
	var observedTmDeployment = observed.tmDeployment
	if !isComponentUpdated(observedTmDeployment, *observed.cluster) && isJobUpdating {
		recorded.Components.TaskManagerDeployment.DeepCopyInto(&status.Components.TaskManagerDeployment)
		status.Components.TaskManagerDeployment.State = v1beta1.ComponentStateUpdating
	} else if observedTmDeployment != nil {
		status.Components.TaskManagerDeployment.Name =
			observedTmDeployment.ObjectMeta.Name
		status.Components.TaskManagerDeployment.State =
			getDeploymentState(observedTmDeployment)
		if status.Components.TaskManagerDeployment.State ==
			v1beta1.ComponentStateReady {
			runningComponents++
		}
	} else if recorded.Components.TaskManagerDeployment.Name != "" {
		status.Components.TaskManagerDeployment =
			v1beta1.FlinkClusterComponentState{
				Name:  recorded.Components.TaskManagerDeployment.Name,
				State: v1beta1.ComponentStateDeleted,
			}
	}

	// (Optional) Job.
	var jobStopped = false
	var jobSucceeded = false
	var jobFailed = false
	var jobCancelled = false
	var observedJob = observed.job
	var recordedJobStatus = recorded.Components.Job
	var jobStatus *v1beta1.JobStatus
	if !isComponentUpdated(observedJob, *observed.cluster) && observed.job == nil {
		jobStatus = &v1beta1.JobStatus{}
		recorded.Components.Job.DeepCopyInto(jobStatus)
		jobStatus.State = v1beta1.JobStateUpdating
	} else if observedJob != nil {
		jobStatus = &v1beta1.JobStatus{}
		if recordedJobStatus != nil {
			recordedJobStatus.DeepCopyInto(jobStatus)
		}
		jobStatus.Name = observedJob.ObjectMeta.Name
		jobStatus.FromSavepoint = getFromSavepoint(observedJob.Spec)
		var flinkJobID = updater.getFlinkJobID()
		if flinkJobID != nil {
			jobStatus.ID = *flinkJobID
		}
		if observedJob.Status.Failed > 0 {
			jobStatus.State = v1beta1.JobStateFailed
			jobStopped = true
			jobFailed = true
		} else if observedJob.Status.Succeeded > 0 {
			jobStatus.State = v1beta1.JobStateSucceeded
			jobStopped = true
			jobSucceeded = true
		} else {
			// When job status is Active, it is possible that the pod is still
			// Pending (for scheduling), so we use Flink job ID to determine
			// the actual state.
			if flinkJobID == nil {
				jobStatus.State = v1beta1.JobStatePending
			} else {
				jobStatus.State = v1beta1.JobStateRunning
			}
			if recordedJobStatus != nil &&
				(recordedJobStatus.State == v1beta1.JobStateFailed ||
					recordedJobStatus.State == v1beta1.JobStateCancelled) {
				jobStatus.RestartCount++
			}
		}
	} else if recordedJobStatus != nil {
		jobStopped = true
		jobStatus = recordedJobStatus.DeepCopy()
		if isJobCancelRequested(*observed.cluster) || jobStatus.State == v1beta1.JobStateCancelled {
			jobStatus.State = v1beta1.JobStateCancelled
			jobCancelled = true
		}
	}
	if jobStatus != nil && observed.savepoint != nil && observed.savepoint.IsSuccessful() {
		jobStatus.SavepointGeneration++
		jobStatus.LastSavepointTriggerID = observed.savepoint.TriggerID
		jobStatus.SavepointLocation = observed.savepoint.Location
		setTimestamp(&jobStatus.LastSavepointTime)
	}
	status.Components.Job = jobStatus

	// Derive the new cluster state.
	switch recorded.State {
	case "", v1beta1.ClusterStateCreating:
		if runningComponents < totalComponents {
			status.State = v1beta1.ClusterStateCreating
		} else {
			status.State = v1beta1.ClusterStateRunning
		}
	case v1beta1.ClusterStateRunning,
		v1beta1.ClusterStateReconciling:
		if jobStopped && !isUpdateTriggered(*recorded) {
			var policy = observed.cluster.Spec.Job.CleanupPolicy
			if jobSucceeded &&
				policy.AfterJobSucceeds != v1beta1.CleanupActionKeepCluster {
				status.State = v1beta1.ClusterStateStopping
			} else if jobFailed &&
				policy.AfterJobFails != v1beta1.CleanupActionKeepCluster {
				status.State = v1beta1.ClusterStateStopping
			} else if jobCancelled &&
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
		if runningComponents == 0 {
			status.State = v1beta1.ClusterStateStopped
		} else if runningComponents < totalComponents {
			status.State = v1beta1.ClusterStatePartiallyStopped
		} else {
			status.State = v1beta1.ClusterStateStopping
		}
	case v1beta1.ClusterStateStopped:
		if isUpdateTriggered(*recorded) {
			status.State = v1beta1.ClusterStateReconciling
		} else {
			status.State = v1beta1.ClusterStateStopped
		}
	default:
		panic(fmt.Sprintf("Unknown cluster state: %v", recorded.State))
	}

	// Savepoint status
	// update savepoint status if it is in progress
	if recorded.Savepoint != nil {
		var newSavepointStatus = recorded.Savepoint.DeepCopy()
		if recorded.Savepoint.State == v1beta1.SavepointStateInProgress && observed.savepoint != nil {
			switch {
			case observed.savepoint.IsSuccessful():
				newSavepointStatus.State = v1beta1.SavepointStateSucceeded
			case observed.savepoint.IsFailed():
				var msg string
				newSavepointStatus.State = v1beta1.SavepointStateFailed
				if observed.savepoint.FailureCause.StackTrace != "" {
					msg = fmt.Sprintf("Savepoint error: %v", observed.savepoint.FailureCause.StackTrace)
				} else if observed.savepointErr != nil {
					msg = fmt.Sprintf("Failed to get triggered savepoint status: %v", observed.savepointErr)
				} else {
					msg = "Failed to get triggered savepoint status"
				}
				if len(msg) > 1024 {
					msg = msg[:1024] + "..."
				}
				newSavepointStatus.Message = msg
			}
		}
		if newSavepointStatus.State == v1beta1.SavepointStateNotTriggered || newSavepointStatus.State == v1beta1.SavepointStateInProgress {
			if savepointTimeout(newSavepointStatus) {
				newSavepointStatus.State = v1beta1.SavepointStateFailed
				newSavepointStatus.Message = "Timed out taking savepoint"
			} else if isJobStopped(recorded.Components.Job) {
				newSavepointStatus.Message = "Flink job is stopped"
				newSavepointStatus.State = v1beta1.SavepointStateFailed
			} else if flinkJobID := updater.getFlinkJobID(); flinkJobID == nil ||
				(recorded.Savepoint.TriggerID != "" && *flinkJobID != recorded.Savepoint.JobID) {
				newSavepointStatus.Message = "There is no Flink job to create savepoint"
				newSavepointStatus.State = v1beta1.SavepointStateFailed
			}
		}
		status.Savepoint = newSavepointStatus
	}

	// User requested control
	var userControl = observed.cluster.Annotations[v1beta1.ControlAnnotation]

	// update job control status in progress
	var controlStatus *v1beta1.FlinkClusterControlStatus
	if recorded.Control != nil && userControl == recorded.Control.Name &&
		recorded.Control.State == v1beta1.ControlStateProgressing {
		controlStatus = recorded.Control.DeepCopy()
		var savepointStatus = status.Savepoint
		switch recorded.Control.Name {
		case v1beta1.ControlNameJobCancel:
			if observed.job == nil && status.Components.Job.State == v1beta1.JobStateCancelled {
				controlStatus.State = v1beta1.ControlStateSucceeded
				setTimestamp(&controlStatus.UpdateTime)
			} else if isJobTerminated(observed.cluster.Spec.Job.RestartPolicy, recorded.Components.Job) {
				controlStatus.Message = "Aborted job cancellation: Job is terminated."
				controlStatus.State = v1beta1.ControlStateFailed
				setTimestamp(&controlStatus.UpdateTime)
			} else if savepointStatus != nil && savepointStatus.State == v1beta1.SavepointStateFailed {
				controlStatus.Message = "Aborted job cancellation: failed to create savepoint."
				controlStatus.State = v1beta1.ControlStateFailed
				setTimestamp(&controlStatus.UpdateTime)
			} else if recorded.Control.Message != "" {
				controlStatus.State = v1beta1.ControlStateFailed
				setTimestamp(&controlStatus.UpdateTime)
			}
		case v1beta1.ControlNameSavepoint:
			if savepointStatus != nil {
				if savepointStatus.State == v1beta1.SavepointStateSucceeded {
					controlStatus.State = v1beta1.ControlStateSucceeded
					setTimestamp(&controlStatus.UpdateTime)
				} else if savepointStatus.State == v1beta1.SavepointStateFailed || savepointStatus.State == v1beta1.SavepointStateTriggerFailed {
					controlStatus.State = v1beta1.ControlStateFailed
					setTimestamp(&controlStatus.UpdateTime)
				}
			}
		}
		// aborted by max retry reach
		var retries = controlStatus.Details[ControlRetries]
		if retries == ControlMaxRetries {
			controlStatus.Message = fmt.Sprintf("Aborted control %v. The maximum number of retries has been reached.", controlStatus.Name)
			controlStatus.State = v1beta1.ControlStateFailed
			setTimestamp(&controlStatus.UpdateTime)
		}
	} else if userControl != "" {
		// Handle new user control.
		updater.log.Info("New user control requested: " + userControl)
		if userControl != v1beta1.ControlNameJobCancel && userControl != v1beta1.ControlNameSavepoint {
			if userControl != "" {
				updater.log.Info(fmt.Sprintf(v1beta1.InvalidControlAnnMsg, v1beta1.ControlAnnotation, userControl))
			}
		} else if recorded.Control != nil && recorded.Control.State == v1beta1.ControlStateProgressing {
			updater.log.Info(fmt.Sprintf(v1beta1.ControlChangeWarnMsg, v1beta1.ControlAnnotation), "current control", recorded.Control.Name, "new control", userControl)
		} else {
			switch userControl {
			case v1beta1.ControlNameSavepoint:
				if observed.cluster.Spec.Job.SavepointsDir == nil || *observed.cluster.Spec.Job.SavepointsDir == "" {
					updater.log.Info(fmt.Sprintf(v1beta1.InvalidSavepointDirMsg, v1beta1.ControlAnnotation))
					break
				} else if isJobStopped(observed.cluster.Status.Components.Job) {
					updater.log.Info(fmt.Sprintf(v1beta1.InvalidJobStateForSavepointMsg, v1beta1.ControlAnnotation))
					break
				}
				// Clear status for new savepoint
				status.Savepoint = &v1beta1.SavepointStatus{
					State:         v1beta1.SavepointStateNotTriggered,
					TriggerReason: v1beta1.SavepointTriggerReasonUserRequested,
				}
				controlStatus = getNewUserControlStatus(userControl)
			case v1beta1.ControlNameJobCancel:
				if isJobTerminated(observed.cluster.Spec.Job.RestartPolicy, recorded.Components.Job) {
					updater.log.Info(fmt.Sprintf(v1beta1.InvalidJobStateForJobCancelMsg, v1beta1.ControlAnnotation))
					break
				}
				// Savepoint for job-cancel
				var observedSavepoint = observed.cluster.Status.Savepoint
				if observedSavepoint == nil ||
					(observedSavepoint.State != v1beta1.SavepointStateInProgress && observedSavepoint.State != v1beta1.SavepointStateNotTriggered) {
					updater.log.Info("There is no savepoint in progress. Trigger savepoint in reconciler.")
					status.Savepoint = &v1beta1.SavepointStatus{
						State:         v1beta1.SavepointStateNotTriggered,
						TriggerReason: v1beta1.SavepointTriggerReasonJobCancel,
					}
				} else {
					updater.log.Info("There is a savepoint in progress. Skip new savepoint.")
				}
				controlStatus = getNewUserControlStatus(userControl)
			}
		}
	}
	// Maintain control status if there is no change.
	if recorded.Control != nil && controlStatus == nil {
		controlStatus = recorded.Control.DeepCopy()
	}
	status.Control = controlStatus

	// Handle update.
	var savepointForJobUpdate *v1beta1.SavepointStatus
	if isUpdateTriggered(*recorded) {
		switch getUpdateState(*observed) {
		case UpdateStateStoppingJob:
			// Even if savepoint has been created for update already, we check the age of savepoint continually.
			// If created savepoint is old and savepoint can be triggered, we should take savepoint again.
			// (e.g., for the case update is not progressed by accidents like network partition)
			if !isSavepointUpToDate(observed.observeTime, *jobStatus) &&
				canTakeSavepoint(*observed.cluster) &&
				(recorded.Savepoint == nil || recorded.Savepoint.State != v1beta1.SavepointStateNotTriggered) {
				savepointForJobUpdate = &v1beta1.SavepointStatus{
					State:         v1beta1.SavepointStateNotTriggered,
					TriggerReason: v1beta1.SavepointTriggerReasonUpdate,
				}
				updater.log.Info("Savepoint will be triggered for job update")
			} else if recorded.Savepoint != nil && recorded.Savepoint.State == v1beta1.SavepointStateInProgress {
				updater.log.Info("Savepoint is in progress")
			} else {
				updater.log.Info("Stopping job")
			}
		case UpdateStateUpdating:
			updater.log.Info("Updating cluster")
		case UpdateStateFinished:
			status.CurrentRevision = observed.cluster.Status.NextRevision
			updater.log.Info("Finished update")
		}
	}
	if savepointForJobUpdate != nil {
		status.Savepoint = savepointForJobUpdate
	}

	return status
}

// Gets Flink job ID based on the observed state and the recorded state.
//
// It is possible that the recorded is not nil, but the observed is, due
// to transient error or being skiped as an optimization.
func (updater *ClusterStatusUpdater) getFlinkJobID() *string {
	// Observed.
	var observedID = updater.observed.flinkJobID
	if observedID != nil && len(*observedID) > 0 {
		return observedID
	}

	// Recorded.
	var recordedJobStatus = updater.observed.cluster.Status.Components.Job
	if recordedJobStatus != nil && len(recordedJobStatus.ID) > 0 {
		return &recordedJobStatus.ID
	}

	return nil
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
	if newStatus.Components.JobManagerDeployment !=
		currentStatus.Components.JobManagerDeployment {
		updater.log.Info(
			"JobManager deployment status changed",
			"current", currentStatus.Components.JobManagerDeployment,
			"new",
			newStatus.Components.JobManagerDeployment)
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
	if newStatus.Components.TaskManagerDeployment !=
		currentStatus.Components.TaskManagerDeployment {
		updater.log.Info(
			"TaskManager deployment status changed",
			"current",
			currentStatus.Components.TaskManagerDeployment,
			"new",
			newStatus.Components.TaskManagerDeployment)
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
		!reflect.DeepEqual(newStatus.CollisionCount, currentStatus.CollisionCount) {
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

func getDeploymentState(deployment *appsv1.Deployment) string {
	if deployment.Status.AvailableReplicas >= *deployment.Spec.Replicas {
		return v1beta1.ComponentStateReady
	}
	return v1beta1.ComponentStateNotReady
}

// syncRevisionStatus synchronizes current FlinkCluster resource and its child ControllerRevision resources.
// When FlinkCluster resource is edited, the operator creates new child ControllerRevision for it
// and updates nextRevision in FlinkClusterStatus to the name of the new ControllerRevision.
// At that time, the name of the ControllerRevision is composed with the hash string generated
// from the FlinkClusterSpec which is to be stored in it.
// Therefore the contents of the ControllerRevision resources are maintained not duplicate.
// If edited FlinkClusterSpec is the same with the content of any existing ControllerRevision resources,
// the operator will only update nextRevision of the FlinkClusterStatus to the name of the ControllerRevision
// that has the same content, instead of creating new ControllerRevision.
// Finally, it maintains the number of child ControllerRevision resources according to RevisionHistoryLimit.
func (updater *ClusterStatusUpdater) syncRevisionStatus(status *v1beta1.FlinkClusterStatus) error {
	var revisions = updater.observed.revisions
	var cluster = updater.observed.cluster
	var currentRevision, nextRevision *appsv1.ControllerRevision
	var controllerHistory = updater.history

	revisionCount := len(revisions)
	history.SortControllerRevisions(revisions)

	// Use a local copy of cluster.Status.CollisionCount to avoid modifying cluster.Status directly.
	// This copy is returned so the value gets carried over to cluster.Status in updateStatefulcluster.
	var collisionCount int32
	if cluster.Status.CollisionCount != nil {
		collisionCount = *cluster.Status.CollisionCount
	}

	// create a new revision from the current cluster
	nextRevision, err := newRevision(cluster, getNextRevisionNumber(revisions), &collisionCount)
	if err != nil {
		return err
	}

	// find any equivalent revisions
	equalRevisions := history.FindEqualRevisions(revisions, nextRevision)
	equalCount := len(equalRevisions)
	if equalCount > 0 && history.EqualRevision(revisions[revisionCount-1], equalRevisions[equalCount-1]) {
		// if the equivalent revision is immediately prior the update revision has not changed
		nextRevision = revisions[revisionCount-1]
	} else if equalCount > 0 {
		// if the equivalent revision is not immediately prior we will roll back by incrementing the
		// Revision of the equivalent revision
		nextRevision, err = controllerHistory.UpdateControllerRevision(
			equalRevisions[equalCount-1],
			nextRevision.Revision)
		if err != nil {
			return err
		}
	} else {
		//if there is no equivalent revision we create a new one
		nextRevision, err = controllerHistory.CreateControllerRevision(cluster, nextRevision, &collisionCount)
		if err != nil {
			return err
		}
	}

	// if the current revision is nil we initialize the history by setting it to the update revision
	if len(cluster.Status.CurrentRevision) == 0 {
		currentRevision = nextRevision
		// attempt to find the revision that corresponds to the current revision
	} else {
		for i := range revisions {
			if revisions[i].Name == cluster.Status.CurrentRevision {
				currentRevision = revisions[i]
				break
			}
		}
	}

	// Revision status
	if status.CurrentRevision == "" {
		status.CurrentRevision = currentRevision.Name
	}
	status.NextRevision = nextRevision.Name
	status.CollisionCount = &collisionCount

	// maintain the revision history limit
	err = updater.truncateHistory()
	if err != nil {
		return err
	}

	return nil
}

func (updater *ClusterStatusUpdater) truncateHistory() error {
	var cluster = updater.observed.cluster
	var revisions = updater.observed.revisions
	// TODO: default limit
	var historyLimit int
	if cluster.Spec.RevisionHistoryLimit != nil {
		historyLimit = int(*cluster.Spec.RevisionHistoryLimit)
	} else {
		historyLimit = 10
	}

	history := make([]*appsv1.ControllerRevision, 0, len(revisions))
	historyLen := len(history)
	if historyLen <= historyLimit {
		return nil
	}
	// delete any non-live history to maintain the revision limit.
	history = history[:(historyLen - historyLimit)]
	for i := 0; i < len(history); i++ {
		if err := updater.history.DeleteControllerRevision(history[i]); err != nil {
			return err
		}
	}
	return nil
}
