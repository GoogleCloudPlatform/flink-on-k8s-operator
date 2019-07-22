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
	"fmt"

	flinkoperatorv1alpha1 "github.com/googlecloudplatform/flink-operator/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// Converter which converts the FlinkCluster spec to the desired
// underlying Kubernetes resource specs.

// _DesiredClusterState holds desired state of a cluster.
type _DesiredClusterState struct {
	jmDeployment *appsv1.Deployment
	jmService    *corev1.Service
	tmDeployment *appsv1.Deployment
	job          *batchv1.Job
}

// Gets the desired state of a cluster.
func getDesiredClusterState(
	cluster *flinkoperatorv1alpha1.FlinkCluster) _DesiredClusterState {
	// The cluster has been deleted, all resources should be cleaned up.
	if cluster == nil {
		return _DesiredClusterState{}
	} else {
		return _DesiredClusterState{
			jmDeployment: getDesiredJobManagerDeployment(cluster),
			jmService:    getDesiredJobManagerService(cluster),
			tmDeployment: getDesiredTaskManagerDeployment(cluster),
			job:          getDesiredJob(cluster),
		}
	}
}

// Gets the desired JobManager deployment spec from the FlinkCluster spec.
func getDesiredJobManagerDeployment(
	flinkCluster *flinkoperatorv1alpha1.FlinkCluster) *appsv1.Deployment {

	if flinkCluster.Status.State == flinkoperatorv1alpha1.ClusterState.Stopping ||
		flinkCluster.Status.State == flinkoperatorv1alpha1.ClusterState.Stopped {
		return nil
	}

	var clusterNamespace = flinkCluster.ObjectMeta.Namespace
	var clusterName = flinkCluster.ObjectMeta.Name
	var imageSpec = flinkCluster.Spec.ImageSpec
	var jobManagerSpec = flinkCluster.Spec.JobManagerSpec
	var rpcPort = corev1.ContainerPort{Name: "rpc", ContainerPort: *jobManagerSpec.Ports.RPC}
	var blobPort = corev1.ContainerPort{Name: "blob", ContainerPort: *jobManagerSpec.Ports.Blob}
	var queryPort = corev1.ContainerPort{Name: "query", ContainerPort: *jobManagerSpec.Ports.Query}
	var uiPort = corev1.ContainerPort{Name: "ui", ContainerPort: *jobManagerSpec.Ports.UI}
	var jobManagerDeploymentName = getJobManagerDeploymentName(clusterName)
	var labels = map[string]string{
		"cluster":   clusterName,
		"app":       "flink",
		"component": "jobmanager",
	}
	var jobManagerDeployment = &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       clusterNamespace,
			Name:            jobManagerDeploymentName,
			OwnerReferences: []metav1.OwnerReference{toOwnerReference(flinkCluster)},
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
							Name:            "jobmanager",
							Image:           imageSpec.Name,
							ImagePullPolicy: corev1.PullPolicy(*imageSpec.PullPolicy),
							Args:            []string{"jobmanager"},
							Ports:           []corev1.ContainerPort{rpcPort, blobPort, queryPort, uiPort},
							Env: []corev1.EnvVar{
								{
									Name:  "JOB_MANAGER_RPC_ADDRESS",
									Value: jobManagerDeploymentName,
								},
							},
						},
					},
				},
			},
		},
	}
	return jobManagerDeployment
}

