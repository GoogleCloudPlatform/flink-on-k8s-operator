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
	"context"
	"fmt"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	"time"

	"github.com/go-logr/logr"
	v1beta1 "github.com/googlecloudplatform/flink-operator/api/v1beta1"
	"github.com/googlecloudplatform/flink-operator/controllers/flinkclient"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ClusterReconciler takes actions to drive the observed state towards the
// desired state.
type ClusterReconciler struct {
	k8sClient   client.Client
	flinkClient flinkclient.FlinkClient
	context     context.Context
	log         logr.Logger
	observed    ObservedClusterState
	desired     DesiredClusterState
	recorder    record.EventRecorder
}

var requeueResult = ctrl.Result{RequeueAfter: 10 * time.Second, Requeue: true}

// Compares the desired state and the observed state, if there is a difference,
// takes actions to drive the observed state towards the desired state.
func (reconciler *ClusterReconciler) reconcile() (ctrl.Result, error) {
	var err error

	// Child resources of the cluster CR will be automatically reclaimed by K8S.
	if reconciler.observed.cluster == nil {
		reconciler.log.Info("The cluster has been deleted, no action to take")
		return ctrl.Result{}, nil
	}

	err = reconciler.reconcileConfigMap()
	if err != nil {
		return ctrl.Result{}, err
	}

	err = reconciler.reconcileJobManagerDeployment()
	if err != nil {
		return ctrl.Result{}, err
	}

	err = reconciler.reconcileJobManagerService()
	if err != nil {
		return ctrl.Result{}, err
	}

	err = reconciler.reconcileJobManagerIngress()
	if err != nil {
		return ctrl.Result{}, err
	}

	err = reconciler.reconcileTaskManagerDeployment()
	if err != nil {
		return ctrl.Result{}, err
	}

	result, err := reconciler.reconcileJob()

	return result, nil
}

func (reconciler *ClusterReconciler) reconcileJobManagerDeployment() error {
	return reconciler.reconcileDeployment(
		"JobManager",
		reconciler.desired.JmDeployment,
		reconciler.observed.jmDeployment)
}

func (reconciler *ClusterReconciler) reconcileTaskManagerDeployment() error {
	return reconciler.reconcileDeployment(
		"TaskManager",
		reconciler.desired.TmDeployment,
		reconciler.observed.tmDeployment)
}

func (reconciler *ClusterReconciler) reconcileDeployment(
	component string,
	desiredDeployment *appsv1.Deployment,
	observedDeployment *appsv1.Deployment) error {
	var log = reconciler.log.WithValues("component", component)

	if desiredDeployment != nil && observedDeployment == nil {
		return reconciler.createDeployment(desiredDeployment, component)
	}

	if desiredDeployment != nil && observedDeployment != nil {
		log.Info("Deployment already exists, no action")
		return nil
		// TODO(dagang): compare and update if needed.
	}

	if desiredDeployment == nil && observedDeployment != nil {
		return reconciler.deleteDeployment(observedDeployment, component)
	}

	return nil
}

func (reconciler *ClusterReconciler) createDeployment(
	deployment *appsv1.Deployment, component string) error {
	var context = reconciler.context
	var log = reconciler.log.WithValues("component", component)
	var k8sClient = reconciler.k8sClient

	log.Info("Creating deployment", "deployment", *deployment)
	var err = k8sClient.Create(context, deployment)
	if err != nil {
		log.Error(err, "Failed to create deployment")
	} else {
		log.Info("Deployment created")
	}
	return err
}

func (reconciler *ClusterReconciler) updateDeployment(
	deployment *appsv1.Deployment, component string) error {
	var context = reconciler.context
	var log = reconciler.log.WithValues("component", component)
	var k8sClient = reconciler.k8sClient

	log.Info("Updating deployment", "deployment", deployment)
	var err = k8sClient.Update(context, deployment)
	if err != nil {
		log.Error(err, "Failed to update deployment")
	} else {
		log.Info("Deployment updated")
	}
	return err
}

