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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	flinkoperatorv1alpha1 "github.com/googlecloudplatform/flink-operator/api/v1alpha1"
)

// FlinkSessionClusterReconciler reconciles a FlinkSessionCluster object
type FlinkSessionClusterReconciler struct {
	client.Client
	Log logr.Logger
}

// FlinkSessionClusterReconcileState holds the context and state for a
// reconcile request.
type _FlinkSessionClusterReconcileState struct {
	request             ctrl.Request
	context             context.Context
	log                 logr.Logger
	flinkSessionCluster flinkoperatorv1alpha1.FlinkSessionCluster
}

// +kubebuilder:rbac:groups=flinkoperator.k8s.io,resources=flinksessionclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=flinkoperator.k8s.io,resources=flinksessionclusters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments/status,verbs=get
// +kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=services/status,verbs=get

func (reconciler *FlinkSessionClusterReconciler) Reconcile(request ctrl.Request) (ctrl.Result, error) {
	var context = context.Background()
	var log = reconciler.Log.WithValues("flinksessioncluster", request.NamespacedName)
	var state = _FlinkSessionClusterReconcileState{
		request:             request,
		context:             context,
		log:                 log,
		flinkSessionCluster: flinkoperatorv1alpha1.FlinkSessionCluster{},
	}
	var flinkSessionCluster = &state.flinkSessionCluster

	log.Info("========== Start reconciling ==========")

	// Get the FlinkSessionCluster resource.
	var err = reconciler.Get(context, request.NamespacedName, flinkSessionCluster)
	if err != nil {
		log.Info("Failed to get FlinkSessionCluster, it might have been deleted by the user", "error", err)
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	log.Info("Found FlinkSessionCluster", "resource", flinkSessionCluster)

	// Create or update JobManager deployment.
	var jobManagerDeployment = reconciler.getDesiredJobManagerDeployment(&state)
	err = reconciler.createOrUpdateDeployment(&state, &jobManagerDeployment, "JobManager")
	if err != nil {
		return ctrl.Result{}, err
	}

	// Create or update JobManager service.
	var jobManagerService = reconciler.getDesiredJobManagerService(&state)
	err = reconciler.createOrUpdateService(&state, &jobManagerService, "JobManager")
	if err != nil {
		return ctrl.Result{}, err
	}

	// Create or update TaskManager deployment.
	var taskManagerDeployment = reconciler.getDesiredTaskManagerDeployment(&state)
	err = reconciler.createOrUpdateDeployment(&state, &taskManagerDeployment, "TaskManager")
	if err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (reconciler *FlinkSessionClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&flinkoperatorv1alpha1.FlinkSessionCluster{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Complete(reconciler)
}

func (reconciler *FlinkSessionClusterReconciler) createOrUpdateDeployment(
	reconcileState *_FlinkSessionClusterReconcileState,
	deployment *appsv1.Deployment,
	component string) error {
	var context = reconcileState.context
	var log = reconcileState.log.WithValues("component", component)
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
		err = reconciler.createDeployment(reconcileState, deployment, component)
	} else {
		log.Info("Deployment already exists", "deployment", currentDeployment)
		// TODO(dagang): compare and update if needed.
	}
	return err
}

func (reconciler *FlinkSessionClusterReconciler) createDeployment(
	reconcileState *_FlinkSessionClusterReconcileState,
	deployment *appsv1.Deployment,
	component string) error {
	var context = reconcileState.context
	var log = reconcileState.log.WithValues("component", component)
	log.Info("Creating deployment", "deployment", *deployment)
	var err = reconciler.Create(context, deployment)
	if err != nil {
		log.Info("Failed to create deployment", "error", err)
	} else {
		log.Info("Deployment created")
	}
	return err
}

func (reconciler *FlinkSessionClusterReconciler) updateManagerDeployment(
	reconcileState *_FlinkSessionClusterReconcileState,
	deployment *appsv1.Deployment,
	component string) error {
	var context = reconcileState.context
	var log = reconcileState.log.WithValues("component", component)
	log.Info("Updating deployment", "deployment", deployment)
	var err = reconciler.Update(context, deployment)
	if err != nil {
		log.Info("Failed to update deployment", "error", err)
	}
	return err
}

func (reconciler *FlinkSessionClusterReconciler) createOrUpdateService(
	reconcileState *_FlinkSessionClusterReconcileState,
	service *corev1.Service,
	component string) error {
	var context = reconcileState.context
	var log = reconcileState.log.WithValues("component", component)
	var currentService = corev1.Service{}
	// Check if service already exists.
	var err = reconciler.Get(
		context,
		types.NamespacedName{
			Namespace: service.ObjectMeta.Namespace,
			Name:      service.Name,
		},
		&currentService)
	if err != nil {
		err = reconciler.createService(reconcileState, service, component)
	} else {
		log.Info("Service already exists", "resource", currentService)
		// TODO(dagang): compare and update if needed.
	}
	return err
}

func (reconciler *FlinkSessionClusterReconciler) createService(
	reconcileState *_FlinkSessionClusterReconcileState,
	service *corev1.Service,
	component string) error {
	var context = reconcileState.context
	var log = reconcileState.log.WithValues("component", component)
	log.Info("Creating service", "resource", *service)
	var err = reconciler.Create(context, service)
	if err != nil {
		log.Info("Failed to create service", "error", err)
	} else {
		log.Info("Service created")
	}
	return err
}

func toOwnerReference(flinkSessionCluster *flinkoperatorv1alpha1.FlinkSessionCluster) metav1.OwnerReference {
	return metav1.OwnerReference{
		APIVersion:         flinkSessionCluster.APIVersion,
		Kind:               flinkSessionCluster.Kind,
		Name:               flinkSessionCluster.Name,
		UID:                flinkSessionCluster.UID,
		Controller:         &[]bool{true}[0],
		BlockOwnerDeletion: &[]bool{false}[0],
	}
}

func (reconciler *FlinkSessionClusterReconciler) getDesiredJobManagerDeployment(
	reconcileState *_FlinkSessionClusterReconcileState) appsv1.Deployment {
	var request = reconcileState.request
	var flinkSessionCluster = reconcileState.flinkSessionCluster
	var imageSpec = flinkSessionCluster.Spec.ImageSpec
	var jobManagerSpec = flinkSessionCluster.Spec.JobManagerSpec
	var rpcPort = corev1.ContainerPort{Name: "rpc", ContainerPort: *jobManagerSpec.Ports.RPC}
	var blobPort = corev1.ContainerPort{Name: "blob", ContainerPort: *jobManagerSpec.Ports.Blob}
	var queryPort = corev1.ContainerPort{Name: "query", ContainerPort: *jobManagerSpec.Ports.QueryPort}
	var uiPort = corev1.ContainerPort{Name: "ui", ContainerPort: *jobManagerSpec.Ports.UI}
	var jobManagerDeploymentName = request.Name + "-jobmanager"
	var labels = map[string]string{
		"cluster":   request.Name,
		"app":       "flink",
		"component": "jobmanager",
	}
	var jobManagerDeployment = appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       request.Namespace,
			Name:            jobManagerDeploymentName,
			OwnerReferences: []metav1.OwnerReference{toOwnerReference(&flinkSessionCluster)},
			Labels:          labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: jobManagerSpec.Replicas,
			Selector: &metav1.LabelSelector{MatchLabels: labels},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						corev1.Container{
							Name:  "jobmanager",
							Image: *imageSpec.URI,
							Args:  []string{"jobmanager"},
							Ports: []corev1.ContainerPort{rpcPort, blobPort, queryPort, uiPort},
							Env:   []corev1.EnvVar{corev1.EnvVar{Name: "JOB_MANAGER_RPC_ADDRESS", Value: jobManagerDeploymentName}},
						},
					},
				},
			},
		},
	}
	return jobManagerDeployment
}

