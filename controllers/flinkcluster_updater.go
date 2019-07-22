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
	"fmt"
	"time"

	"github.com/go-logr/logr"
	flinkoperatorv1alpha1 "github.com/googlecloudplatform/flink-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type _ClusterStatusUpdater struct {
	k8sClient     client.Client
	context       context.Context
	log           logr.Logger
	observedState _ObservedClusterState
}

// Compares the current status recorded in the cluster's status field and the
// new status derived from the status of the components, updates the cluster
// status if it is changed.
func (updater *_ClusterStatusUpdater) updateClusterStatusIfChanged() error {
	if updater.observedState.cluster == nil {
		updater.log.Info("The cluster has been deleted, no status to update")
		return nil
	}

	// Current status recorded in the cluster's status field.
	var currentStatus = flinkoperatorv1alpha1.FlinkClusterStatus{}
	updater.observedState.cluster.Status.DeepCopyInto(&currentStatus)
	currentStatus.LastUpdateTime = ""

	// New status derived from the cluster's components.
	var newStatus = updater.deriveClusterStatus()

	// Compare
	var changed = updater.isStatusChanged(currentStatus, newStatus)

	// Update
	if changed {
		updater.log.Info(
			"Updating status",
			"current",
			updater.observedState.cluster.Status,
			"new", newStatus)
		newStatus.LastUpdateTime = time.Now().Format(time.RFC3339)
		return updater.updateClusterStatus(newStatus)
	} else {
		updater.log.Info("No status change", "state", currentStatus.State)
	}
	return nil
}

