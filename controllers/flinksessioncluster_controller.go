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
	"time"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	flinkoperatorv1alpha1 "github.com/googlecloudplatform/flink-operator/api/v1alpha1"
)

// FlinkSessionClusterReconciler reconciles a FlinkSessionCluster object
type FlinkSessionClusterReconciler struct {
	client.Client
	Log logr.Logger
}

// +kubebuilder:rbac:groups=flinkoperator.k8s.io,resources=flinksessionclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=flinkoperator.k8s.io,resources=flinksessionclusters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments/status,verbs=get
// +kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=services/status,verbs=get
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=batch,resources=jobs/status,verbs=get

// Reconcile the observed state towards the desired state for a FlinkSessionCluster custom resource.
func (reconciler *FlinkSessionClusterReconciler) Reconcile(request ctrl.Request) (ctrl.Result, error) {
	var handler = _FlinkSessionClusterHandler{
		reconciler:          reconciler,
		request:             request,
		context:             context.Background(),
		log:                 reconciler.Log.WithValues("flinksessioncluster", request.NamespacedName),
		flinkSessionCluster: flinkoperatorv1alpha1.FlinkSessionCluster{},
	}
	return handler.Reconcile(request)
}

// SetupWithManager registers this reconciler with the controller manager and
// starts watching FlinkSessionCluster, Deployment and Service resources.
func (reconciler *FlinkSessionClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&flinkoperatorv1alpha1.FlinkSessionCluster{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Owns(&batchv1.Job{}).
		Complete(reconciler)
}

// _FlinkSessionClusterHandler holds the context and state for a
// reconcile request.
type _FlinkSessionClusterHandler struct {
	reconciler                    *FlinkSessionClusterReconciler
	request                       ctrl.Request
	context                       context.Context
	log                           logr.Logger
	flinkSessionCluster           flinkoperatorv1alpha1.FlinkSessionCluster
	observedJobManagerDeployment  *appsv1.Deployment
	observedJobManagerService     *corev1.Service
	observedTaskManagerDeployment *appsv1.Deployment
	observedJob                   *batchv1.Job
}

func (handler *_FlinkSessionClusterHandler) Reconcile(
	request ctrl.Request) (ctrl.Result, error) {
	var reconciler = handler.reconciler
	var log = handler.log
	var context = handler.context
	var flinkSessionCluster = &handler.flinkSessionCluster

	log.Info("========== Start reconciling ==========")

	// Get the FlinkSessionCluster resource.
	var err = reconciler.Get(context, request.NamespacedName, flinkSessionCluster)
	if err != nil {
		log.Info("Failed to get FlinkSessionCluster, it might have been deleted", "error", err)
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	log.Info("Found FlinkSessionCluster", "resource", flinkSessionCluster)

	// Create or update JobManager deployment.
	var jmDeployment = getDesiredJobManagerDeployment(flinkSessionCluster)
	err = handler.createOrUpdateDeployment(&jmDeployment, "JobManager")
	if err != nil {
		return ctrl.Result{}, err
	}

	// Create or update JobManager service.
	var jmService = getDesiredJobManagerService(flinkSessionCluster)
	err = handler.createOrUpdateService(&jmService, "JobManager")
	if err != nil {
		return ctrl.Result{}, err
	}

	// Create or update TaskManager deployment.
	var tmDeployment = getDesiredTaskManagerDeployment(flinkSessionCluster)
	err = handler.createOrUpdateDeployment(&tmDeployment, "TaskManager")
	if err != nil {
		return ctrl.Result{}, err
	}

	// (Optional) Submit job
	if flinkSessionCluster.Status.State ==
		flinkoperatorv1alpha1.ClusterState.Running {
		var job = getDesiredJob(flinkSessionCluster)
		err = handler.submitJobIfNeeded(&job)
		if err != nil {
			log.Error(err, "Failed to submit job.")
			return ctrl.Result{}, err
		}
	}

	// TODO(dagang): Stop the cluster after the job finishes.

	// Update cluster status if needed.
	err = handler.updateClusterStatusIfNeeded()
	if err != nil {
		log.Error(err, "Failed to update cluster status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (handler *_FlinkSessionClusterHandler) createOrUpdateDeployment(
	deployment *appsv1.Deployment, component string) error {
	var context = handler.context
	var log = handler.log.WithValues("component", component)
	var reconciler = handler.reconciler
	var observedDeployment = appsv1.Deployment{}

	// Check if deployment already exists.
	var err = reconciler.Get(
		context,
		types.NamespacedName{
			Namespace: deployment.ObjectMeta.Namespace,
			Name:      deployment.Name,
		},
		&observedDeployment)
	if err != nil {
		err = handler.createDeployment(deployment, component)
		log.Error(err, "Failed to create deployment", "deployment", deployment)
	} else {
		log.Info("Deployment already exists", "deployment", observedDeployment)
		if component == "JobManager" {
			handler.observedJobManagerDeployment = &observedDeployment
		} else if component == "TaskManager" {
			handler.observedTaskManagerDeployment = &observedDeployment
		}
		// TODO(dagang): compare and update if needed.
	}
	return err
}

func (handler *_FlinkSessionClusterHandler) createDeployment(
	deployment *appsv1.Deployment, component string) error {
	var context = handler.context
	var log = handler.log.WithValues("component", component)
	var reconciler = handler.reconciler

	log.Info("Creating deployment", "deployment", *deployment)
	var err = reconciler.Create(context, deployment)
	if err != nil {
		log.Error(err, "Failed to create deployment")
	} else {
		log.Info("Deployment created")
	}
	return err
}

func (handler *_FlinkSessionClusterHandler) updateManagerDeployment(
	deployment *appsv1.Deployment, component string) error {
	var context = handler.context
	var log = handler.log.WithValues("component", component)
	var reconciler = handler.reconciler

	log.Info("Updating deployment", "deployment", deployment)
	var err = reconciler.Update(context, deployment)
	if err != nil {
		log.Error(err, "Failed to update deployment")
	} else {
		log.Info("Deployment updated")
	}
	return err
}

func (handler *_FlinkSessionClusterHandler) createOrUpdateService(
	service *corev1.Service, component string) error {
	var context = handler.context
	var log = handler.log.WithValues("component", component)
	var reconciler = handler.reconciler

	// Check if service already exists.
	var observedService = corev1.Service{}
	var err = reconciler.Get(
		context,
		types.NamespacedName{
			Namespace: service.ObjectMeta.Namespace,
			Name:      service.Name,
		},
		&observedService)
	if err != nil {
		err = handler.createService(service, component)
	} else {
		log.Info("JobManager service already exists", "resource", observedService)
		handler.observedJobManagerService = &observedService
		// TODO(dagang): compare and update if needed.
	}
	return err
}

func (handler *_FlinkSessionClusterHandler) createService(
	service *corev1.Service, component string) error {
	var context = handler.context
	var log = handler.log.WithValues("component", component)
	var reconciler = handler.reconciler

	log.Info("Creating service", "resource", *service)
	var err = reconciler.Create(context, service)
	if err != nil {
		log.Info("Failed to create service", "error", err)
	} else {
		log.Info("Service created")
	}
	return err
}

func (handler *_FlinkSessionClusterHandler) updateClusterStatusIfNeeded() error {
	var currentStatus = flinkoperatorv1alpha1.FlinkSessionClusterStatus{}
	handler.flinkSessionCluster.Status.DeepCopyInto(&currentStatus)
	// Reset timestamp
	currentStatus.LastUpdateTime = ""

	var newStatus = handler.getClusterStatus()
	if newStatus != currentStatus {
		handler.log.Info(
			"Updating status",
			"current",
			handler.flinkSessionCluster.Status,
			"new", newStatus)
		newStatus.LastUpdateTime = time.Now().Format(time.RFC3339)
		return handler.updateClusterStatus(newStatus)
	} else {
		handler.log.Info("No status change")
	}
	return nil
}

func (handler *_FlinkSessionClusterHandler) getClusterStatus() flinkoperatorv1alpha1.FlinkSessionClusterStatus {
	var status = flinkoperatorv1alpha1.FlinkSessionClusterStatus{}
	var readyComponents = 0

	// JobManager deployment.
	if handler.observedJobManagerDeployment != nil {
		status.Components.JobManagerDeployment.Name =
			handler.observedJobManagerDeployment.ObjectMeta.Name
		if handler.observedJobManagerDeployment.Status.AvailableReplicas <
			handler.observedJobManagerDeployment.Status.Replicas ||
			handler.observedJobManagerDeployment.Status.ReadyReplicas <
				handler.observedJobManagerDeployment.Status.Replicas {
			status.Components.JobManagerDeployment.State =
				flinkoperatorv1alpha1.ClusterComponentState.NotReady
		} else {
			status.Components.JobManagerDeployment.State =
				flinkoperatorv1alpha1.ClusterComponentState.Ready
			readyComponents++
		}
	}

	// JobManager service.
	if handler.observedJobManagerService != nil {
		status.Components.JobManagerService.Name =
			handler.observedJobManagerService.ObjectMeta.Name
		status.Components.JobManagerService.State =
			flinkoperatorv1alpha1.ClusterComponentState.Ready
		readyComponents++
	}

	// TaskManager deployment.
	if handler.observedTaskManagerDeployment != nil {
		status.Components.TaskManagerDeployment.Name =
			handler.observedTaskManagerDeployment.ObjectMeta.Name
		if handler.observedTaskManagerDeployment.Status.AvailableReplicas <
			handler.observedTaskManagerDeployment.Status.Replicas ||
			handler.observedTaskManagerDeployment.Status.ReadyReplicas <
				handler.observedTaskManagerDeployment.Status.Replicas {
			status.Components.TaskManagerDeployment.State =
				flinkoperatorv1alpha1.ClusterComponentState.NotReady
		} else {
			status.Components.TaskManagerDeployment.State =
				flinkoperatorv1alpha1.ClusterComponentState.Ready
			readyComponents++
		}
	}

	// (Optional) Job.
	if handler.observedJob != nil {
		status.Job = new(flinkoperatorv1alpha1.JobStatus)
		status.Job.Name = handler.observedJob.ObjectMeta.Name
		if handler.observedJob.Status.Active > 0 {
			status.Job.State = flinkoperatorv1alpha1.JobState.Running
		} else if handler.observedJob.Status.Failed > 0 {
			status.Job.State = flinkoperatorv1alpha1.JobState.Failed
		} else if handler.observedJob.Status.Succeeded > 0 {
			status.Job.State = flinkoperatorv1alpha1.JobState.Succeeded
		} else {
			status.Job.State = flinkoperatorv1alpha1.JobState.Unknown
		}
	}

	if readyComponents < 3 {
		status.State = flinkoperatorv1alpha1.ClusterState.Reconciling
	} else {
		status.State = flinkoperatorv1alpha1.ClusterState.Running
	}

	return status
}

func (handler *_FlinkSessionClusterHandler) updateClusterStatus(
	status flinkoperatorv1alpha1.FlinkSessionClusterStatus) error {
	var flinkSessionCluster = flinkoperatorv1alpha1.FlinkSessionCluster{}
	handler.flinkSessionCluster.DeepCopyInto(&flinkSessionCluster)
	flinkSessionCluster.Status = status
	return handler.reconciler.Update(handler.context, &flinkSessionCluster)
}

func (handler *_FlinkSessionClusterHandler) submitJobIfNeeded(job *batchv1.Job) error {
	var context = handler.context
	var reconciler = handler.reconciler
	var log = handler.log

	// Check if the job already exists.
	var observedJob = batchv1.Job{}
	var err = reconciler.Get(
		context,
		types.NamespacedName{
			Namespace: job.ObjectMeta.Namespace,
			Name:      job.Name,
		},
		&observedJob)
	if err != nil {
		err = handler.submitJob(job)
	} else {
		log.Info("Job already exists", "resource", observedJob)
		handler.observedJob = &observedJob
	}
	return err
}

func (handler *_FlinkSessionClusterHandler) submitJob(job *batchv1.Job) error {
	var context = handler.context
	var log = handler.log
	var reconciler = handler.reconciler

	log.Info("Submitting job", "resource", *job)
	var err = reconciler.Create(context, job)
	if err != nil {
		log.Info("Failed to submitted job", "error", err)
	} else {
		log.Info("Job submitted")
	}
	return err
}