func (reconciler *ClusterReconciler) deleteDeployment(
	deployment *appsv1.Deployment, component string) error {
	var context = reconciler.context
	var log = reconciler.log.WithValues("component", component)
	var k8sClient = reconciler.k8sClient

	log.Info("Deleting deployment", "deployment", deployment)
	var err = k8sClient.Delete(context, deployment)
	err = client.IgnoreNotFound(err)
	if err != nil {
		log.Error(err, "Failed to delete deployment")
	} else {
		log.Info("Deployment deleted")
	}
	return err
}

func (reconciler *ClusterReconciler) reconcileJobManagerService() error {
	var desiredJmService = reconciler.desired.JmService
	var observedJmService = reconciler.observed.jmService

	if desiredJmService != nil && observedJmService == nil {
		return reconciler.createService(desiredJmService, "JobManager")
	}

	if desiredJmService != nil && observedJmService != nil {
		reconciler.log.Info("JobManager service already exists, no action")
		return nil
		// TODO(dagang): compare and update if needed.
	}

	if desiredJmService == nil && observedJmService != nil {
		return reconciler.deleteService(observedJmService, "JobManager")
		// TODO(dagang): compare and update if needed.
	}

	return nil
}

func (reconciler *ClusterReconciler) createService(
	service *corev1.Service, component string) error {
	var context = reconciler.context
	var log = reconciler.log.WithValues("component", component)
	var k8sClient = reconciler.k8sClient

	log.Info("Creating service", "resource", *service)
	var err = k8sClient.Create(context, service)
	if err != nil {
		log.Info("Failed to create service", "error", err)
	} else {
		log.Info("Service created")
	}
	return err
}

func (reconciler *ClusterReconciler) deleteService(
	service *corev1.Service, component string) error {
	var context = reconciler.context
	var log = reconciler.log.WithValues("component", component)
	var k8sClient = reconciler.k8sClient

	log.Info("Deleting service", "service", service)
	var err = k8sClient.Delete(context, service)
	err = client.IgnoreNotFound(err)
	if err != nil {
		log.Error(err, "Failed to delete service")
	} else {
		log.Info("service deleted")
	}
	return err
}

func (reconciler *ClusterReconciler) reconcileJobManagerIngress() error {
	var desiredJmIngress = reconciler.desired.JmIngress
	var observedJmIngress = reconciler.observed.jmIngress

	if desiredJmIngress != nil && observedJmIngress == nil {
		return reconciler.createIngress(desiredJmIngress, "JobManager")
	}

	if desiredJmIngress != nil && observedJmIngress != nil {
		reconciler.log.Info("JobManager ingress already exists, no action")
		return nil
		// TODO: compare and update if needed.
	}

	if desiredJmIngress == nil && observedJmIngress != nil {
		return reconciler.deleteIngress(observedJmIngress, "JobManager")
	}

	return nil
}

func (reconciler *ClusterReconciler) createIngress(
	ingress *extensionsv1beta1.Ingress, component string) error {
	var context = reconciler.context
	var log = reconciler.log.WithValues("component", component)
	var k8sClient = reconciler.k8sClient

	log.Info("Creating ingress", "resource", *ingress)
	var err = k8sClient.Create(context, ingress)
	if err != nil {
		log.Info("Failed to create ingress", "error", err)
	} else {
		log.Info("Ingress created")
	}
	return err
}

func (reconciler *ClusterReconciler) deleteIngress(
	ingress *extensionsv1beta1.Ingress, component string) error {
	var context = reconciler.context
	var log = reconciler.log.WithValues("component", component)
	var k8sClient = reconciler.k8sClient

	log.Info("Deleting ingress", "ingress", ingress)
	var err = k8sClient.Delete(context, ingress)
	err = client.IgnoreNotFound(err)
	if err != nil {
		log.Error(err, "Failed to delete ingress")
	} else {
		log.Info("Ingress deleted")
	}
	return err
}