// Gets the desired JobManager service spec from a cluster spec.
func getDesiredJobManagerService(
	flinkCluster *flinkoperatorv1alpha1.FlinkCluster) *corev1.Service {

	if flinkCluster.Status.State == flinkoperatorv1alpha1.ClusterState.Stopping ||
		flinkCluster.Status.State == flinkoperatorv1alpha1.ClusterState.Stopped {
		return nil
	}

	var clusterNamespace = flinkCluster.ObjectMeta.Namespace
	var clusterName = flinkCluster.ObjectMeta.Name
	var jobManagerSpec = flinkCluster.Spec.JobManagerSpec
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
		Port:       *jobManagerSpec.Ports.Query,
		TargetPort: intstr.FromString("query")}
	var uiPort = corev1.ServicePort{
		Name:       "ui",
		Port:       *jobManagerSpec.Ports.UI,
		TargetPort: intstr.FromString("ui")}
	var jobManagerServiceName = getJobManagerServiceName(clusterName)
	var labels = map[string]string{
		"cluster":   clusterName,
		"app":       "flink",
		"component": "jobmanager",
	}
	var jobManagerService = &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: clusterNamespace,
			Name:      jobManagerServiceName,
			OwnerReferences: []metav1.OwnerReference{
				toOwnerReference(flinkCluster)},
			Labels: labels,
		},
		Spec: corev1.ServiceSpec{
			Selector: labels,
			Ports:    []corev1.ServicePort{rpcPort, blobPort, queryPort, uiPort},
		},
	}
	// This implementation is specific to GKE, see details at
	// https://cloud.google.com/kubernetes-engine/docs/how-to/exposing-apps
	// https://cloud.google.com/kubernetes-engine/docs/how-to/internal-load-balancing
	switch jobManagerSpec.AccessScope {
	case flinkoperatorv1alpha1.AccessScope.Cluster:
		jobManagerService.Spec.Type = corev1.ServiceTypeClusterIP
	case flinkoperatorv1alpha1.AccessScope.VPC:
		jobManagerService.Spec.Type = corev1.ServiceTypeLoadBalancer
		jobManagerService.Annotations =
			map[string]string{"cloud.google.com/load-balancer-type": "Internal"}
	case flinkoperatorv1alpha1.AccessScope.External:
		jobManagerService.Spec.Type = corev1.ServiceTypeLoadBalancer
	default:
		panic(fmt.Sprintf(
			"Unknown service access cope: %v", jobManagerSpec.AccessScope))
	}
	return jobManagerService
}

// Gets the desired TaskManager deployment spec from a cluster spec.
func getDesiredTaskManagerDeployment(
	flinkCluster *flinkoperatorv1alpha1.FlinkCluster) *appsv1.Deployment {

	if flinkCluster.Status.State == flinkoperatorv1alpha1.ClusterState.Stopping ||
		flinkCluster.Status.State == flinkoperatorv1alpha1.ClusterState.Stopped {
		return nil
	}

	var clusterNamespace = flinkCluster.ObjectMeta.Namespace
	var clusterName = flinkCluster.ObjectMeta.Name
	var imageSpec = flinkCluster.Spec.ImageSpec
	var taskManagerSpec = flinkCluster.Spec.TaskManagerSpec
	var dataPort = corev1.ContainerPort{Name: "data", ContainerPort: *taskManagerSpec.Ports.Data}
	var rpcPort = corev1.ContainerPort{Name: "rpc", ContainerPort: *taskManagerSpec.Ports.RPC}
	var queryPort = corev1.ContainerPort{Name: "query", ContainerPort: *taskManagerSpec.Ports.Query}
	var taskManagerDeploymentName = getTaskManagerDeploymentName(clusterName)
	var jobManagerDeploymentName = getJobManagerDeploymentName(clusterName)
	var labels = map[string]string{
		"cluster":   clusterName,
		"app":       "flink",
		"component": "taskmanager",
	}
	var taskManagerDeployment = &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: clusterNamespace,
			Name:      taskManagerDeploymentName,
			OwnerReferences: []metav1.OwnerReference{
				toOwnerReference(flinkCluster)},
			Labels: labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &taskManagerSpec.Replicas,
			Selector: &metav1.LabelSelector{MatchLabels: labels},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						corev1.Container{
							Name:            "taskmanager",
							Image:           imageSpec.Name,
							ImagePullPolicy: corev1.PullPolicy(*imageSpec.PullPolicy),
							Args:            []string{"taskmanager"},
							Ports: []corev1.ContainerPort{
								dataPort, rpcPort, queryPort},
							Env: []corev1.EnvVar{
								{
									Name:  "JOB_MANAGER_RPC_ADDRESS",
									Value: jobManagerDeploymentName,
								},
							},
						},
					},
				},
			},
		},
	}
	return taskManagerDeployment
}

