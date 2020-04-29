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
	"time"

	"github.com/go-logr/logr"
	v1beta1 "github.com/googlecloudplatform/flink-operator/api/v1beta1"
	"github.com/googlecloudplatform/flink-operator/controllers/flinkclient"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/types"
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
}

var requeueResult = ctrl.Result{RequeueAfter: 10 * time.Second, Requeue: true}

// Compares the desired state and the observed state, if there is a difference,
// takes actions to drive the observed state towards the desired state.
func (reconciler *ClusterReconciler) reconcile() (ctrl.Result, error) {
	var err error

	// Child resources of the cluster CR will be automatically reclaimed by K8S.
	if reconciler.observed.cluster == nil {
		reconciler.log.Info("The cluster has been deleted, no action to take")
		//Flink uses Kubernetes ownerReference’s to cleanup all cluster components.
		//All the Flink created resources, including ConfigMap, Service, Deployment,
		//Pod, have been set the ownerReference to service/<ClusterId>. When the service
		//is deleted, all other resource will be deleted automatically.
		//More info : https://ci.apache.org/projects/flink/flink-docs-release-1.10/ops/deployment/native_kubernetes.html
		reconciler.reconcileNativeClusterService()
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

	err = reconciler.reconcileNativeSessionClusterJob()
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

func (reconciler *ClusterReconciler) reconcileNativeSessionClusterJob() error {
	var desiredNativeSessionClusterJob = reconciler.desired.NativeClusterSessionJob
	var observedNativeSessionClusterJob = reconciler.observed.nativeClusterSessionJob

	if desiredNativeSessionClusterJob != nil && observedNativeSessionClusterJob == nil {
		return reconciler.createJob(desiredNativeSessionClusterJob)
	}

	if desiredNativeSessionClusterJob != nil && observedNativeSessionClusterJob != nil {
		reconciler.log.Info("NativeSessionClusterJob already exists, no action")
		return nil
		// TODO: compare and update if needed.
	}

	if desiredNativeSessionClusterJob == nil && observedNativeSessionClusterJob != nil {
		reconciler.reconcileNativeClusterService()
		return reconciler.deleteJob(observedNativeSessionClusterJob)
	}

	return nil
}

func (reconciler *ClusterReconciler) reconcileNativeClusterService() {
	// Delete the service which created by the flink.
	var nativeFlinkSessionService = new(corev1.Service)
	var k8sClient = reconciler.k8sClient
	var context = reconciler.context
	var log = reconciler.log.WithValues("component", "nativeSessionJob")

	if reconciler.observed.nativeClusterSessionJob == nil {
		log.Info("The job for the native session cluster has been deleted.")
		return
	}
	var jobObjectMeta = reconciler.observed.nativeClusterSessionJob.ObjectMeta
	log.Info("Deleting nativeFlinkSessionService")
	err := k8sClient.Get(
		context,
		types.NamespacedName{
			Namespace: jobObjectMeta.Namespace,
			Name:      getNativeFlinkClusterName(jobObjectMeta.Name),
		},
		nativeFlinkSessionService)
	if err != nil {
		log.Error(err, "Failed to get nativeFlinkSessionService.")
	} else {
		log.Info("Get nativeFlinkSessionService", "state", *nativeFlinkSessionService)
		err = reconciler.deleteService(nativeFlinkSessionService, "nativeSessionJob")
		if err != nil {
			log.Info("Failed to delete nativeFlinkSessionService", "error", err)
		} else {
			log.Info("Delete nativeFlinkSessionService successfully.")
		}
	}
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

		if len(jobID) > 0 && reconciler.shouldTakeSavepoint(jobID) {
			reconciler.takeSavepoint(jobID)
		}

		if !reconciler.isJobStopped() {
			log.Info("Job is not finished yet, no action", "jobID", jobID)
			return requeueResult, nil
		}

		log.Info("Job has finished, no action")
		return ctrl.Result{}, nil
	}

	// Delete
	if desiredJob == nil && observedJob != nil {
		if len(jobID) > 0 {
			log.Info("Cancelling job", "jobID", jobID)
			var err = reconciler.cancelFlinkJob(jobID, true /* takeSavepoint */)
			if err != nil {
				log.Error(err, "Failed to cancel job", "jobID", jobID)
				statusUpdateErr := reconciler.updateCancelStatus(err)
				if statusUpdateErr != nil {
					log.Error(
						statusUpdateErr, "Failed to update control status.", "error", statusUpdateErr)
				}
				return requeueResult, nil
			}
		}
		var err = reconciler.deleteJob(observedJob)
		statusUpdateErr := reconciler.updateCancelStatus(err)
		if statusUpdateErr != nil {
			log.Error(
				statusUpdateErr, "Failed to update control status.", "error", statusUpdateErr)
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

func (reconciler *ClusterReconciler) isJobStopped() bool {
	var jobStatus = reconciler.observed.cluster.Status.Components.Job
	return jobStatus != nil &&
		(jobStatus.State == v1beta1.JobStateSucceeded ||
			jobStatus.State == v1beta1.JobStateFailed ||
			jobStatus.State == v1beta1.JobStateCancelled)
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

func (reconciler *ClusterReconciler) shouldTakeSavepoint(
	jobID string) bool {
	var log = reconciler.log
	var jobSpec = reconciler.observed.cluster.Spec.Job
	var jobStatus = reconciler.observed.cluster.Status.Components.Job
	var controlStatus = reconciler.observed.cluster.Status.Control

	if !reconciler.canTakeSavepoint() {
		return false
	}

	// User requested.
	if jobSpec.SavepointGeneration > jobStatus.SavepointGeneration {
		log.Info(
			"Savepoint is requested",
			"statusGen", jobStatus.SavepointGeneration,
			"specGen", jobSpec.SavepointGeneration)
		return true
		// Triggered by control annotation
	} else if controlStatus != nil &&
		controlStatus.Name == v1beta1.ControlNameSavepoint &&
		controlStatus.State == v1beta1.ControlStateProgressing {
		log.Info(
			"Savepoint is requested",
			"control name", controlStatus.Name,
			"control state", controlStatus.State)
		return true
	}

	if jobSpec.AutoSavepointSeconds == nil {
		return false
	}

	// First savepoint.
	if len(jobStatus.LastSavepointTime) == 0 {
		return true
	}

	// Interval expired.
	var tc = &TimeConverter{}
	var lastTime = tc.FromString(jobStatus.LastSavepointTime)
	var nextTime = lastTime.Add(
		time.Duration(int64(*jobSpec.AutoSavepointSeconds) * int64(time.Second)))
	return time.Now().After(nextTime)
}

// Checks whether it is possible to take savepoint.
func (reconciler *ClusterReconciler) canTakeSavepoint() bool {
	var jobSpec = reconciler.observed.cluster.Spec.Job
	return jobSpec != nil && jobSpec.SavepointsDir != nil &&
		!reconciler.isJobStopped()
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
	var isSavepointSucceeded = savepointStatus.Completed && savepointStatus.FailureCause.StackTrace == ""
	var cluster = v1beta1.FlinkCluster{}
	reconciler.observed.cluster.DeepCopyInto(&cluster)
	if isSavepointSucceeded {
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
			if !isSavepointSucceeded || retries != "1" {
				controlStatus.Details["retries"] = retries
			}
		} else {
			reconciler.log.Error(err, "failed to get retries from control status", "control status", controlStatus)
		}
		controlStatus.Details["savepointTriggerID"] = savepointStatus.TriggerID
		controlStatus.Details["jobID"] = savepointStatus.JobID
		setTimestamp(&controlStatus.UpdateTime)
	}
	return reconciler.k8sClient.Status().Update(reconciler.context, &cluster)
}

func (reconciler *ClusterReconciler) updateCancelStatus(cancelErr error) error {
	var observedControlStatus = reconciler.observed.cluster.Status.Control
	if observedControlStatus == nil || observedControlStatus.Name != v1beta1.ControlNameCancel ||
		observedControlStatus.State != v1beta1.ControlStateProgressing {
		return nil
	}

	var cluster = v1beta1.FlinkCluster{}
	reconciler.observed.cluster.DeepCopyInto(&cluster)

	var controlStatus = cluster.Status.Control
	if controlStatus.Details == nil {
		controlStatus.Details = make(map[string]string)
	}
	var retries, err = getRetryCount(controlStatus.Details)
	if err == nil {
		if cancelErr != nil || retries != "1" {
			controlStatus.Details["retries"] = retries
		}
	} else {
		reconciler.log.Error(err, "failed to get retries from control status", "control status", controlStatus)
	}
	controlStatus.Details["jobID"] = cluster.Status.Components.Job.ID
	setTimestamp(&controlStatus.UpdateTime)

	return reconciler.k8sClient.Status().Update(reconciler.context, &cluster)
}