func (updater *_ClusterStatusUpdater) deriveClusterStatus() flinkoperatorv1alpha1.FlinkClusterStatus {
	var status = flinkoperatorv1alpha1.FlinkClusterStatus{}
	var totalComponents = 3
	var readyComponents = 0
	var recordedClusterStatus = &updater.observedState.cluster.Status

	// JobManager deployment.
	var observedJmDeployment = updater.observedState.jmDeployment
	if observedJmDeployment != nil {
		status.Components.JobManagerDeployment.Name =
			observedJmDeployment.ObjectMeta.Name
		if observedJmDeployment.Status.AvailableReplicas <
			observedJmDeployment.Status.Replicas ||
			observedJmDeployment.Status.ReadyReplicas <
				observedJmDeployment.Status.Replicas {
			status.Components.JobManagerDeployment.State =
				flinkoperatorv1alpha1.ClusterComponentState.NotReady
		} else {
			status.Components.JobManagerDeployment.State =
				flinkoperatorv1alpha1.ClusterComponentState.Ready
			readyComponents++
		}
	} else if recordedClusterStatus.Components.JobManagerDeployment.Name != "" {
		status.Components.JobManagerDeployment =
			flinkoperatorv1alpha1.FlinkClusterComponentState{
				Name:  recordedClusterStatus.Components.JobManagerDeployment.Name,
				State: flinkoperatorv1alpha1.ClusterComponentState.Deleted,
			}
	}

	// JobManager service.
	var observedJmService = updater.observedState.jmService
	if observedJmService != nil {
		var state string
		if observedJmService.Spec.Type == corev1.ServiceTypeClusterIP {
			if observedJmService.Spec.ClusterIP != "" {
				state = flinkoperatorv1alpha1.ClusterComponentState.Ready
				readyComponents++
			} else {
				state = flinkoperatorv1alpha1.ClusterComponentState.NotReady
			}
		} else if observedJmService.Spec.Type == corev1.ServiceTypeLoadBalancer {
			if len(observedJmService.Status.LoadBalancer.Ingress) > 0 {
				state = flinkoperatorv1alpha1.ClusterComponentState.Ready
				readyComponents++
			} else {
				state = flinkoperatorv1alpha1.ClusterComponentState.NotReady
			}
		}
		status.Components.JobManagerService =
			flinkoperatorv1alpha1.FlinkClusterComponentState{
				Name:  observedJmService.ObjectMeta.Name,
				State: state,
			}
	} else if recordedClusterStatus.Components.JobManagerService.Name != "" {
		status.Components.JobManagerService =
			flinkoperatorv1alpha1.FlinkClusterComponentState{
				Name:  recordedClusterStatus.Components.JobManagerService.Name,
				State: flinkoperatorv1alpha1.ClusterComponentState.Deleted,
			}
	}

	// TaskManager deployment.
	var observedTmDeployment = updater.observedState.tmDeployment
	if observedTmDeployment != nil {
		status.Components.TaskManagerDeployment.Name =
			observedTmDeployment.ObjectMeta.Name
		if observedTmDeployment.Status.AvailableReplicas <
			observedTmDeployment.Status.Replicas ||
			observedTmDeployment.Status.ReadyReplicas <
				observedTmDeployment.Status.Replicas {
			status.Components.TaskManagerDeployment.State =
				flinkoperatorv1alpha1.ClusterComponentState.NotReady
		} else {
			status.Components.TaskManagerDeployment.State =
				flinkoperatorv1alpha1.ClusterComponentState.Ready
			readyComponents++
		}
	} else if recordedClusterStatus.Components.TaskManagerDeployment.Name != "" {
		status.Components.TaskManagerDeployment =
			flinkoperatorv1alpha1.FlinkClusterComponentState{
				Name:  recordedClusterStatus.Components.TaskManagerDeployment.Name,
				State: flinkoperatorv1alpha1.ClusterComponentState.Deleted,
			}
	}

	// (Optional) Job.
	var jobFinished = false
	var observedJob = updater.observedState.job
	if observedJob != nil {
		status.Components.Job = new(flinkoperatorv1alpha1.JobStatus)
		status.Components.Job.Name = observedJob.ObjectMeta.Name
		if observedJob.Status.Active > 0 {
			status.Components.Job.State = flinkoperatorv1alpha1.JobState.Running
		} else if observedJob.Status.Failed > 0 {
			status.Components.Job.State = flinkoperatorv1alpha1.JobState.Failed
			jobFinished = true
		} else if observedJob.Status.Succeeded > 0 {
			status.Components.Job.State = flinkoperatorv1alpha1.JobState.Succeeded
			jobFinished = true
		} else {
			status.Components.Job.State = flinkoperatorv1alpha1.JobState.Unknown
		}
	}

	// Derive the next state.
	switch recordedClusterStatus.State {
	case "", flinkoperatorv1alpha1.ClusterState.Creating:
		if readyComponents < totalComponents {
			status.State = flinkoperatorv1alpha1.ClusterState.Creating
		} else {
			status.State = flinkoperatorv1alpha1.ClusterState.Running
		}
	case flinkoperatorv1alpha1.ClusterState.Running,
		flinkoperatorv1alpha1.ClusterState.Reconciling:
		if readyComponents < totalComponents {
			status.State = flinkoperatorv1alpha1.ClusterState.Reconciling
		} else if jobFinished {
			status.State = flinkoperatorv1alpha1.ClusterState.Stopping
		} else {
			status.State = flinkoperatorv1alpha1.ClusterState.Running
		}
	case flinkoperatorv1alpha1.ClusterState.Stopping:
		if readyComponents == 0 {
			status.State = flinkoperatorv1alpha1.ClusterState.Stopped
		} else {
			status.State = flinkoperatorv1alpha1.ClusterState.Stopping
		}
	case flinkoperatorv1alpha1.ClusterState.Stopped:
		status.State = flinkoperatorv1alpha1.ClusterState.Stopped
	default:
		panic(fmt.Sprintf("Unknown cluster state: %v", recordedClusterStatus.State))
	}

	return status
}

func (updater *_ClusterStatusUpdater) isStatusChanged(
	currentStatus flinkoperatorv1alpha1.FlinkClusterStatus,
	newStatus flinkoperatorv1alpha1.FlinkClusterStatus) bool {
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
		if *newStatus.Components.Job != *currentStatus.Components.Job {
			updater.log.Info(
				"Job status changed",
				"current",
				*currentStatus.Components.Job,
				"new",
				*newStatus.Components.Job)
			changed = true
		}
	}
	return changed
}

func (updater *_ClusterStatusUpdater) updateClusterStatus(
	status flinkoperatorv1alpha1.FlinkClusterStatus) error {
	var flinkCluster = flinkoperatorv1alpha1.FlinkCluster{}
	updater.observedState.cluster.DeepCopyInto(&flinkCluster)
	flinkCluster.Status = status
	return updater.k8sClient.Update(updater.context, &flinkCluster)
}
