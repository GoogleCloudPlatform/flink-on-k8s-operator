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
	"github.com/googlecloudplatform/flink-operator/controllers/history"
	"time"

	"github.com/go-logr/logr"
	v1beta1 "github.com/googlecloudplatform/flink-operator/api/v1beta1"
	"github.com/googlecloudplatform/flink-operator/controllers/flinkclient"
	"github.com/googlecloudplatform/flink-operator/controllers/model"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// controllerKind contains the schema.GroupVersionKind for this controller type.
var controllerKind = v1beta1.GroupVersion.WithKind("FlinkCluster")

// FlinkClusterReconciler reconciles a FlinkCluster object
type FlinkClusterReconciler struct {
	Client client.Client
	Log    logr.Logger
	Mgr    ctrl.Manager
}

// +kubebuilder:rbac:groups=flinkoperator.k8s.io,resources=flinkclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=flinkoperator.k8s.io,resources=flinkclusters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments/status,verbs=get
// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=statefulsets/status,verbs=get
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=pods/status,verbs=get
// +kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=services/status,verbs=get
// +kubebuilder:rbac:groups=core,resources=events,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=events/status,verbs=get
// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=batch,resources=jobs/status,verbs=get
// +kubebuilder:rbac:groups=extensions,resources=ingresses,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=extensions,resources=ingresses/status,verbs=get

// Reconcile the observed state towards the desired state for a FlinkCluster custom resource.
func (reconciler *FlinkClusterReconciler) Reconcile(
	request ctrl.Request) (ctrl.Result, error) {
	var log = reconciler.Log.WithValues(
		"cluster", request.NamespacedName)
	var handler = FlinkClusterHandler{
		k8sClient: reconciler.Client,
		flinkClient: flinkclient.FlinkClient{
			Log:        log,
			HTTPClient: flinkclient.HTTPClient{Log: log},
		},
		request:  request,
		context:  context.Background(),
		log:      log,
		recorder: reconciler.Mgr.GetEventRecorderFor("FlinkOperator"),
		observed: ObservedClusterState{},
	}
	return handler.reconcile(request)
}

// SetupWithManager registers this reconciler with the controller manager and
// starts watching FlinkCluster, Deployment and Service resources.
func (reconciler *FlinkClusterReconciler) SetupWithManager(
	mgr ctrl.Manager) error {
	reconciler.Mgr = mgr
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1beta1.FlinkCluster{}).
		Owns(&appsv1.Deployment{}).
		Owns(&appsv1.StatefulSet{}).
		Owns(&corev1.Service{}).
		Owns(&batchv1.Job{}).
		Complete(reconciler)
}

// FlinkClusterHandler holds the context and state for a
// reconcile request.
type FlinkClusterHandler struct {
	k8sClient         client.Client
	flinkClient       flinkclient.FlinkClient
	request           ctrl.Request
	context           context.Context
	log               logr.Logger
	recorder          record.EventRecorder
	observed          ObservedClusterState
	controllerHistory history.Interface
	desired           model.DesiredClusterState
}

func (handler *FlinkClusterHandler) reconcile(
	request ctrl.Request) (ctrl.Result, error) {
	var k8sClient = handler.k8sClient
	var flinkClient = handler.flinkClient
	var log = handler.log
	var context = handler.context
	var observed = &handler.observed
	var desired = &handler.desired
	var statusChanged bool
	var err error

	// History interface
	var history = history.NewHistory(k8sClient, context)

	log.Info("============================================================")
	log.Info("---------- 1. Observe the current state ----------")

	var observer = ClusterStateObserver{
		k8sClient:   k8sClient,
		flinkClient: flinkClient,
		request:     request,
		context:     context,
		log:         log,
		history:     history,
	}
	err = observer.observe(observed)
	if err != nil {
		log.Error(err, "Failed to observe the current state")
		return ctrl.Result{}, err
	}

	// Sync history and observe revision status
	err = observer.syncRevisionStatus(observed)
	if err != nil {
		log.Error(err, "Failed to sync flinkCluster history")
		return ctrl.Result{}, err
	}

	log.Info("---------- 2. Update cluster status ----------")

	var updater = ClusterStatusUpdater{
		k8sClient: k8sClient,
		context:   context,
		log:       log,
		recorder:  handler.recorder,
		observed:  handler.observed,
	}
	statusChanged, err = updater.updateStatusIfChanged()
	if err != nil {
		log.Error(err, "Failed to update cluster status")
		return ctrl.Result{}, err
	}
	if statusChanged {
		log.Info(
			"Wait status to be stable before taking further actions.",
			"requeueAfter",
			5)
		return ctrl.Result{
			Requeue: true, RequeueAfter: 5 * time.Second,
		}, nil
	}

	log.Info("---------- 3. Compute the desired state ----------")

	*desired = getDesiredClusterState(observed, time.Now())
	if desired.ConfigMap != nil {
		log.Info("Desired state", "ConfigMap", *desired.ConfigMap)
	} else {
		log.Info("Desired state", "ConfigMap", "nil")
	}
	if desired.JmStatefulSet != nil {
		log.Info("Desired state", "JobManager StatefulSet", *desired.JmStatefulSet)
	} else {
		log.Info("Desired state", "JobManager StatefulSet", "nil")
	}
	if desired.JmService != nil {
		log.Info("Desired state", "JobManager service", *desired.JmService)
	} else {
		log.Info("Desired state", "JobManager service", "nil")
	}
	if desired.JmIngress != nil {
		log.Info("Desired state", "JobManager ingress", *desired.JmIngress)
	} else {
		log.Info("Desired state", "JobManager ingress", "nil")
	}
	if desired.TmStatefulSet != nil {
		log.Info("Desired state", "TaskManager StatefulSet", *desired.TmStatefulSet)
	} else {
		log.Info("Desired state", "TaskManager StatefulSet", "nil")
	}
	if desired.Job != nil {
		log.Info("Desired state", "Job", *desired.Job)
	} else {
		log.Info("Desired state", "Job", "nil")
	}

	log.Info("---------- 4. Take actions ----------")

	var reconciler = ClusterReconciler{
		k8sClient:   k8sClient,
		flinkClient: flinkClient,
		context:     context,
		log:         log,
		observed:    handler.observed,
		desired:     handler.desired,
		recorder:    handler.recorder,
	}
	result, err := reconciler.reconcile()
	if err != nil {
		log.Error(err, "Failed to reconcile")
	}
	if result.RequeueAfter > 0 {
		log.Info("Requeue reconcile request", "after", result.RequeueAfter)
	}

	return result, err
}
