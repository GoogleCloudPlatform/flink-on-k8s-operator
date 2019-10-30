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
	v1alpha1 "github.com/googlecloudplatform/flink-operator/api/v1alpha1"
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
}

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

	// Create
	if desiredJob != nil && observedJob == nil {
		// If the observed Flink job status list is not nil (e.g., emtpy list), it
		// means Flink REST API server is up and running. It is the source of
		// truth of whether we can submit a job.
		if reconciler.observed.flinkJobList != nil {
			var err = reconciler.createJob(desiredJob)
			return ctrl.Result{RequeueAfter: 10 * time.Second, Requeue: true}, err
		}
		log.Info("Waiting for Flink API to be ready before creating job")
		return ctrl.Result{RequeueAfter: 10 * time.Second, Requeue: true}, nil
	}

	// Update
	var jobID = reconciler.getFlinkJobID()
	if desiredJob != nil && observedJob != nil {
		if len(jobID) > 0 && reconciler.shouldAutoTakeSavepoint(jobID) {
			reconciler.takeSavepoint(jobID)
		}

		if !reconciler.isJobFinished() {
			log.Info("Job is not finished yet, no action", "jobID", jobID)
			return ctrl.Result{RequeueAfter: 10 * time.Second, Requeue: true}, nil
		}

		log.Info("Job has finished, no action")
		return ctrl.Result{}, nil
	}

	// Delete
	if desiredJob == nil && observedJob != nil {
		if len(jobID) > 0 {
			log.Info("Cancelling job", "jobID", jobID)
			var err = reconciler.cancelJob(jobID)
			if err != nil {
				log.Error(err, "Failed to cancel job", "jobID", jobID)
				return ctrl.Result{RequeueAfter: 10 * time.Second, Requeue: true}, nil
			}
		}
		var err = reconciler.deleteJob(observedJob)
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

func (reconciler *ClusterReconciler) isJobFinished() bool {
	var jobStatus = reconciler.observed.cluster.Status.Components.Job
	return jobStatus != nil &&
		(jobStatus.State == v1alpha1.JobState.Succeeded ||
			jobStatus.State == v1alpha1.JobState.Failed ||
			jobStatus.State == v1alpha1.JobState.Cancelled)
}

// Takes a savepoint if possible then stops the job.
func (reconciler *ClusterReconciler) cancelJob(jobID string) error {
	var log = reconciler.log
	if reconciler.canTakeSavepoint() {
		var err = reconciler.takeSavepoint(jobID)
		if err != nil {
			return err
		}
	} else {
		log.Info("Can not take savepoint before stopping job", "jobID", jobID)
	}
	if !reconciler.isJobFinished() {
		return reconciler.stopJob(jobID)
	}
	log.Info("Job has finished, no need to cancel", "jobID", jobID)
	return nil
}

// Stops a job through Flink REST API.
func (reconciler *ClusterReconciler) stopJob(jobID string) error {
	var apiBaseURL = getFlinkAPIBaseURL(reconciler.observed.cluster)
	reconciler.log.Info("Stoping job", "jobID", jobID)
	return reconciler.flinkClient.StopJob(apiBaseURL, jobID)
}

func (reconciler *ClusterReconciler) shouldAutoTakeSavepoint(
	jobID string) bool {
	var jobSpec = reconciler.observed.cluster.Spec.Job
	var jobStatus = reconciler.observed.cluster.Status.Components.Job

	if !reconciler.canTakeSavepoint() {
		return false
	}

	// Not enabled.
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
		!reconciler.isJobFinished()
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

	if status.Completed && err == nil {
		err = reconciler.updateSavepointStatus(status)
		if err != nil {
			log.Error(
				err, "Failed to update savepoint status.", "error", err)
		}
	} else {
		log.Info("Failed to take savepoint.", "jobID", jobID)
	}
	return err
}

func (reconciler *ClusterReconciler) updateSavepointStatus(
	savepointStatus flinkclient.SavepointStatus) error {
	var cluster = v1alpha1.FlinkCluster{}
	reconciler.observed.cluster.DeepCopyInto(&cluster)
	cluster.Status = reconciler.observed.cluster.Status
	var jobStatus = cluster.Status.Components.Job
	jobStatus.LastSavepointTriggerID = savepointStatus.TriggerID
	jobStatus.SavepointLocation = savepointStatus.Location
	var tc = &TimeConverter{}
	jobStatus.LastSavepointTime = tc.ToString(time.Now())
	return reconciler.k8sClient.Update(reconciler.context, &cluster)
}
