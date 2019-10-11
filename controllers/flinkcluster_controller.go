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
	flinkoperatorv1alpha1 "github.com/googlecloudplatform/flink-operator/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// FlinkClusterReconciler reconciles a FlinkCluster object
type FlinkClusterReconciler struct {
	client.Client
	Log logr.Logger
	mgr ctrl.Manager
}

// +kubebuilder:rbac:groups=flinkoperator.k8s.io,resources=flinkclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=flinkoperator.k8s.io,resources=flinkclusters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments/status,verbs=get
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=pods/status,verbs=get
// +kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=services/status,verbs=get
// +kubebuilder:rbac:groups=core,resources=events,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=events/status,verbs=get
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=batch,resources=jobs/status,verbs=get

// Reconcile the observed state towards the desired state for a FlinkCluster custom resource.
func (reconciler *FlinkClusterReconciler) Reconcile(
	request ctrl.Request) (ctrl.Result, error) {
	var handler = _FlinkClusterHandler{
		k8sClient: reconciler,
		request:   request,
		context:   context.Background(),
		log: reconciler.Log.WithValues(
			"flinkcluster", request.NamespacedName),
		eventRecorder: reconciler.mgr.GetEventRecorderFor("FlinkOperator"),
		observedState: _ObservedClusterState{},
	}
	return handler.Reconcile(request)
}

// SetupWithManager registers this reconciler with the controller manager and
// starts watching FlinkCluster, Deployment and Service resources.
func (reconciler *FlinkClusterReconciler) SetupWithManager(
	mgr ctrl.Manager) error {
	reconciler.mgr = mgr
	return ctrl.NewControllerManagedBy(mgr).
		For(&flinkoperatorv1alpha1.FlinkCluster{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Owns(&batchv1.Job{}).
		Complete(reconciler)
}

// _FlinkClusterHandler holds the context and state for a
// reconcile request.
type _FlinkClusterHandler struct {
	k8sClient     client.Client
	request       ctrl.Request
	context       context.Context
	log           logr.Logger
	eventRecorder record.EventRecorder
	observedState _ObservedClusterState
	desiredState  _DesiredClusterState
}

func (handler *_FlinkClusterHandler) Reconcile(
	request ctrl.Request) (ctrl.Result, error) {
	var k8sClient = handler.k8sClient
	var log = handler.log
	var context = handler.context
	var observedState = &handler.observedState
	var desiredState = &handler.desiredState
	var err error

	log.Info("============================================================")
	log.Info("---------- 1. Observe the current state ----------")

	var observer = _ClusterStateObserver{
		k8sClient: k8sClient, request: request, context: context, log: log}
	err = observer.observe(observedState)
	if err != nil {
		log.Error(err, "Failed to observe the current state")
		return ctrl.Result{}, err
	}

	log.Info("---------- 2. Compute the desired state ----------")
	*desiredState = getDesiredClusterState(observedState.cluster)
	if desiredState.JmDeployment != nil {
		log.Info("Desired state", "JobManager deployment", *desiredState.JmDeployment)
	} else {
		log.Info("Desired state", "JobManager deployment", "nil")
	}
	if desiredState.JmService != nil {
		log.Info("Desired state", "JobManager service", *desiredState.JmService)
	} else {
		log.Info("Desired state", "JobManager service", "nil")
	}
	if desiredState.TmDeployment != nil {
		log.Info("Desired state", "TaskManager deployment", *desiredState.TmDeployment)
	} else {
		log.Info("Desired state", "TaskManager deployment", "nil")
	}
	if desiredState.Job != nil {
		log.Info("Desired state", "Job", *desiredState.Job)
	} else {
		log.Info("Desired state", "Job", "nil")
	}

	log.Info("---------- 3. Update cluster status ----------")

	// Update cluster status if changed.
	var updater = _ClusterStatusUpdater{
		k8sClient:     handler.k8sClient,
		context:       handler.context,
		log:           handler.log,
		eventRecorder: handler.eventRecorder,
		observedState: handler.observedState,
	}
	err = updater.updateClusterStatusIfChanged()
	if err != nil {
		log.Error(err, "Failed to update cluster status")
		return ctrl.Result{}, err
	}

	log.Info("---------- 4. Take actions ----------")

	var reconciler = _ClusterReconciler{
		k8sClient:     handler.k8sClient,
		context:       handler.context,
		log:           handler.log,
		observedState: handler.observedState,
		desiredState:  handler.desiredState,
	}
	err = reconciler.reconcile()
	if err != nil {
		log.Error(err, "Failed to reconcile")
		return ctrl.Result{
			RequeueAfter: 5 * time.Second,
			Requeue:      true,
		}, err
	}

	return ctrl.Result{}, nil
}
