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

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
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
		Complete(reconciler)
}

// _FlinkSessionClusterHandler holds the context and state for a
// reconcile request.
type _FlinkSessionClusterHandler struct {
	reconciler          *FlinkSessionClusterReconciler
	request             ctrl.Request
	context             context.Context
	log                 logr.Logger
	flinkSessionCluster flinkoperatorv1alpha1.FlinkSessionCluster
}

func (handler *_FlinkSessionClusterHandler) Reconcile(request ctrl.Request) (ctrl.Result, error) {
	var reconciler = handler.reconciler
	var log = handler.log
	var context = handler.context
	var flinkSessionCluster = &handler.flinkSessionCluster

	log.Info("========== Start reconciling ==========")

	// Get the FlinkSessionCluster resource.
	var err = reconciler.Get(context, request.NamespacedName, flinkSessionCluster)
	if err != nil {
		log.Info("Failed to get FlinkSessionCluster, it might have been deleted by the user", "error", err)
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	log.Info("Found FlinkSessionCluster", "resource", flinkSessionCluster)

	// Create or update JobManager deployment.
	var jobManagerDeployment = getDesiredJobManagerDeployment(flinkSessionCluster)
	err = handler.createOrUpdateDeployment(&jobManagerDeployment, "JobManager")
	if err != nil {
		return ctrl.Result{}, err
	}

	// Create or update JobManager service.
	var jobManagerService = getDesiredJobManagerService(flinkSessionCluster)
	err = handler.createOrUpdateService(&jobManagerService, "JobManager")
	if err != nil {
		return ctrl.Result{}, err
	}

	// Create or update TaskManager deployment.
	var taskManagerDeployment = getDesiredTaskManagerDeployment(flinkSessionCluster)
	err = handler.createOrUpdateDeployment(&taskManagerDeployment, "TaskManager")
	if err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (handler *_FlinkSessionClusterHandler) createOrUpdateDeployment(deployment *appsv1.Deployment, component string) error {
	var context = handler.context
	var log = handler.log.WithValues("component", component)
	var reconciler = handler.reconciler
	var currentDeployment = appsv1.Deployment{}

	// Check if deployment already exists.
	var err = reconciler.Get(
		context,
		types.NamespacedName{
			Namespace: deployment.ObjectMeta.Namespace,
			Name:      deployment.Name,
		},
		&currentDeployment)
	if err != nil {
		err = handler.createDeployment(deployment, component)
	} else {
		log.Info("Deployment already exists", "deployment", currentDeployment)
		// TODO(dagang): compare and update if needed.
	}
	return err
}

func (handler *_FlinkSessionClusterHandler) createDeployment(deployment *appsv1.Deployment, component string) error {
	var context = handler.context
	var log = handler.log.WithValues("component", component)
	var reconciler = handler.reconciler

	log.Info("Creating deployment", "deployment", *deployment)
	var err = reconciler.Create(context, deployment)
	if err != nil {
		log.Info("Failed to create deployment", "error", err)
	} else {
		log.Info("Deployment created")
	}
	return err
}

func (handler *_FlinkSessionClusterHandler) updateManagerDeployment(deployment *appsv1.Deployment, component string) error {
	var context = handler.context
	var log = handler.log.WithValues("component", component)
	var reconciler = handler.reconciler

	log.Info("Updating deployment", "deployment", deployment)
	var err = reconciler.Update(context, deployment)
	if err != nil {
		log.Info("Failed to update deployment", "error", err)
	}
	return err
}

func (handler *_FlinkSessionClusterHandler) createOrUpdateService(service *corev1.Service, component string) error {
	var context = handler.context
	var log = handler.log.WithValues("component", component)
	var reconciler = handler.reconciler

	// Check if service already exists.
	var currentService = corev1.Service{}
	var err = reconciler.Get(
		context,
		types.NamespacedName{
			Namespace: service.ObjectMeta.Namespace,
			Name:      service.Name,
		},
		&currentService)
	if err != nil {
		err = handler.createService(service, component)
	} else {
		log.Info("Service already exists", "resource", currentService)
		// TODO(dagang): compare and update if needed.
	}
	return err
}

func (handler *_FlinkSessionClusterHandler) createService(service *corev1.Service, component string) error {
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

func (handler *_FlinkSessionClusterHandler) updateStatus(status string) error {
	var flinkSessionCluster = flinkoperatorv1alpha1.FlinkSessionCluster{}
	handler.flinkSessionCluster.DeepCopyInto(&flinkSessionCluster)
	flinkSessionCluster.Status = flinkoperatorv1alpha1.FlinkSessionClusterStatus{Status: status}
	return handler.reconciler.Update(handler.context, &flinkSessionCluster)
}