// Gets the desired job spec from a cluster spec.
func getDesiredJob(
	flinkCluster *flinkoperatorv1alpha1.FlinkCluster) *batchv1.Job {
	var jobSpec = flinkCluster.Spec.JobSpec
	if jobSpec == nil {
		return nil
	}

	if flinkCluster.Status.State ==
		flinkoperatorv1alpha1.ClusterState.Creating {
		return nil
	}

	var imageSpec = flinkCluster.Spec.ImageSpec
	var jobManagerSpec = flinkCluster.Spec.JobManagerSpec
	var clusterNamespace = flinkCluster.ObjectMeta.Namespace
	var clusterName = flinkCluster.ObjectMeta.Name
	var jobName = getJobName(clusterName)
	var jobManagerServiceName = clusterName + "-jobmanager"
	var jobManagerAddress = fmt.Sprintf(
		"%s:%d", jobManagerServiceName, *jobManagerSpec.Ports.UI)
	var labels = map[string]string{
		"cluster": clusterName,
		"app":     "flink",
	}
	var jobArgs = []string{"./bin/flink", "run"}
	jobArgs = append(jobArgs, "--jobmanager", jobManagerAddress)
	if jobSpec.ClassName != nil {
		jobArgs = append(jobArgs, "--class", *jobSpec.ClassName)
	}
	if jobSpec.Savepoint != nil {
		jobArgs = append(jobArgs, "--fromSavepoint", *jobSpec.Savepoint)
	}
	if jobSpec.AllowNonRestoredState != nil &&
		*jobSpec.AllowNonRestoredState == true {
		jobArgs = append(jobArgs, "--allowNonRestoredState")
	}
	if jobSpec.Parallelism != nil {
		jobArgs = append(
			jobArgs, "--parallelism", fmt.Sprint(*jobSpec.Parallelism))
	}
	if jobSpec.NoLoggingToStdout != nil &&
		*jobSpec.NoLoggingToStdout == true {
		jobArgs = append(jobArgs, "--sysoutLogging")
	}
	jobArgs = append(jobArgs, jobSpec.JarFile)
	jobArgs = append(jobArgs, jobSpec.Args...)
	var job = &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: clusterNamespace,
			Name:      jobName,
			OwnerReferences: []metav1.OwnerReference{
				toOwnerReference(flinkCluster)},
			Labels: labels,
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						corev1.Container{
							Name:            "main",
							Image:           imageSpec.Name,
							ImagePullPolicy: corev1.PullPolicy(*imageSpec.PullPolicy),
							Args:            jobArgs,
						},
					},
					RestartPolicy: corev1.RestartPolicy(jobSpec.RestartPolicy),
				},
			},
		},
	}
	return job
}

// Converts the FlinkCluster as owner reference for its child resources.
func toOwnerReference(
	flinkCluster *flinkoperatorv1alpha1.FlinkCluster) metav1.OwnerReference {
	return metav1.OwnerReference{
		APIVersion:         flinkCluster.APIVersion,
		Kind:               flinkCluster.Kind,
		Name:               flinkCluster.Name,
		UID:                flinkCluster.UID,
		Controller:         &[]bool{true}[0],
		BlockOwnerDeletion: &[]bool{false}[0],
	}
}

// Gets JobManager deployment name
func getJobManagerDeploymentName(clusterName string) string {
	return clusterName + "-jobmanager"
}

// Gets JobManager service name
func getJobManagerServiceName(clusterName string) string {
	return clusterName + "-jobmanager"
}

// Gets TaskManager name
func getTaskManagerDeploymentName(clusterName string) string {
	return clusterName + "-taskmanager"
}

// Gets Job name
func getJobName(clusterName string) string {
	return clusterName + "-job"
}
