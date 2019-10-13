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
	flinkoperatorv1alpha1 "github.com/googlecloudplatform/flink-operator/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type _ClusterReconciler struct {
	k8sClient     client.Client
	context       context.Context
	log           logr.Logger
	observedState _ObservedClusterState
	desiredState  _DesiredClusterState
}

// Compares the desired state and the observed state, if there is a difference,
// takes actions to drive the observed state towards the desired state.
func (reconciler *_ClusterReconciler) reconcile() error {
	var err error

	// Child resources of the cluster CR will be automatically reclaimed by K8S.
	if reconciler.observedState.cluster == nil {
		reconciler.log.Info("The cluster has been deleted, no action to take")
		return nil
	}

	err = reconciler.reconcileJobManagerDeployment()
	if err != nil {
		return err
	}

	err = reconciler.reconcileJobManagerService()
	if err != nil {
		return err
	}

	err = reconciler.reconcileJobManagerIngress()
	if err != nil {
		return err
	}

	err = reconciler.reconcileTaskManagerDeployment()
	if err != nil {
		return err
	}

	err = reconciler.reconcileJob()
	if err != nil {
		return err
	}

	return nil
}

func (reconciler *_ClusterReconciler) reconcileJobManagerDeployment() error {
	return reconciler.reconcileDeployment(
		"JobManager",
		reconciler.desiredState.JmDeployment,
		reconciler.observedState.jmDeployment)
}

func (reconciler *_ClusterReconciler) reconcileTaskManagerDeployment() error {
	return reconciler.reconcileDeployment(
		"TaskManager",
		reconciler.desiredState.TmDeployment,
		reconciler.observedState.tmDeployment)
}

func (reconciler *_ClusterReconciler) reconcileDeployment(
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

func (reconciler *_ClusterReconciler) createDeployment(
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

func (reconciler *_ClusterReconciler) updateDeployment(
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

func (reconciler *_ClusterReconciler) deleteDeployment(
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

func (reconciler *_ClusterReconciler) reconcileJobManagerService() error {
	var desiredJmService = reconciler.desiredState.JmService
	var observedJmService = reconciler.observedState.jmService

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

func (reconciler *_ClusterReconciler) createService(
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

func (reconciler *_ClusterReconciler) deleteService(
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

func (reconciler *_ClusterReconciler) reconcileJobManagerIngress() error {
	var desiredJmIngress = reconciler.desiredState.JmIngress
	var observedJmIngress = reconciler.observedState.jmIngress

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

func (reconciler *_ClusterReconciler) createIngress(
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

func (reconciler *_ClusterReconciler) deleteIngress(
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

func (reconciler *_ClusterReconciler) reconcileJob() error {
	var log = reconciler.log
	var desiredJob = reconciler.desiredState.Job
	var observedJob = reconciler.observedState.job
	var observedClusterComponents = reconciler.observedState.cluster.Status.Components
	if desiredJob != nil {
		if observedJob == nil {
			var jmDeploymentReady = observedClusterComponents.JobManagerDeployment.State ==
				flinkoperatorv1alpha1.ClusterComponentState.Ready
			var jmServiceReady = observedClusterComponents.JobManagerService.State ==
				flinkoperatorv1alpha1.ClusterComponentState.Ready
			var tmDeploymentReady = observedClusterComponents.TaskManagerDeployment.State ==
				flinkoperatorv1alpha1.ClusterComponentState.Ready
			if jmDeploymentReady && jmServiceReady && tmDeploymentReady {
				return reconciler.createJob(desiredJob)
			}
			log.Info("Skip creating job, waiting for other components to be ready")
		} else {
			log.Info("Job already exists, no action")
		}
	}
	return nil
}

func (reconciler *_ClusterReconciler) createJob(job *batchv1.Job) error {
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
