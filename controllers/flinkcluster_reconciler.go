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
	"errors"
	"fmt"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	"time"

	"github.com/go-logr/logr"
	v1beta1 "github.com/googlecloudplatform/flink-operator/api/v1beta1"
	"github.com/googlecloudplatform/flink-operator/controllers/batchscheduler"
	"github.com/googlecloudplatform/flink-operator/controllers/flinkclient"
	"github.com/googlecloudplatform/flink-operator/controllers/model"

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
	desired     model.DesiredClusterState
	recorder    record.EventRecorder
}

const JobCheckInterval = 10 * time.Second

var requeueResult = ctrl.Result{RequeueAfter: JobCheckInterval, Requeue: true}

// Compares the desired state and the observed state, if there is a difference,
// takes actions to drive the observed state towards the desired state.
func (reconciler *ClusterReconciler) reconcile() (ctrl.Result, error) {
	var err error

	// Child resources of the cluster CR will be automatically reclaimed by K8S.
	if reconciler.observed.cluster == nil {
		reconciler.log.Info("The cluster has been deleted, no action to take")
		return ctrl.Result{}, nil
	}

	if shouldUpdateCluster(&reconciler.observed) {
		reconciler.log.Info("The cluster update is in progress")
	}
	// If batch-scheduling enabled
	if reconciler.observed.cluster.Spec.BatchSchedulerName != nil &&
		*reconciler.observed.cluster.Spec.BatchSchedulerName != "" {
		scheduler, err := batchscheduler.GetScheduler(*reconciler.observed.cluster.Spec.BatchSchedulerName)
		if err != nil {
			return ctrl.Result{}, err
		}

		err = scheduler.Schedule(reconciler.observed.cluster, &reconciler.desired)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	err = reconciler.reconcileConfigMap()
	if err != nil {
		return ctrl.Result{}, err
	}

	err = reconciler.reconcileJobManagerStatefulSet()
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

	err = reconciler.reconcileTaskManagerStatefulSet()
	if err != nil {
		return ctrl.Result{}, err
	}

	result, err := reconciler.reconcileJob()

	return result, nil
}

func (reconciler *ClusterReconciler) reconcileJobManagerStatefulSet() error {
	return reconciler.reconcileStatefulSet(
		"JobManager",
		reconciler.desired.JmStatefulSet,
		reconciler.observed.jmStatefulSet)
}

func (reconciler *ClusterReconciler) reconcileTaskManagerStatefulSet() error {
	return reconciler.reconcileStatefulSet(
		"TaskManager",
		reconciler.desired.TmStatefulSet,
		reconciler.observed.tmStatefulSet)
}

func (reconciler *ClusterReconciler) reconcileStatefulSet(
	component string,
	desiredStatefulSet *appsv1.StatefulSet,
	observedStatefulSet *appsv1.StatefulSet) error {
	var log = reconciler.log.WithValues("component", component)

	if desiredStatefulSet != nil && observedStatefulSet == nil {
		return reconciler.createStatefulSet(desiredStatefulSet, component)
	}

	if desiredStatefulSet != nil && observedStatefulSet != nil {
		var cluster = reconciler.observed.cluster
		if shouldUpdateCluster(&reconciler.observed) && !isComponentUpdated(observedStatefulSet, cluster) {
			updateComponent := fmt.Sprintf("%v StatefulSet", component)
			var err error
			if *reconciler.observed.cluster.Spec.RecreateOnUpdate {
				err = reconciler.deleteOldComponent(desiredStatefulSet, observedStatefulSet, updateComponent)
			} else {
				err = reconciler.updateComponent(desiredStatefulSet, updateComponent)
			}
			if err != nil {
				return err
			}
			return nil
		}
		log.Info("StatefulSet already exists, no action")
		return nil
	}

	if desiredStatefulSet == nil && observedStatefulSet != nil {
		return reconciler.deleteStatefulSet(observedStatefulSet, component)
	}

	return nil
}

func (reconciler *ClusterReconciler) createStatefulSet(
	statefulSet *appsv1.StatefulSet, component string) error {
	var context = reconciler.context
	var log = reconciler.log.WithValues("component", component)
	var k8sClient = reconciler.k8sClient

	log.Info("Creating StatefulSet", "StatefulSet", *statefulSet)
	var err = k8sClient.Create(context, statefulSet)
	if err != nil {
		log.Error(err, "Failed to create StatefulSet")
	} else {
		log.Info("StatefulSet created")
	}
	return err
}

func (reconciler *ClusterReconciler) deleteOldComponent(desired runtime.Object, observed runtime.Object, component string) error {
	var log = reconciler.log.WithValues("component", component)
	if isComponentUpdated(observed, reconciler.observed.cluster) {
		reconciler.log.Info(fmt.Sprintf("%v is already updated, no action", component))
		return nil
	}

	var context = reconciler.context
	var k8sClient = reconciler.k8sClient
	log.Info("Deleting component for update", "component", desired)
	err := k8sClient.Delete(context, desired)
	if err != nil {
		log.Error(err, "Failed to delete component for update")
		return err
	}
	log.Info("Component deleted for update successfully")
	return nil
}

func (reconciler *ClusterReconciler) updateComponent(desired runtime.Object, component string) error {
	var log = reconciler.log.WithValues("component", component)
	var context = reconciler.context
	var k8sClient = reconciler.k8sClient

	log.Info("Update component", "component", desired)
	err := k8sClient.Update(context, desired)
	if err != nil {
		log.Error(err, "Failed to update component for update")
		return err
	}
	log.Info("Component update successfully")
	return nil
}

func (reconciler *ClusterReconciler) updateStatefulSet(
	statefulSet *appsv1.StatefulSet, component string) error {
	var context = reconciler.context
	var log = reconciler.log.WithValues("component", component)
	var k8sClient = reconciler.k8sClient

	log.Info("Updating StatefulSet", "StatefulSet", statefulSet)
	var err = k8sClient.Update(context, statefulSet)
	if err != nil {
		log.Error(err, "Failed to update StatefulSet")
	} else {
		log.Info("StatefulSet updated")
	}
	return err
}

func (reconciler *ClusterReconciler) deleteStatefulSet(
	statefulSet *appsv1.StatefulSet, component string) error {
	var context = reconciler.context
	var log = reconciler.log.WithValues("component", component)
	var k8sClient = reconciler.k8sClient

	log.Info("Deleting StatefulSet", "StatefulSet", statefulSet)
	var err = k8sClient.Delete(context, statefulSet)
	err = client.IgnoreNotFound(err)
	if err != nil {
		log.Error(err, "Failed to delete StatefulSet")
	} else {
		log.Info("StatefulSet deleted")
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
		var cluster = reconciler.observed.cluster
		if shouldUpdateCluster(&reconciler.observed) && !isComponentUpdated(observedJmService, cluster) {
			// v1.Service API does not handle update correctly when below values are empty.
			desiredJmService.SetResourceVersion(observedJmService.GetResourceVersion())
			desiredJmService.Spec.ClusterIP = observedJmService.Spec.ClusterIP
			var err error
			if *reconciler.observed.cluster.Spec.RecreateOnUpdate {
				err = reconciler.deleteOldComponent(desiredJmService, observedJmService, "JobManager service")
			} else {
				err = reconciler.updateComponent(desiredJmService, "JobManager service")
			}
			if err != nil {
				return err
			}
			return nil
		}
		reconciler.log.Info("JobManager service already exists, no action")
		return nil
	}

	if desiredJmService == nil && observedJmService != nil {
		return reconciler.deleteService(observedJmService, "JobManager")
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
		var cluster = reconciler.observed.cluster
		if shouldUpdateCluster(&reconciler.observed) && !isComponentUpdated(observedJmIngress, cluster) {
			var err error
			if *reconciler.observed.cluster.Spec.RecreateOnUpdate {
				err = reconciler.deleteOldComponent(desiredJmIngress, observedJmIngress, "JobManager ingress")
			} else {
				err = reconciler.updateComponent(desiredJmIngress, "JobManager ingress")
			}
			if err != nil {
				return err
			}
			return nil
		}
		reconciler.log.Info("JobManager ingress already exists, no action")
		return nil
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
		var cluster = reconciler.observed.cluster
		if shouldUpdateCluster(&reconciler.observed) && !isComponentUpdated(observedConfigMap, cluster) {
			var err error
			if *reconciler.observed.cluster.Spec.RecreateOnUpdate {
				err = reconciler.deleteOldComponent(desiredConfigMap, observedConfigMap, "ConfigMap")
			} else {
				err = reconciler.updateComponent(desiredConfigMap, "ConfigMap")
			}
			if err != nil {
				return err
			}
			return nil
		}
		reconciler.log.Info("ConfigMap already exists, no action")
		return nil
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
	var recorded = observed.cluster.Status
	var jobSpec = observed.cluster.Spec.Job
	var job = recorded.Components.Job
	var err error
	var jobID = reconciler.getFlinkJobID()

	// Update status changed via job reconciliation.
	var newSavepointStatus *v1beta1.SavepointStatus
	var newControlStatus *v1beta1.FlinkClusterControlStatus
	defer reconciler.updateStatus(&newSavepointStatus, &newControlStatus)

	// Create new Flink job submitter when starting new job, updating job or restarting job in failure.
	if desiredJob != nil && !job.IsActive() {
		log.Info("Deploying Flink job")

		// TODO: Record event or introduce Condition in CRD status to notify update state pended.
		// https://github.com/kubernetes/apimachinery/blob/57f2a0733447cfd41294477d833cce6580faaca3/pkg/apis/meta/v1/types.go#L1376
		var unexpectedJobs = observed.flinkJob.unexpected
		if len(unexpectedJobs) > 0 {
			// This is an exceptional situation.
			// There should be no jobs because all jobs are terminated in the previous iterations.
			// In this case user should identify the problem so that the job is not executed multiple times unintentionally
			// cause of Flink error, Flink operator error or other unknown error.
			// If user want to proceed, unexpected jobs should be terminated.
			log.Error(errors.New("unexpected jobs found"), "Failed to create job submitter", "unexpected jobs", unexpectedJobs)
			return ctrl.Result{}, nil
		}

		// Create Flink job submitter
		log.Info("Updating job status to proceed creating new job submitter")
		// Job status must be updated before creating a job submitter to ensure the observed job is the job submitted by the operator.
		err = reconciler.updateJobDeployStatus()
		if err != nil {
			log.Info("Failed to update the job status for job submission")
			return requeueResult, err
		}
		var observedSubmitter = observed.flinkJobSubmitter.job
		if observedSubmitter != nil {
			log.Info("Found old job submitter")
			err = reconciler.deleteJob(observedSubmitter)
			if err != nil {
				return requeueResult, err
			}
		}
		err = reconciler.createJob(desiredJob)

		return requeueResult, err
	}

	if desiredJob != nil && job.IsActive() {
		if job.State == v1beta1.JobStateDeploying {
			log.Info("Job submitter is deployed, wait until completed")
			return requeueResult, nil
		}

		// Suspend or stop job to proceed update.
		if recorded.Revision.IsUpdateTriggered() {
			log.Info("Preparing job update")
			var takeSavepoint = jobSpec.TakeSavepointOnUpdate == nil || *jobSpec.TakeSavepointOnUpdate
			var shouldSuspend = takeSavepoint && isBlank(jobSpec.FromSavepoint)
			if shouldSuspend {
				newSavepointStatus, err = reconciler.trySuspendJob()
			} else if shouldUpdateJob(&observed) {
				err = reconciler.cancelJob()
			}
			return requeueResult, err
		}

		// Trigger savepoint if required.
		if len(jobID) > 0 {
			var savepointTriggerReason = reconciler.shouldTakeSavepoint()
			if savepointTriggerReason != "" {
				newSavepointStatus, err = reconciler.triggerSavepoint(jobID, savepointTriggerReason, false)
			}
			// Get new control status when the savepoint reason matches the requested control.
			var userControl = getNewControlRequest(observed.cluster)
			if userControl == v1beta1.ControlNameSavepoint && savepointTriggerReason == v1beta1.SavepointTriggerReasonUserRequested {
				newControlStatus = getControlStatus(userControl, v1beta1.ControlStateInProgress)
			}
			return requeueResult, err
		}

		log.Info("Job is not finished yet, no action", "jobID", jobID)
		return requeueResult, nil
	}

	// Job cancel requested. Stop Flink job.
	if desiredJob == nil && job.IsActive() {
		log.Info("Stopping job", "jobID", jobID)
		newSavepointStatus, err = reconciler.trySuspendJob()
		var userControl = getNewControlRequest(observed.cluster)
		if userControl == v1beta1.ControlNameJobCancel {
			newControlStatus = getControlStatus(userControl, v1beta1.ControlStateInProgress)
		}
		return requeueResult, err
	}

	if job.IsStopped() {
		log.Info("Job has finished, no action")
	}

	return ctrl.Result{}, nil
}

func (reconciler *ClusterReconciler) createJob(job *batchv1.Job) error {
	var context = reconciler.context
	var log = reconciler.log
	var k8sClient = reconciler.k8sClient

	log.Info("Creating job submitter", "resource", *job)
	var err = k8sClient.Create(context, job)
	if err != nil {
		log.Info("Failed to created job submitter", "error", err)
	} else {
		log.Info("Job submitter created")
	}
	return err
}

func (reconciler *ClusterReconciler) deleteJob(job *batchv1.Job) error {
	var context = reconciler.context
	var log = reconciler.log
	var k8sClient = reconciler.k8sClient

	var deletePolicy = metav1.DeletePropagationBackground
	var deleteOption = client.DeleteOptions{PropagationPolicy: &deletePolicy}

	log.Info("Deleting job submitter", "job", job)
	var err = k8sClient.Delete(context, job, &deleteOption)
	err = client.IgnoreNotFound(err)
	if err != nil {
		log.Error(err, "Failed to delete job submitter")
	} else {
		log.Info("Job submitter deleted")
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

func (reconciler *ClusterReconciler) trySuspendJob() (*v1beta1.SavepointStatus, error) {
	var log = reconciler.log
	var recorded = reconciler.observed.cluster.Status
	var jobID = reconciler.getFlinkJobID()

	log.Info("Checking the conditions for progressing")
	var canSuspend = reconciler.canSuspendJob(jobID, recorded.Savepoint)
	if canSuspend {
		log.Info("Triggering savepoint for suspending job")
		var newSavepointStatus, err = reconciler.triggerSavepoint(jobID, v1beta1.SavepointTriggerReasonUpdate, true)
		if err != nil {
			log.Info("Failed to trigger savepoint", "jobID", jobID, "triggerID", newSavepointStatus.TriggerID, "error", err)
		} else {
			log.Info("Successfully savepoint triggered", "jobID", jobID, "triggerID", newSavepointStatus.TriggerID)
		}
		return newSavepointStatus, err
	}

	return nil, nil
}

func (reconciler *ClusterReconciler) cancelJob() error {
	var log = reconciler.log
	var observedFlinkJob = reconciler.observed.flinkJob.status

	log.Info("Stopping Flink job", "", observedFlinkJob)
	var err = reconciler.cancelRunningJobs(false /* takeSavepoint */)
	if err != nil {
		log.Info("Failed to stop Flink job")
		return err
	}

	// TODO: Not to delete the job submitter immediately, and retain the latest ones for inspection.
	var observedSubmitter = reconciler.observed.flinkJobSubmitter.job
	if observedSubmitter != nil {
		var err = reconciler.deleteJob(observedSubmitter)
		if err != nil {
			log.Error(
				err, "Failed to delete job submitter", "job", observedSubmitter)
			return err
		}
	}

	return nil
}

func (reconciler *ClusterReconciler) cancelUnexpectedJobs(
	takeSavepoint bool) error {
	var unexpectedJobs = reconciler.observed.flinkJob.unexpected
	return reconciler.cancelJobs(takeSavepoint, unexpectedJobs)
}

// Cancel running jobs.
func (reconciler *ClusterReconciler) cancelRunningJobs(
	takeSavepoint bool) error {
	var log = reconciler.log
	var runningJobs = reconciler.observed.flinkJob.unexpected
	var flinkJob = reconciler.observed.flinkJob.status
	if flinkJob != nil && flinkJob.ID != "" &&
		getFlinkJobDeploymentState(flinkJob.Status) == v1beta1.JobStateRunning {
		runningJobs = append(runningJobs, flinkJob.ID)
	}
	if len(runningJobs) == 0 {
		log.Info("No running Flink jobs to stop.")
		return nil
	}
	return reconciler.cancelJobs(takeSavepoint, runningJobs)
}

// Cancel jobs.
func (reconciler *ClusterReconciler) cancelJobs(
	takeSavepoint bool,
	jobs []string) error {
	var log = reconciler.log
	for _, jobID := range jobs {
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
	if takeSavepoint && canTakeSavepoint(*reconciler.observed.cluster) {
		log.Info("Taking savepoint before stopping job", "jobID", jobID)
		var err = reconciler.takeSavepoint(jobID)
		if err != nil {
			return err
		}
	}

	var apiBaseURL = getFlinkAPIBaseURL(reconciler.observed.cluster)
	reconciler.log.Info("Stoping job", "jobID", jobID)
	return reconciler.flinkClient.StopJob(apiBaseURL, jobID)
}

// canSuspendJob
func (reconciler *ClusterReconciler) canSuspendJob(jobID string, s *v1beta1.SavepointStatus) bool {
	var log = reconciler.log
	var firstTry = !finalSavepointRequested(jobID, s)
	if firstTry {
		return true
	}

	switch s.State {
	case v1beta1.SavepointStateSucceeded:
		log.Info("Successfully savepoint completed, wait until the job stops")
		return false
	case v1beta1.SavepointStateInProgress:
		log.Info("Savepoint is in progress, wait until it is completed")
		return false
	case v1beta1.SavepointStateTriggerFailed:
		log.Info("Savepoint trigger failed in previous request")
	case v1beta1.SavepointStateFailed:
		log.Info("Savepoint failed on previous request")
	}

	var retryTimeArrived = hasTimeElapsed(s.UpdateTime, time.Now(), SavepointRetryIntervalSeconds)
	if !retryTimeArrived {
		log.Info("Wait until next retry time arrived")
	}
	return retryTimeArrived
}

func (reconciler *ClusterReconciler) shouldTakeSavepoint() string {
	var observed = reconciler.observed
	var cluster = observed.cluster
	var jobSpec = observed.cluster.Spec.Job
	var job = observed.cluster.Status.Components.Job
	var savepoint = observed.cluster.Status.Savepoint
	var newRequestedControl = getNewControlRequest(cluster)

	if !canTakeSavepoint(*reconciler.observed.cluster) {
		return ""
	}

	// Savepoint trigger priority is user request including update and job stop.
	switch {
	// TODO: spec.job.cancelRequested will be deprecated
	// Should stop job with savepoint by user requested control
	case newRequestedControl == v1beta1.ControlNameJobCancel || (jobSpec.CancelRequested != nil && *jobSpec.CancelRequested):
		return v1beta1.SavepointTriggerReasonJobCancel
	// Take savepoint by user request
	case newRequestedControl == v1beta1.ControlNameSavepoint:
		fallthrough
	// TODO: spec.job.savepointGeneration will be deprecated
	case jobSpec.SavepointGeneration > job.SavepointGeneration:
		// Triggered by savepointGeneration increased.
		// When previous savepoint is failed, savepoint trigger by spec.job.savepointGeneration is not possible
		// because the field cannot be increased more.
		// Note: checkSavepointGeneration in flinkcluster_validate.go
		return v1beta1.SavepointTriggerReasonUserRequested
	// Scheduled auto savepoint
	case jobSpec.AutoSavepointSeconds != nil:
		// When previous try was failed, check retry interval.
		if savepoint.IsFailed() && savepoint.TriggerReason == v1beta1.SavepointTriggerReasonScheduled {
			var nextRetryTime = getTime(savepoint.UpdateTime).Add(SavepointRetryIntervalSeconds * time.Second)
			if time.Now().After(nextRetryTime) {
				return v1beta1.SavepointTriggerReasonScheduled
			} else {
				return ""
			}
		}
		// Check if next trigger time arrived.
		var compareTime string
		if len(job.SavepointTime) == 0 {
			compareTime = job.StartTime
		} else {
			compareTime = job.SavepointTime
		}
		var nextTime = getTimeAfterAddedSeconds(compareTime, int64(*jobSpec.AutoSavepointSeconds))
		if time.Now().After(nextTime) {
			return v1beta1.SavepointTriggerReasonScheduled
		}
	}
	return ""
}

// Trigger savepoint for a job then return savepoint status to update.
func (reconciler *ClusterReconciler) triggerSavepoint(jobID string, triggerReason string, cancel bool) (*v1beta1.SavepointStatus, error) {
	var log = reconciler.log
	var cluster = reconciler.observed.cluster
	var apiBaseURL = getFlinkAPIBaseURL(reconciler.observed.cluster)
	var triggerSuccess bool
	var triggerID flinkclient.SavepointTriggerID
	var message string
	var err error

	log.Info(fmt.Sprintf("Trigger savepoint for %s", triggerReason), "jobID", jobID)
	triggerID, err = reconciler.flinkClient.TriggerSavepoint(apiBaseURL, jobID, *cluster.Spec.Job.SavepointsDir, cancel)
	if err != nil {
		// limit message size to 1KiB
		if message = err.Error(); len(message) > 1024 {
			message = message[:1024] + "..."
		}
		triggerSuccess = false
		log.Info("Failed to trigger savepoint", "jobID", jobID, "triggerID", triggerID, "error", err)
	} else {
		triggerSuccess = true
		log.Info("Successfully savepoint triggered", "jobID", jobID, "triggerID", triggerID)
	}
	newSavepointStatus := reconciler.getNewSavepointStatus(triggerID.RequestID, triggerReason, message, triggerSuccess)

	return newSavepointStatus, err
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

	return err
}

func (reconciler *ClusterReconciler) updateStatus(
	ss **v1beta1.SavepointStatus, cs **v1beta1.FlinkClusterControlStatus) {
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

func (reconciler *ClusterReconciler) updateJobDeployStatus() error {
	var log = reconciler.log
	var observedCluster = reconciler.observed.cluster
	var desiredJobSubmitter = reconciler.desired.Job
	var err error

	var clusterClone = observedCluster.DeepCopy()
	var newJob = clusterClone.Status.Components.Job

	// Reset running job information.
	newJob.ID = ""
	newJob.StartTime = ""
	newJob.EndTime = ""

	// Mark as job submitter is deployed.
	setTimestamp(&newJob.DeployTime)
	setTimestamp(&clusterClone.Status.LastUpdateTime)

	// Latest savepoint location should be fromSavepoint.
	var fromSavepoint = getFromSavepoint(desiredJobSubmitter.Spec)
	newJob.FromSavepoint = fromSavepoint
	if newJob.SavepointLocation != "" {
		newJob.SavepointLocation = fromSavepoint
	}

	// Update job status.
	err = reconciler.k8sClient.Status().Update(reconciler.context, clusterClone)
	if err != nil {
		log.Error(
			err, "Failed to update job status for new job submitter", "error", err)
	} else {
		log.Info("Succeeded to update job status for new job submitter.", "job status", newJob)
	}
	return err
}

// getNewSavepointStatus returns newly triggered savepoint status.
func (reconciler *ClusterReconciler) getNewSavepointStatus(triggerID string, triggerReason string, message string, triggerSuccess bool) *v1beta1.SavepointStatus {
	var jobID = reconciler.getFlinkJobID()
	var savepointState string
	var now string
	setTimestamp(&now)

	if triggerSuccess {
		savepointState = v1beta1.SavepointStateInProgress
	} else {
		savepointState = v1beta1.SavepointStateTriggerFailed
	}
	var savepointStatus = &v1beta1.SavepointStatus{
		JobID:         jobID,
		TriggerID:     triggerID,
		TriggerReason: triggerReason,
		TriggerTime:   now,
		UpdateTime:    now,
		Message:       message,
		State:         savepointState,
	}
	return savepointStatus
}

// Convert raw time to object and add `addedSeconds` to it,
// getting a time object for the parsed `rawTime` with `addedSeconds` added to it.
func getTimeAfterAddedSeconds(rawTime string, addedSeconds int64) time.Time {
	var tc = &TimeConverter{}
	var lastTriggerTime = time.Time{}
	if len(rawTime) != 0 {
		lastTriggerTime = tc.FromString(rawTime)
	}
	return lastTriggerTime.Add(time.Duration(addedSeconds * int64(time.Second)))
}
