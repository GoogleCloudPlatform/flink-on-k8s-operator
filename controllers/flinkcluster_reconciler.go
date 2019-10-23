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
		log.Info("Skip creating job, waiting for Flink API to be ready")
		return ctrl.Result{RequeueAfter: 10 * time.Second, Requeue: true}, nil
	}

	// Update
	if desiredJob != nil && observedJob != nil {
		var jobID = reconciler.getFlinkJobID()
		if reconciler.shouldTakeSavepoint() {
			var triggerID string
			var err error
			for i := 0; i < 3; i++ {
				log.Info("Taking savepoint.", "jobID", jobID, "i", i)
				triggerID, _, err = reconciler.takeSavepoint(jobID)
				if err == nil {
					break
				}
				log.Info("Failed to take savepoint.", "jobID", jobID, "i", i)
			}

			if err == nil {
				err = reconciler.updateSavepointStatus(triggerID, err)
				if err != nil {
					log.Error(
						err, "Failed to update savepoint status.", "error", err)
				}
			}
		} else {
			log.Info("Skip taking savepoint.", "jobID", jobID)
		}

		if !reconciler.isJobFinished() {
			log.Info("Flink job is not finished.")
			return ctrl.Result{RequeueAfter: 10 * time.Second, Requeue: true}, nil
		}

		log.Info("Job has finished, no action")
		return ctrl.Result{}, nil
	}

	// Delete
	// TODO: take savepoint before deleting.
	if desiredJob == nil && observedJob != nil {
		reconciler.deleteJob(observedJob)
		return ctrl.Result{}, nil
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

func (reconciler *ClusterReconciler) shouldTakeSavepoint() bool {
	var jobSpec = reconciler.observed.cluster.Spec.Job
	var jobStatus = reconciler.observed.cluster.Status.Components.Job

	// Not enabled.
	if jobSpec.AutoSavepointSeconds == nil || jobSpec.SavepointsDir == nil {
		return false
	}

	// Job is not running yet.
	var flinkJobID = reconciler.getFlinkJobID()
	if len(flinkJobID) == 0 {
		return false
	}

	// Job has finished.
	if reconciler.isJobFinished() {
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

// Takes savepoint for a job, returns (TriggerID, SavepointStatus, error).
func (reconciler *ClusterReconciler) takeSavepoint(jobID string) (
	string, flinkclient.SavepointStatus, error) {
	var log = reconciler.log
	var apiBaseURL = getFlinkAPIBaseURL(reconciler.observed.cluster)
	var triggerID, status, err = reconciler.flinkClient.TakeSavepoint(
		apiBaseURL, jobID, *reconciler.observed.cluster.Spec.Job.SavepointsDir)
	log.Info(
		"Savepoint status.",
		"jobID", jobID,
		"triggerID", triggerID.RequestID,
		"state", status.State,
		"failureCause", status.FailureCause,
		"error", err)
	if err == nil && len(status.FailureCause.StackTrace) > 0 {
		err = fmt.Errorf("%s", status.FailureCause.StackTrace)
	}
	return triggerID.RequestID, status, err
}

func (reconciler *ClusterReconciler) isJobFinished() bool {
	var jobStatus = reconciler.observed.cluster.Status.Components.Job
	return jobStatus != nil &&
		(jobStatus.State == v1alpha1.JobState.Succeeded ||
			jobStatus.State == v1alpha1.JobState.Failed)
}

func (reconciler *ClusterReconciler) updateSavepointStatus(
	triggerID string, savepointErr error) error {
	var cluster = v1alpha1.FlinkCluster{}
	reconciler.observed.cluster.DeepCopyInto(&cluster)
	cluster.Status = reconciler.observed.cluster.Status
	cluster.Status.Components.Job.LastSavepointTriggerID = triggerID
	var tc = &TimeConverter{}
	cluster.Status.Components.Job.LastSavepointTime = tc.ToString(time.Now())
	return reconciler.k8sClient.Update(reconciler.context, &cluster)
}
