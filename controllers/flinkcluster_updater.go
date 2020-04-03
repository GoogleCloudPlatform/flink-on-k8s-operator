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
	"k8s.io/apimachinery/pkg/types"
	"reflect"
	"strconv"
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

	// Adjust control annotation
	updater.adjustControlAnnotation(newStatus.Control)

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

	// Control.
	if (oldStatus.Control == nil && newStatus.Control != nil) ||
		(oldStatus.Control != nil && newStatus.Control != nil &&
			(oldStatus.Control.State == v1beta1.ControlStateFailed || oldStatus.Control.State == v1beta1.ControlStateSucceeded)) {
		updater.createStatusChangeEvent(fmt.Sprintf("Control(%v)", newStatus.Control.Name), "", newStatus.Control.State)
	} else if oldStatus.Control != nil && newStatus.Control != nil &&
		oldStatus.Control.State != newStatus.Control.State {
		updater.createStatusChangeEvent(fmt.Sprintf("Control(%v)", newStatus.Control.Name), oldStatus.Control.State, newStatus.Control.State)
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

	// ConfigMap.
	var observedConfigMap = observed.configMap
	if observedConfigMap != nil {
		status.Components.ConfigMap.Name =
			observedConfigMap.ObjectMeta.Name
		status.Components.ConfigMap.State =
			v1beta1.ComponentStateReady
	} else if recorded.Components.ConfigMap.Name != "" {
		status.Components.ConfigMap =
			v1beta1.FlinkClusterComponentState{
				Name:  recorded.Components.ConfigMap.Name,
				State: v1beta1.ComponentStateDeleted,
			}
	}

	// JobManager deployment.
	var observedJmDeployment = observed.jmDeployment
	if observedJmDeployment != nil {
		status.Components.JobManagerDeployment.Name =
			observedJmDeployment.ObjectMeta.Name
		status.Components.JobManagerDeployment.State =
			getDeploymentState(observedJmDeployment)
		if status.Components.JobManagerDeployment.State ==
			v1beta1.ComponentStateReady {
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
	if observedJmService != nil {
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
	if observedJmIngress != nil {
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
	if observedTmDeployment != nil {
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
	if observedJob != nil {
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
			if recordedJobStatus != nil && (recordedJobStatus.State ==
				v1beta1.JobStateFailed ||
				recordedJobStatus.State == v1beta1.JobStateCancelled) {
				jobStatus.RestartCount++
			}
		}
	} else if recordedJobStatus != nil {
		jobStatus = recordedJobStatus.DeepCopy()
		jobStopped = true
		var cancelRequested = observed.cluster.Spec.Job.CancelRequested
		if (cancelRequested != nil && *cancelRequested) ||
			(observed.cluster.Status.Control != nil && observed.cluster.Status.Control.Name == v1beta1.ControlNameCancel) {
			jobStatus.State = v1beta1.JobStateCancelled
			jobCancelled = true
		}
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
		if jobStopped {
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
		status.State = v1beta1.ClusterStateStopped
	default:
		panic(fmt.Sprintf("Unknown cluster state: %v", recorded.State))
	}

	// User requested control
	var desiredControl = observed.cluster.Annotations[v1beta1.ControlDesiredAnnotation]

	// update control status in progress
	if recorded.Control != nil && desiredControl == recorded.Control.Name &&
		recorded.Control.State == v1beta1.ControlStateProgressing {
		switch recorded.Control.Name {
		case v1beta1.ControlNameCancel:
			if observed.job == nil && status.Components.Job.State == v1beta1.JobStateCancelled {
				status.Control = new(v1beta1.FlinkClusterControlState)
				status.Control = recorded.Control.DeepCopy()
				status.Control.State = v1beta1.ControlStateSucceeded
				setTimestamp(&status.Control.UpdateTime)
			}
		case v1beta1.ControlNameSavepoint:
			status.Control = new(v1beta1.FlinkClusterControlState)
			status.Control = recorded.Control.DeepCopy()
			setTimestamp(&status.Control.UpdateTime)
			lastSavepointGeneration, err := strconv.Atoi(recorded.Control.Data["lastSavepointGeneration"])
			if err == nil {
				if lastSavepointGeneration < int(jobStatus.SavepointGeneration) {
					status.Control.State = v1beta1.ControlStateSucceeded
				} else {
					updater.log.Info("savepointing is not finished yet.")
				}
			} else {
				updater.log.Error(err, "failed to get status.control.data[\"lastSavepointGeneration\"] for control state update.")
			}
		}
	} else {
		// handle new desired control
		if desiredControl != v1beta1.ControlNameCancel && desiredControl != v1beta1.ControlNameSavepoint {
			if desiredControl != "" {
				updater.log.Error(errors.New("control name is not valid"), "control name", "new control", desiredControl)
			}
		} else if recorded.Control != nil && recorded.Control.State == v1beta1.ControlStateProgressing {
			updater.log.Error(errors.New("control change is not allowed while it is in progress"), "current control", recorded.Control.Name, "new control", desiredControl)
		} else {
			status.Control = new(v1beta1.FlinkClusterControlState)
			status.Control.Name = desiredControl
			status.Control.State = v1beta1.ControlStateProgressing
			setTimestamp(&status.Control.UpdateTime)
			switch status.Control.Name {
			case v1beta1.ControlNameSavepoint:
				status.Control.Data = make(map[string]string)
				status.Control.Data["lastSavepointGeneration"] = strconv.Itoa(int(jobStatus.SavepointGeneration))
			}
		}
	}
	// maintain control status if there is no change
	if recorded.Control != nil && status.Control == nil {
		status.Control = new(v1beta1.FlinkClusterControlState)
		status.Control = recorded.Control.DeepCopy()
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
	return changed
}

func (updater *ClusterStatusUpdater) updateClusterStatus(
	status v1beta1.FlinkClusterStatus) error {
	var cluster = v1beta1.FlinkCluster{}
	updater.observed.cluster.DeepCopyInto(&cluster)
	cluster.Status = status
	return updater.k8sClient.Status().Update(updater.context, &cluster)
}

func getDeploymentState(deployment *appsv1.Deployment) string {
	if deployment.Status.AvailableReplicas >= *deployment.Spec.Replicas {
		return v1beta1.ComponentStateReady
	}
	return v1beta1.ComponentStateNotReady
}

// Clear finished or improper desired control in annotations
func (updater *ClusterStatusUpdater) adjustControlAnnotation(newControlStatus *v1beta1.FlinkClusterControlState) error {
	var desiredControl = updater.observed.cluster.Annotations[v1beta1.ControlDesiredAnnotation]
	if desiredControl == "" {
		return nil
	}
	if newControlStatus == nil || // refused control in updater
		desiredControl != newControlStatus.Name || // refused control in updater
		(desiredControl == newControlStatus.Name && // finished control
			newControlStatus.State == v1beta1.ControlStateSucceeded ||
			newControlStatus.State == v1beta1.ControlStateFailed) {
		// make annotation patch cleared
		annotationPatch := objectForPatch{
			Metadata: objectMetaForPatch{
				Annotations: map[string]interface{}{
					v1beta1.ControlDesiredAnnotation: nil,
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