func (reconciler *FlinkSessionClusterReconciler) getDesiredJobManagerService(
	reconcileState *_FlinkSessionClusterReconcileState) corev1.Service {
	var request = reconcileState.request
	var flinkSessionCluster = reconcileState.flinkSessionCluster
	var jobManagerSpec = flinkSessionCluster.Spec.JobManagerSpec
	var rpcPort = corev1.ServicePort{
		Name:       "rpc",
		Port:       *jobManagerSpec.Ports.RPC,
		TargetPort: intstr.FromString("rpc")}
	var blobPort = corev1.ServicePort{
		Name:       "blob",
		Port:       *jobManagerSpec.Ports.Blob,
		TargetPort: intstr.FromString("blob")}
	var queryPort = corev1.ServicePort{
		Name:       "query",
		Port:       *jobManagerSpec.Ports.QueryPort,
		TargetPort: intstr.FromString("query")}
	var uiPort = corev1.ServicePort{
		Name:       "ui",
		Port:       *jobManagerSpec.Ports.UI,
		TargetPort: intstr.FromString("ui")}
	var jobManagerServiceName = request.Name + "-jobmanager"
	var labels = map[string]string{
		"cluster":   request.Name,
		"app":       "flink",
		"component": "jobmanager",
	}
	var jobManagerService = corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       request.Namespace,
			Name:            jobManagerServiceName,
			OwnerReferences: []metav1.OwnerReference{toOwnerReference(&flinkSessionCluster)},
			Labels:          labels,
		},
		Spec: corev1.ServiceSpec{
			Selector: labels,
			Ports:    []corev1.ServicePort{rpcPort, blobPort, queryPort, uiPort},
		},
	}
	return jobManagerService
}

