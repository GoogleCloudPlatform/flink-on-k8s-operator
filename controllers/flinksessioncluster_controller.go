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

// +kubebuilder:rbac:groups=flinkoperator.k8s.io,resources=flinksessionclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=flinkoperator.k8s.io,resources=flinksessionclusters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments/status,verbs=get

func (r *FlinkSessionClusterReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	var context = context.Background()
	var log = r.Log.WithValues("flinksessioncluster", req.NamespacedName)

	// Get the FlinkSessionCluster resource.
	var flinkSessionCluster = flinkoperatorv1alpha1.FlinkSessionCluster{}
	var err = r.Get(context, req.NamespacedName, &flinkSessionCluster)
	if err != nil {
		log.Error(err, "Failed to get flinksessioncluster")
		return ctrl.Result{}, err
	}
	log.Info("Reconciling", "desired state", flinkSessionCluster)

	// Create JobManager deployment.
	err = r.createJobManagerDeployment(req, context, log, &flinkSessionCluster)
	if err != nil {
		log.Info("Failed to create JobManager deployment", "error", err)
		return ctrl.Result{}, nil
	}

	return ctrl.Result{}, nil
}

func (r *FlinkSessionClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&flinkoperatorv1alpha1.FlinkSessionCluster{}).
		Complete(r)
}

func (r *FlinkSessionClusterReconciler) createJobManagerDeployment(
	req ctrl.Request,
	context context.Context,
	log logr.Logger,
	flinkSessionCluster *flinkoperatorv1alpha1.FlinkSessionCluster) error {
	var jobManagerSpec = flinkSessionCluster.Spec.JobManagerSpec
	var rpcPort = corev1.ContainerPort{Name: "rpc", ContainerPort: *jobManagerSpec.Ports.RPC}
	var blobPort = corev1.ContainerPort{Name: "blob", ContainerPort: *jobManagerSpec.Ports.Blob}
	var queryPort = corev1.ContainerPort{Name: "query", ContainerPort: *jobManagerSpec.Ports.QueryPort}
	var uiPort = corev1.ContainerPort{Name: "ui", ContainerPort: *jobManagerSpec.Ports.UI}
	var podLabel = "flink-jobmanager"
	var jobManagerDeploymentName = req.Name + "-jobmanager"
	var jobManagerDeployment = appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: req.Namespace,
			Name:      jobManagerDeploymentName,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: jobManagerSpec.Replicas,
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": podLabel}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": podLabel,
					},
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
	return r.Create(context, &jobManagerDeployment)
}

func (r *FlinkSessionClusterReconciler) updateStatus(
	context context.Context,
	flinkSessionCluster *flinkoperatorv1alpha1.FlinkSessionCluster,
	status string) error {
	flinkSessionCluster.Status = flinkoperatorv1alpha1.FlinkSessionClusterStatus{Status: status}
	return r.Update(context, flinkSessionCluster)
}
