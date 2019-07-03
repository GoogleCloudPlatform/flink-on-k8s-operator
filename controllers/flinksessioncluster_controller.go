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

	// Get the FlinkSessionCluster resource.
	var err = reconciler.Get(context, request.NamespacedName, flinkSessionCluster)
	if err != nil {
		log.Info("Failed to get FlinkSessionCluster, it might have been deleted by the user", "error", err)
		return ctrl.Result{}, nil
	}
	log.Info("Reconciling", "resource", flinkSessionCluster)

	// Create JobManager deployment.
	err = reconciler.createJobManagerDeployment(&state)
	if err != nil {
		log.Info("Failed to create JobManager deployment", "error", err)
		return ctrl.Result{}, err
	}
	log.Info("JobManager deployment created")

	return ctrl.Result{}, nil
}

func (reconciler *FlinkSessionClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&flinkoperatorv1alpha1.FlinkSessionCluster{}).
		Complete(reconciler)
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

func (reconciler *FlinkSessionClusterReconciler) createJobManagerDeployment(
	reconcileState *_FlinkSessionClusterReconcileState) error {
	var request = reconcileState.request
	var log = reconcileState.log
	var flinkSessionCluster = reconcileState.flinkSessionCluster
	var jobManagerSpec = flinkSessionCluster.Spec.JobManagerSpec
	var rpcPort = corev1.ContainerPort{Name: "rpc", ContainerPort: *jobManagerSpec.Ports.RPC}
	var blobPort = corev1.ContainerPort{Name: "blob", ContainerPort: *jobManagerSpec.Ports.Blob}
	var queryPort = corev1.ContainerPort{Name: "query", ContainerPort: *jobManagerSpec.Ports.QueryPort}
	var uiPort = corev1.ContainerPort{Name: "ui", ContainerPort: *jobManagerSpec.Ports.UI}
	var jobManagerDeploymentName = request.Name + "-jobmanager"
	var labels = map[string]string{
		"flink-cluster":   request.Name,
		"flink-component": "jobmanager",
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
							Image: "flink:latest",
							Args:  []string{"jobmanager"},
							Ports: []corev1.ContainerPort{rpcPort, blobPort, queryPort, uiPort},
							Env:   []corev1.EnvVar{corev1.EnvVar{Name: "JOB_MANAGER_RPC_ADDRESS", Value: jobManagerDeploymentName}},
						},
					},
				},
			},
		},
	}
	log.Info("Creating JobManager deployment", "deployment", jobManagerDeployment)
	return reconciler.Create(reconcileState.context, &jobManagerDeployment)
}

func (reconciler *FlinkSessionClusterReconciler) updateStatus(
	reconcileState *_FlinkSessionClusterReconcileState,
	status string) error {
	var flinkSessionCluster = flinkoperatorv1alpha1.FlinkSessionCluster{}
	reconcileState.flinkSessionCluster.DeepCopyInto(&flinkSessionCluster)
	flinkSessionCluster.Status = flinkoperatorv1alpha1.FlinkSessionClusterStatus{Status: status}
	return reconciler.Update(reconcileState.context, &flinkSessionCluster)
}