func (reconciler *FlinkSessionClusterReconciler) getDesiredTaskManagerDeployment(
	reconcileState *_FlinkSessionClusterReconcileState) appsv1.Deployment {
	var request = reconcileState.request
	var flinkSessionCluster = reconcileState.flinkSessionCluster
	var imageSpec = flinkSessionCluster.Spec.ImageSpec
	var taskManagerSpec = flinkSessionCluster.Spec.TaskManagerSpec
	var dataPort = corev1.ContainerPort{Name: "data", ContainerPort: *taskManagerSpec.Ports.Data}
	var rpcPort = corev1.ContainerPort{Name: "rpc", ContainerPort: *taskManagerSpec.Ports.RPC}
	var queryPort = corev1.ContainerPort{Name: "query", ContainerPort: *taskManagerSpec.Ports.Query}
	var taskManagerDeploymentName = request.Name + "-taskmanager"
	var jobManagerDeploymentName = request.Name + "-jobmanager"
	var labels = map[string]string{
		"cluster":   request.Name,
		"app":       "flink",
		"component": "taskmanager",
	}
	var taskManagerDeployment = appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       request.Namespace,
			Name:            taskManagerDeploymentName,
			OwnerReferences: []metav1.OwnerReference{toOwnerReference(&flinkSessionCluster)},
			Labels:          labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: taskManagerSpec.Replicas,
			Selector: &metav1.LabelSelector{MatchLabels: labels},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						corev1.Container{
							Name:  "taskmanager",
							Image: *imageSpec.URI,
							Args:  []string{"taskmanager"},
							Ports: []corev1.ContainerPort{dataPort, rpcPort, queryPort},
							Env:   []corev1.EnvVar{corev1.EnvVar{Name: "JOB_MANAGER_RPC_ADDRESS", Value: jobManagerDeploymentName}},
						},
					},
				},
			},
		},
	}
	return taskManagerDeployment
}

func (reconciler *FlinkSessionClusterReconciler) updateStatus(
	reconcileState *_FlinkSessionClusterReconcileState,
	status string) error {
	var flinkSessionCluster = flinkoperatorv1alpha1.FlinkSessionCluster{}
	reconcileState.flinkSessionCluster.DeepCopyInto(&flinkSessionCluster)
	flinkSessionCluster.Status = flinkoperatorv1alpha1.FlinkSessionClusterStatus{Status: status}
	return reconciler.Update(reconcileState.context, &flinkSessionCluster)
}