func (reconciler *ClusterReconciler) reconcileConfigMap() error {
	var desiredConfigMap = reconciler.desired.ConfigMap
	var observedConfigMap = reconciler.observed.configMap

	if desiredConfigMap != nil && observedConfigMap == nil {
		return reconciler.createConfigMap(desiredConfigMap, "ConfigMap")
	}

	if desiredConfigMap != nil && observedConfigMap != nil {
		reconciler.log.Info("ConfigMap already exists, no action")
		return nil
		// TODO: compare and update if needed.
	}

	if desiredConfigMap == nil && observedConfigMap != nil {
		return reconciler.deleteConfigMap(observedConfigMap, "ConfigMap")
	}

	return nil
}

func (reconciler *ClusterReconciler) createConfigMap(
	cm *corev1.ConfigMap, component string) error {
	var context = reconciler.context
	var log = reconciler.log.WithValues("component", component)
	var k8sClient = reconciler.k8sClient

	log.Info("Creating configMap", "configMap", *cm)
	var err = k8sClient.Create(context, cm)
	if err != nil {
		log.Info("Failed to create configMap", "error", err)
	} else {
		log.Info("ConfigMap created")
	}
	return err
}

func (reconciler *ClusterReconciler) deleteConfigMap(
	cm *corev1.ConfigMap, component string) error {
	var context = reconciler.context
	var log = reconciler.log.WithValues("component", component)
	var k8sClient = reconciler.k8sClient

	log.Info("Deleting configMap", "configMap", cm)
	var err = k8sClient.Delete(context, cm)
	err = client.IgnoreNotFound(err)
	if err != nil {
		log.Error(err, "Failed to delete configMap")
	} else {
		log.Info("ConfigMap deleted")
	}
	return err
}

