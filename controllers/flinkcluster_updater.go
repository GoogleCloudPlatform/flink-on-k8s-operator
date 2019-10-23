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
	"reflect"
	"time"

	"github.com/go-logr/logr"
	v1alpha1 "github.com/googlecloudplatform/flink-operator/api/v1alpha1"
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
	var oldStatus = v1alpha1.FlinkClusterStatus{}
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
	oldStatus v1alpha1.FlinkClusterStatus,
	newStatus v1alpha1.FlinkClusterStatus) {
	if oldStatus.Components.JobManagerDeployment.State !=
		newStatus.Components.JobManagerDeployment.State {
		updater.createStatusChangeEvent(
			"JobManager deployment",
			oldStatus.Components.JobManagerDeployment.State,
			newStatus.Components.JobManagerDeployment.State)
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
	recorded *v1alpha1.FlinkClusterStatus,
	observed *ObservedClusterState) v1alpha1.FlinkClusterStatus {
	var status = v1alpha1.FlinkClusterStatus{}
	var runningComponents = 0
	// jmDeployment, jmService, tmDeployment.
	var totalComponents = 3

	// JobManager deployment.
	var observedJmDeployment = observed.jmDeployment
	if observedJmDeployment != nil {
		status.Components.JobManagerDeployment.Name =
			observedJmDeployment.ObjectMeta.Name
		status.Components.JobManagerDeployment.State =
			getDeploymentState(observedJmDeployment)
		if status.Components.JobManagerDeployment.State ==
			v1alpha1.ComponentState.Ready {
			runningComponents++
		}
	} else if recorded.Components.JobManagerDeployment.Name != "" {
		status.Components.JobManagerDeployment =
			v1alpha1.FlinkClusterComponentState{
				Name:  recorded.Components.JobManagerDeployment.Name,
				State: v1alpha1.ComponentState.Deleted,
			}
	}

	// JobManager service.
	var observedJmService = observed.jmService
	if observedJmService != nil {
		var state string
		if observedJmService.Spec.Type == corev1.ServiceTypeClusterIP {
			if observedJmService.Spec.ClusterIP != "" {
				state = v1alpha1.ComponentState.Ready
				runningComponents++
			} else {
				state = v1alpha1.ComponentState.NotReady
			}
		} else if observedJmService.Spec.Type == corev1.ServiceTypeLoadBalancer {
			if len(observedJmService.Status.LoadBalancer.Ingress) > 0 {
				state = v1alpha1.ComponentState.Ready
				runningComponents++
			} else {
				state = v1alpha1.ComponentState.NotReady
			}
		}
		status.Components.JobManagerService =
			v1alpha1.FlinkClusterComponentState{
				Name:  observedJmService.ObjectMeta.Name,
				State: state,
			}
	} else if recorded.Components.JobManagerService.Name != "" {
		status.Components.JobManagerService =
			v1alpha1.FlinkClusterComponentState{
				Name:  recorded.Components.JobManagerService.Name,
				State: v1alpha1.ComponentState.Deleted,
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
			state = v1alpha1.ComponentState.Ready
		} else {
			state = v1alpha1.ComponentState.NotReady
		}

		status.Components.JobManagerIngress =
			&v1alpha1.JobManagerIngressStatus{
				Name:  observedJmIngress.ObjectMeta.Name,
				State: state,
				URLs:  urls,
			}
	} else if recorded.Components.JobManagerIngress != nil &&
		recorded.Components.JobManagerIngress.Name != "" {
		status.Components.JobManagerIngress =
			&v1alpha1.JobManagerIngressStatus{
				Name:  recorded.Components.JobManagerIngress.Name,
				State: v1alpha1.ComponentState.Deleted,
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
			v1alpha1.ComponentState.Ready {
			runningComponents++
		}
	} else if recorded.Components.TaskManagerDeployment.Name != "" {
		status.Components.TaskManagerDeployment =
			v1alpha1.FlinkClusterComponentState{
				Name:  recorded.Components.TaskManagerDeployment.Name,
				State: v1alpha1.ComponentState.Deleted,
			}
	}

	// (Optional) Job.
	var jobFinished = false
	var observedJob = observed.job
	if observedJob != nil {
		status.Components.Job = &v1alpha1.JobStatus{}
		var recordedJobStatus = observed.cluster.Status.Components.Job
		if recordedJobStatus != nil {
			recordedJobStatus.DeepCopyInto(status.Components.Job)
		}
		status.Components.Job.Name = observedJob.ObjectMeta.Name

		var flinkJobID = updater.getFlinkJobID()
		if flinkJobID != nil {
			status.Components.Job.ID = *flinkJobID
		}

		if observedJob.Status.Active > 0 {
			// When job status is Active, it is possible that the pod is still
			// Pending (for scheduling), so we use Flink job ID to determine
			// the actual state.
			if flinkJobID == nil {
				status.Components.Job.State = v1alpha1.JobState.Pending
			} else {
				status.Components.Job.State = v1alpha1.JobState.Running
			}
		} else if observedJob.Status.Failed > 0 {
			status.Components.Job.State = v1alpha1.JobState.Failed
			jobFinished = true
		} else if observedJob.Status.Succeeded > 0 {
			status.Components.Job.State = v1alpha1.JobState.Succeeded
			jobFinished = true
		} else {
			status.Components.Job = nil
		}
	}

	// Derive the new cluster state.
	switch recorded.State {
	case "", v1alpha1.ClusterState.Creating:
		if runningComponents < totalComponents {
			status.State = v1alpha1.ClusterState.Creating
		} else {
			status.State = v1alpha1.ClusterState.Running
		}
	case v1alpha1.ClusterState.Running,
		v1alpha1.ClusterState.Reconciling:
		if jobFinished {
			status.State = v1alpha1.ClusterState.Stopping
		} else if runningComponents < totalComponents {
			status.State = v1alpha1.ClusterState.Reconciling
		} else {
			status.State = v1alpha1.ClusterState.Running
		}
	case v1alpha1.ClusterState.Stopping:
		if runningComponents == 0 {
			status.State = v1alpha1.ClusterState.Stopped
		} else {
			status.State = v1alpha1.ClusterState.Stopping
		}
	case v1alpha1.ClusterState.Stopped:
		status.State = v1alpha1.ClusterState.Stopped
	default:
		panic(fmt.Sprintf("Unknown cluster state: %v", recorded.State))
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
	currentStatus v1alpha1.FlinkClusterStatus,
	newStatus v1alpha1.FlinkClusterStatus) bool {
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
	}
	return changed
}

func (updater *ClusterStatusUpdater) updateClusterStatus(
	status v1alpha1.FlinkClusterStatus) error {
	var flinkCluster = v1alpha1.FlinkCluster{}
	updater.observed.cluster.DeepCopyInto(&flinkCluster)
	flinkCluster.Status = status
	return updater.k8sClient.Update(updater.context, &flinkCluster)
}

func getDeploymentState(deployment *appsv1.Deployment) string {
	if deployment.Status.AvailableReplicas >= *deployment.Spec.Replicas {
		return v1alpha1.ComponentState.Ready
	}
	return v1alpha1.ComponentState.NotReady
}
