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
	"github.com/prometheus/common/log"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// _ClusterStateObserver gets the observed state of the cluster.
type _ClusterStateObserver struct {
	k8sClient client.Client
	request   ctrl.Request
	context   context.Context
	log       logr.Logger
}

// _ObservedClusterState holds observed state of a cluster.
type _ObservedClusterState struct {
	cluster      *flinkoperatorv1alpha1.FlinkCluster
	jmDeployment *appsv1.Deployment
	jmService    *corev1.Service
	tmDeployment *appsv1.Deployment
	job          *batchv1.Job
}

// Observes the state of the cluster and its components.
// NOT_FOUND error is ignored because it is normal, other errors are returned.
func (observer *_ClusterStateObserver) observe(
	observedState *_ObservedClusterState) error {
	var err error
	var log = observer.log

	// Cluster state.
	var observedCluster = new(flinkoperatorv1alpha1.FlinkCluster)
	err = observer.observeCluster(observedCluster)
	if err != nil {
		if client.IgnoreNotFound(err) != nil {
			log.Error(err, "Failed to get the cluster resource")
			return err
		} else {
			log.Info("Observed cluster", "cluster", "nil")
		}
	} else {
		log.Info("Observed cluster", "cluster", *observedCluster)
		observedState.cluster = observedCluster
	}

	// JobManager deployment.
	var observedJmDeployment = new(appsv1.Deployment)
	err = observer.observeJobManagerDeployment(observedJmDeployment)
	if err != nil {
		if client.IgnoreNotFound(err) != nil {
			log.Error(err, "Failed to get JobManager deployment")
			return err
		} else {
			log.Info("Observed JobManager deployment", "state", "nil")
		}
	} else {
		log.Info("Observed JobManager deployment", "state", *observedJmDeployment)
		observedState.jmDeployment = observedJmDeployment
	}

	// JobManager service.
	var observedJmService = new(corev1.Service)
	err = observer.observeJobManagerService(observedJmService)
	if err != nil {
		if client.IgnoreNotFound(err) != nil {
			log.Error(err, "Failed to get JobManager service")
			return err
		} else {
			log.Info("Observed JobManager service", "state", "nil")
		}
	} else {
		log.Info("Observed JobManager service", "state", *observedJmService)
		observedState.jmService = observedJmService
	}

	// TaskManager deployment.
	var observedTmDeployment = new(appsv1.Deployment)
	err = observer.observeTaskManagerDeployment(observedJmDeployment)
	if err != nil {
		if client.IgnoreNotFound(err) != nil {
			log.Error(err, "Failed to get JobManager service")
			return err
		} else {
			log.Info("Observed TaskManager deployment", "state", "nil")
		}
	} else {
		log.Info("Observed TaskManager deployment", "state", *observedTmDeployment)
		observedState.tmDeployment = observedTmDeployment
	}

	// (Optional) job.
	if observedState.cluster != nil && observedState.cluster.Spec.JobSpec != nil {
		var observedJob = new(batchv1.Job)
		err = observer.observeJob(observedJob)
		if err != nil {
			if client.IgnoreNotFound(err) != nil {
				log.Error(err, "Failed to get job")
				return err
			} else {
				log.Info("Observed job", "state", "nil")
			}
		} else {
			log.Info("Observed job", "state", *observedJob)
			observedState.job = observedJob
		}
	}

	return nil
}

func (observer *_ClusterStateObserver) observeCluster(
	cluster *flinkoperatorv1alpha1.FlinkCluster) error {
	return observer.k8sClient.Get(
		observer.context, observer.request.NamespacedName, cluster)
}

func (observer *_ClusterStateObserver) observeJobManagerDeployment(
	observedDeployment *appsv1.Deployment) error {
	var clusterNamespace = observer.request.Namespace
	var clusterName = observer.request.Name
	var jmDeploymentName = getJobManagerDeploymentName(clusterName)

	// Check if deployment already exists.
	var err = observer.k8sClient.Get(
		observer.context,
		types.NamespacedName{
			Namespace: clusterNamespace,
			Name:      jmDeploymentName,
		},
		observedDeployment)
	if err != nil {
		if client.IgnoreNotFound(err) != nil {
			observer.log.Error(err, "Failed to get JobManager deployment")
		} else {
			log.Info("JobManager deployment not found")
		}
	}
	return err
}

func (observer *_ClusterStateObserver) observeTaskManagerDeployment(
	observedDeployment *appsv1.Deployment) error {
	var clusterNamespace = observer.request.Namespace
	var clusterName = observer.request.Name
	var tmDeploymentName = getTaskManagerDeploymentName(clusterName)

	return observer.k8sClient.Get(
		observer.context,
		types.NamespacedName{
			Namespace: clusterNamespace,
			Name:      tmDeploymentName,
		},
		observedDeployment)
}

func (observer *_ClusterStateObserver) observeJobManagerService(
	observedService *corev1.Service) error {
	var clusterNamespace = observer.request.Namespace
	var clusterName = observer.request.Name

	return observer.k8sClient.Get(
		observer.context,
		types.NamespacedName{
			Namespace: clusterNamespace,
			Name:      getJobManagerServiceName(clusterName),
		},
		observedService)
}

func (observer *_ClusterStateObserver) observeJob(
	observedJob *batchv1.Job) error {
	var clusterNamespace = observer.request.Namespace
	var clusterName = observer.request.Name

	return observer.k8sClient.Get(
		observer.context,
		types.NamespacedName{
			Namespace: clusterNamespace,
			Name:      getJobName(clusterName),
		},
		observedJob)
}