func (reconciler *ClusterReconciler) reconcileJob() (ctrl.Result, error) {
	var log = reconciler.log
	var desiredJob = reconciler.desired.Job
	var observed = reconciler.observed
	var observedJob = observed.job
	var err error

	// Update status changed via job reconciliation.
	var newSavepointStatus *v1beta1.SavepointStatus
	var newControlStatus *v1beta1.FlinkClusterControlStatus
	defer reconciler.updateStatus(&newSavepointStatus, &newControlStatus)

	// Create
	if desiredJob != nil && observedJob == nil {
		// If the observed Flink job status list is not nil (e.g., emtpy list),
		// it means Flink REST API server is up and running. It is the source of
		// truth of whether we can submit a job.
		if observed.flinkJobList == nil {
			log.Info("Waiting for Flink API server to be ready")
			return requeueResult, nil
		}

		if len(observed.flinkRunningJobIDs) > 0 {
			log.Info("Cancelling unexpected running job(s)")
			err = reconciler.cancelRunningJobs(false /* takeSavepoint */)
			return requeueResult, err
		}

		err = reconciler.createJob(desiredJob)
		return requeueResult, err
	}

	// Update or restart
	var jobID = reconciler.getFlinkJobID()
	if desiredJob != nil && observedJob != nil {
		var restartPolicy = observed.cluster.Spec.Job.RestartPolicy
		var observedJobStatus = observed.cluster.Status.Components.Job

		if shouldRestartJob(restartPolicy, observedJobStatus) {
			var err = reconciler.restartJob()
			if err != nil {
				return requeueResult, err
			}
			return ctrl.Result{}, nil
		}

		if len(jobID) > 0 {
			if ok, savepointTriggerReason := reconciler.shouldTakeSavepoint(); ok {
				newSavepointStatus, _ = reconciler.takeSavepointAsync(jobID, savepointTriggerReason)
			}
		}

		var jobStatus = reconciler.observed.cluster.Status.Components.Job
		if !isJobStopped(jobStatus) {
			log.Info("Job is not finished yet, no action", "jobID", jobID)
			return requeueResult, nil
		}

		log.Info("Job has finished, no action")
		return ctrl.Result{}, nil
	}

	// Delete
	if desiredJob == nil && observedJob != nil {
		// Cancel Flink job if it is live
		// 1) In the case of which savepoint was triggered, after it is completed, proceed to delete step.
		// 2) When savepoint was skipped, continue to next step immediately.
		//
		// If savepoint or cancellation was failed, the control state will fall to the failed in the updater.
		if len(jobID) > 0 && len(observed.flinkRunningJobIDs) == 1 {
			log.Info("Cancelling job", "jobID", jobID)
			if isSavepointTriggered(reconciler.observed.cluster.Status.Savepoint) {
				log.Info("Triggered savepoint already and wait until it is completed.")
				return requeueResult, nil
			}
			newSavepointStatus, err = reconciler.cancelFlinkJobAsync(jobID, true /* takeSavepoint */)
			if err != nil {
				log.Error(err, "Failed to cancel job", "jobID", jobID)
				newControlStatus = getFailedCancelStatus(err)
				return requeueResult, err
			}
			if newSavepointStatus != nil && newSavepointStatus.State != SavepointStateSucceeded {
				return requeueResult, nil
			}
		}
		// If there is no running job, proceed to delete the job
		var err = reconciler.deleteJob(observedJob)
		if err != nil {
			newControlStatus = getFailedCancelStatus(err)
		}
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (reconciler *ClusterReconciler) createJob(job *batchv1.Job) error {
	var context = reconciler.context
	var log = reconciler.log
	var k8sClient = reconciler.k8sClient

	log.Info("Submitting job", "resource", *job)
	var err = k8sClient.Create(context, job)
	if err != nil {
		log.Info("Failed to created job", "error", err)
	} else {
		log.Info("Job created")
	}
	return err
}

func (reconciler *ClusterReconciler) deleteJob(job *batchv1.Job) error {
	var context = reconciler.context
	var log = reconciler.log
	var k8sClient = reconciler.k8sClient

	log.Info("Deleting job", "job", job)
	var err = k8sClient.Delete(context, job)
	err = client.IgnoreNotFound(err)
	if err != nil {
		log.Error(err, "Failed to delete job")
	} else {
		log.Info("Job deleted")
	}
	return err
}

func (reconciler *ClusterReconciler) getFlinkJobID() string {
	var jobStatus = reconciler.observed.cluster.Status.Components.Job
	if jobStatus != nil && len(jobStatus.ID) > 0 {
		return jobStatus.ID
	}
	return ""
}

func (reconciler *ClusterReconciler) restartJob() error {
	var log = reconciler.log
	var observedJob = reconciler.observed.job
	var desiredJob = reconciler.desired.Job

	log.Info("Restarting job", "old", observedJob, "new", desiredJob)

	var err = reconciler.cancelRunningJobs(false /* takeSavepoint */)
	if err != nil {
		return err
	}

	if observedJob != nil {
		var err = reconciler.deleteJob(observedJob)
		if err != nil {
			log.Error(
				err, "Failed to delete failed job", "job", observedJob)
			return err
		}
	}

	// Do not create new job immediately, leave it to the next reconciliation,
	// because we still need to be able to create the new job if we encounter
	// ephemeral error here. It is better to organize the logic in a central place.

	return nil
}

// Cancel running jobs.
func (reconciler *ClusterReconciler) cancelRunningJobs(
	takeSavepoint bool) error {
	var log = reconciler.log
	var runningJobIDs = reconciler.observed.flinkRunningJobIDs
	for _, jobID := range runningJobIDs {
		log.Info("Cancel running job", "jobID", jobID)
		var err = reconciler.cancelFlinkJob(jobID, takeSavepoint)
		if err != nil {
			log.Error(err, "Failed to cancel running job", "jobID", jobID)
			return err
		}
	}
	return nil
}

// Takes a savepoint if possible then stops the job.
func (reconciler *ClusterReconciler) cancelFlinkJob(jobID string, takeSavepoint bool) error {
	var log = reconciler.log
	if takeSavepoint && reconciler.canTakeSavepoint() {
		var err = reconciler.takeSavepoint(jobID)
		if err != nil {
			return err
		}
	} else {
		log.Info("Skip taking savepoint before stopping job", "jobID", jobID)
	}

	var apiBaseURL = getFlinkAPIBaseURL(reconciler.observed.cluster)
	reconciler.log.Info("Stoping job", "jobID", jobID)
	return reconciler.flinkClient.StopJob(apiBaseURL, jobID)
}

// Trigger savepoint if it is possible, then return the savepoint status to update.
// If triggering savepoint is skipped or triggered savepoint was created, proceed to stop the job.
func (reconciler *ClusterReconciler) cancelFlinkJobAsync(jobID string, takeSavepoint bool) (newSavepointStatus *v1beta1.SavepointStatus, err error) {
	var log = reconciler.log
	var cluster = reconciler.observed.cluster
	var observedSavepoint = reconciler.observed.cluster.Status.Savepoint
	if takeSavepoint && reconciler.canTakeSavepoint() {
		if observedSavepoint != nil && observedSavepoint.TriggerID == "" {
			newSavepointStatus, err = reconciler.takeSavepointAsync(jobID, SavepointTriggerReasonJobCancel)
			if err != nil {
				log.Info("Failed to trigger savepoint.")
				return newSavepointStatus, fmt.Errorf("failed to trigger savepoint: %v", err)
			}
			log.Info("Triggered savepoint and wait it is completed.")
			return newSavepointStatus, nil
		}
		if observedSavepoint != nil && observedSavepoint.State == SavepointStateSucceeded {
			log.Info("Successfully savepoint created. Continue to stop job.")
		} else {
			return nil, fmt.Errorf("failed to create savepoint for job cancellation: invalid savepoint status: %v", err)
		}
	} else {
		log.Info("Skip taking savepoint before stopping job", "jobID", jobID)
	}
	var apiBaseURL = getFlinkAPIBaseURL(cluster)
	log.Info("Stopping job", "jobID", jobID)
	err = reconciler.flinkClient.StopJob(apiBaseURL, jobID)
	if err != nil {
		return nil, fmt.Errorf("failed to stop job: %v", err)
	}
	return nil, nil
}

func (reconciler *ClusterReconciler) shouldTakeSavepoint() (bool, string) {
	var log = reconciler.log
	var jobSpec = reconciler.observed.cluster.Spec.Job
	var jobStatus = reconciler.observed.cluster.Status.Components.Job
	var controlStatus = reconciler.observed.cluster.Status.Control
	var savepointStatus = reconciler.observed.cluster.Status.Savepoint

	if !reconciler.canTakeSavepoint() {
		return false, ""
	}

	// User requested.

	// In the case of which savepoint status is in finished state,
	// savepoint trigger by spec.job.savepointGeneration is not possible
	// because the field cannot be increased more when savepoint is failed.
	//
	// Savepoint retry by annotation is possible because the annotations would be cleared
	// when the last savepoint was finished and user can attach the annotation again.
	if controlStatus != nil &&
		controlStatus.Name == v1beta1.ControlNameSavepoint &&
		controlStatus.State == v1beta1.ControlStateProgressing {
		log.Info(
			"Savepoint is requested",
			"control name", controlStatus.Name,
			"control state", controlStatus.State)
		return true, SavepointTriggerReasonUserRequested
	} else if jobSpec.SavepointGeneration > jobStatus.SavepointGeneration && // TODO: spec.job.savepointGeneration will be deprecated
		(savepointStatus != nil && savepointStatus.State != SavepointStateFailed && savepointStatus.State != SavepointStateTriggerFailed) {
		log.Info(
			"Savepoint is requested",
			"statusGen", jobStatus.SavepointGeneration,
			"specGen", jobSpec.SavepointGeneration)
		return true, SavepointTriggerReasonUserRequested
	}

	if jobSpec.AutoSavepointSeconds == nil {
		return false, ""
	}

	// First savepoint.
	if len(jobStatus.LastSavepointTime) == 0 {
		return true, SavepointTriggerReasonScheduled
	}

	// Interval expired.
	var tc = &TimeConverter{}
	var lastTime = tc.FromString(jobStatus.LastSavepointTime)
	var nextTime = lastTime.Add(
		time.Duration(int64(*jobSpec.AutoSavepointSeconds) * int64(time.Second)))
	return time.Now().After(nextTime), SavepointTriggerReasonScheduled
}

// Checks whether it is possible to take savepoint.
func (reconciler *ClusterReconciler) canTakeSavepoint() bool {
	var jobSpec = reconciler.observed.cluster.Spec.Job
	var savepointStatus = reconciler.observed.cluster.Status.Savepoint
	var jobStatus = reconciler.observed.cluster.Status.Components.Job
	return jobSpec != nil && jobSpec.SavepointsDir != nil &&
		!isJobStopped(jobStatus) &&
		(savepointStatus == nil ||
			(savepointStatus.State != SavepointStateProgressing) ||
			(savepointStatus.State == SavepointStateProgressing && savepointStatus.TriggerID == ""))
}

// Trigger savepoint for a job then return savepoint status to update.
func (reconciler *ClusterReconciler) takeSavepointAsync(jobID string, triggerReason string) (*v1beta1.SavepointStatus, error) {
	var log = reconciler.log
	var cluster = reconciler.observed.cluster
	var apiBaseURL = getFlinkAPIBaseURL(reconciler.observed.cluster)
	var triggerSuccess bool
	var triggerID string
	var message string
	var err error

	log.Info("Trigger savepoint.", "jobID", jobID)
	triggerID, err = reconciler.flinkClient.TakeSavepointAsync(apiBaseURL, jobID, *cluster.Spec.Job.SavepointsDir)
	if err != nil {
		// limit message size to 1KiB
		if message = err.Error(); len(message) > 1024 {
			message = message[:1024]
		}
		triggerSuccess = false
		log.Info("Savepoint trigger is failed.", "jobID", jobID, "triggerID", triggerID, "error", err)
	} else {
		triggerSuccess = true
		log.Info("Savepoint is triggered successfully.", "jobID", jobID, "triggerID", triggerID)
	}
	newSavepointStatus := getNewSavepointStatus(jobID, triggerID, triggerReason, message, triggerSuccess)
	return &newSavepointStatus, err
}

// Takes savepoint for a job then update job status with the info.
func (reconciler *ClusterReconciler) takeSavepoint(
	jobID string) error {
	var log = reconciler.log
	var apiBaseURL = getFlinkAPIBaseURL(reconciler.observed.cluster)

	log.Info("Taking savepoint.", "jobID", jobID)
	var status, err = reconciler.flinkClient.TakeSavepoint(
		apiBaseURL, jobID, *reconciler.observed.cluster.Spec.Job.SavepointsDir)
	log.Info(
		"Savepoint status.",
		"status", status,
		"error", err)
	if err == nil && len(status.FailureCause.StackTrace) > 0 {
		err = fmt.Errorf("%s", status.FailureCause.StackTrace)
	}

	if err != nil || !status.Completed {
		log.Info("Failed to take savepoint.", "jobID", jobID)
	}

	statusUpdateErr := reconciler.updateSavepointStatus(status)
	if statusUpdateErr != nil {
		log.Error(
			statusUpdateErr, "Failed to update savepoint status.", "error", statusUpdateErr)
	}

	return err
}

func (reconciler *ClusterReconciler) updateSavepointStatus(
	savepointStatus flinkclient.SavepointStatus) error {
	var cluster = v1beta1.FlinkCluster{}
	reconciler.observed.cluster.DeepCopyInto(&cluster)
	if savepointStatus.IsSuccessful() {
		var jobStatus = cluster.Status.Components.Job
		jobStatus.SavepointGeneration++
		jobStatus.LastSavepointTriggerID = savepointStatus.TriggerID
		jobStatus.SavepointLocation = savepointStatus.Location
		setTimestamp(&jobStatus.LastSavepointTime)
		setTimestamp(&cluster.Status.LastUpdateTime)
	}
	// case in which savepointing is triggered by control annotation
	var controlStatus = cluster.Status.Control
	if controlStatus != nil && controlStatus.Name == v1beta1.ControlNameSavepoint &&
		controlStatus.State == v1beta1.ControlStateProgressing {
		if controlStatus.Details == nil {
			controlStatus.Details = make(map[string]string)
		}
		var retries, err = getRetryCount(controlStatus.Details)
		if err == nil {
			if savepointStatus.IsFailed() || retries != "1" {
				controlStatus.Details[ControlRetries] = retries
			}
		} else {
			reconciler.log.Error(err, "failed to get retries from control status", "control status", controlStatus)
		}
		controlStatus.Details[ControlSavepointTriggerID] = savepointStatus.TriggerID
		controlStatus.Details[ControlJobID] = savepointStatus.JobID
		setTimestamp(&controlStatus.UpdateTime)
	}
	return reconciler.k8sClient.Status().Update(reconciler.context, &cluster)
}

// If job cancellation is failed, fill the status message with error message.
// Then, the state will be transited to the failed by the updater.
func getFailedCancelStatus(cancelErr error) *v1beta1.FlinkClusterControlStatus {
	var state string
	var message string
	var now string
	setTimestamp(&now)
	state = v1beta1.ControlStateProgressing
	// limit message size to 1KiB
	if message = cancelErr.Error(); len(message) > 1024 {
		message = message[:1024]
	}
	return &v1beta1.FlinkClusterControlStatus{
		Name:       v1beta1.ControlNameJobCancel,
		State:      state,
		UpdateTime: now,
		Message:    message,
	}
}

func (reconciler *ClusterReconciler) updateStatus(ss **v1beta1.SavepointStatus, cs **v1beta1.FlinkClusterControlStatus) {
	var log = reconciler.log

	var savepointStatus = *ss
	var controlStatus = *cs
	if savepointStatus == nil && controlStatus == nil {
		return
	}

	// Record events
	if savepointStatus != nil {
		eventType, eventReason, eventMessage := getSavepointEvent(*savepointStatus)
		reconciler.recorder.Event(reconciler.observed.cluster, eventType, eventReason, eventMessage)
	}
	if controlStatus != nil {
		eventType, eventReason, eventMessage := getControlEvent(*controlStatus)
		reconciler.recorder.Event(reconciler.observed.cluster, eventType, eventReason, eventMessage)
	}

	// Update status
	var clusterClone = reconciler.observed.cluster.DeepCopy()
	var statusUpdateErr error
	retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		var newStatus = &clusterClone.Status
		if savepointStatus != nil {
			newStatus.Savepoint = savepointStatus
		}
		if controlStatus != nil {
			newStatus.Control = controlStatus
		}
		setTimestamp(&newStatus.LastUpdateTime)
		statusUpdateErr = reconciler.k8sClient.Status().Update(reconciler.context, clusterClone)
		if statusUpdateErr == nil {
			return nil
		}
		var clusterUpdated v1beta1.FlinkCluster
		if err := reconciler.k8sClient.Get(
			reconciler.context,
			types.NamespacedName{Namespace: clusterClone.Namespace, Name: clusterClone.Name}, &clusterUpdated); err == nil {
			clusterClone = clusterUpdated.DeepCopy()
		}
		return statusUpdateErr
	})
	if statusUpdateErr != nil {
		log.Error(
			statusUpdateErr, "Failed to update status.", "error", statusUpdateErr)
	}
}
