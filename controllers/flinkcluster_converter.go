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
	"regexp"
	"strings"
	"time"

	v1alpha1 "github.com/googlecloudplatform/flink-operator/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// Converter which converts the FlinkCluster spec to the desired
// underlying Kubernetes resource specs.

var delayDeleteClusterMinutes int32 = 5

// DesiredClusterState holds desired state of a cluster.
type DesiredClusterState struct {
	JmDeployment *appsv1.Deployment
	JmService    *corev1.Service
	JmIngress    *extensionsv1beta1.Ingress
	TmDeployment *appsv1.Deployment
	Job          *batchv1.Job
}

// Gets the desired state of a cluster.
func getDesiredClusterState(
	cluster *v1alpha1.FlinkCluster,
	now time.Time) DesiredClusterState {
	// The cluster has been deleted, all resources should be cleaned up.
	if cluster == nil {
		return DesiredClusterState{}
	}
	return DesiredClusterState{
		JmDeployment: getDesiredJobManagerDeployment(cluster, now),
		JmService:    getDesiredJobManagerService(cluster, now),
		JmIngress:    getDesiredJobManagerIngress(cluster, now),
		TmDeployment: getDesiredTaskManagerDeployment(cluster, now),
		Job:          getDesiredJob(cluster),
	}
}

// Gets the desired JobManager deployment spec from the FlinkCluster spec.
func getDesiredJobManagerDeployment(
	flinkCluster *v1alpha1.FlinkCluster,
	now time.Time) *appsv1.Deployment {

	if flinkCluster.Status.State == v1alpha1.ClusterState.Stopped {
		return nil
	}

	if isStopDelayExpired(flinkCluster.Status, delayDeleteClusterMinutes, now) {
		return nil
	}

	var clusterNamespace = flinkCluster.ObjectMeta.Namespace
	var clusterName = flinkCluster.ObjectMeta.Name
	var imageSpec = flinkCluster.Spec.Image
	var jobManagerSpec = flinkCluster.Spec.JobManager
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
	var envVars = []corev1.EnvVar{
		{
			Name:  "JOB_MANAGER_RPC_ADDRESS",
			Value: jobManagerDeploymentName,
		},
		{
			Name: "JOB_MANAGER_CPU_LIMIT",
			ValueFrom: &corev1.EnvVarSource{
				ResourceFieldRef: &corev1.ResourceFieldSelector{
					ContainerName: "jobmanager",
					Resource:      "limits.cpu",
					Divisor:       resource.MustParse("1m"),
				},
			},
		},
		{
			Name: "JOB_MANAGER_MEMORY_LIMIT",
			ValueFrom: &corev1.EnvVarSource{
				ResourceFieldRef: &corev1.ResourceFieldSelector{
					ContainerName: "jobmanager",
					Resource:      "limits.memory",
					Divisor:       resource.MustParse("1Mi"),
				},
			},
		},
		{
			Name:  "FLINK_PROPERTIES",
			Value: getFlinkProperties(flinkCluster.Spec.FlinkProperties),
		},
	}
	envVars = append(envVars, flinkCluster.Spec.EnvVars...)
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
							ImagePullPolicy: imageSpec.PullPolicy,
							Args:            []string{"jobmanager"},
							Ports: []corev1.ContainerPort{
								rpcPort, blobPort, queryPort, uiPort},
							Resources:    jobManagerSpec.Resources,
							Env:          envVars,
							VolumeMounts: jobManagerSpec.Mounts,
						},
					},
					Volumes:          jobManagerSpec.Volumes,
					NodeSelector:     jobManagerSpec.NodeSelector,
					ImagePullSecrets: imageSpec.PullSecrets,
				},
			},
		},
	}
	return jobManagerDeployment
}

// Gets the desired JobManager service spec from a cluster spec.
func getDesiredJobManagerService(
	flinkCluster *v1alpha1.FlinkCluster,
	now time.Time) *corev1.Service {

	if flinkCluster.Status.State == v1alpha1.ClusterState.Stopped {
		return nil
	}

	if isStopDelayExpired(flinkCluster.Status, delayDeleteClusterMinutes, now) {
		return nil
	}

	var clusterNamespace = flinkCluster.ObjectMeta.Namespace
	var clusterName = flinkCluster.ObjectMeta.Name
	var jobManagerSpec = flinkCluster.Spec.JobManager
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
	case v1alpha1.AccessScope.Cluster:
		jobManagerService.Spec.Type = corev1.ServiceTypeClusterIP
	case v1alpha1.AccessScope.VPC:
		jobManagerService.Spec.Type = corev1.ServiceTypeLoadBalancer
		jobManagerService.Annotations =
			map[string]string{"cloud.google.com/load-balancer-type": "Internal"}
	case v1alpha1.AccessScope.External:
		jobManagerService.Spec.Type = corev1.ServiceTypeLoadBalancer
	default:
		panic(fmt.Sprintf(
			"Unknown service access cope: %v", jobManagerSpec.AccessScope))
	}
	return jobManagerService
}

// Gets the desired JobManager ingress spec from a cluster spec.
func getDesiredJobManagerIngress(
	flinkCluster *v1alpha1.FlinkCluster,
	now time.Time) *extensionsv1beta1.Ingress {
	var jobManagerIngressSpec = flinkCluster.Spec.JobManager.Ingress
	if jobManagerIngressSpec == nil {
		return nil
	}

	if flinkCluster.Status.State == v1alpha1.ClusterState.Stopped {
		return nil
	}

	if isStopDelayExpired(flinkCluster.Status, delayDeleteClusterMinutes, now) {
		return nil
	}

	var clusterNamespace = flinkCluster.ObjectMeta.Namespace
	var clusterName = flinkCluster.ObjectMeta.Name
	var jobManagerServiceName = getJobManagerServiceName(clusterName)
	var jobManagerServiceUIPort = intstr.FromString("ui")
	var ingressName = getJobManagerIngressName(clusterName)
	var ingressAnnotations = jobManagerIngressSpec.Annotations
	var ingressHost string
	var ingressTLS []extensionsv1beta1.IngressTLS
	var labels = map[string]string{
		"cluster":   clusterName,
		"app":       "flink",
		"component": "jobmanager",
	}
	if jobManagerIngressSpec.HostFormat != nil {
		ingressHost = getJobManagerIngressHost(*jobManagerIngressSpec.HostFormat, clusterName)
	}
	if jobManagerIngressSpec.UseTLS != nil && *jobManagerIngressSpec.UseTLS == true {
		var secretName string
		var hosts []string
		if ingressHost != "" {
			hosts = []string{ingressHost}
		}
		if jobManagerIngressSpec.TLSSecretName != nil {
			secretName = *jobManagerIngressSpec.TLSSecretName
		}
		if hosts != nil || secretName != "" {
			ingressTLS = []extensionsv1beta1.IngressTLS{{
				Hosts:      hosts,
				SecretName: secretName,
			}}
		}
	}
	var jobManagerIngress = &extensionsv1beta1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: clusterNamespace,
			Name:      ingressName,
			OwnerReferences: []metav1.OwnerReference{
				toOwnerReference(flinkCluster)},
			Labels:      labels,
			Annotations: ingressAnnotations,
		},
		Spec: extensionsv1beta1.IngressSpec{
			TLS: ingressTLS,
			Rules: []extensionsv1beta1.IngressRule{{
				Host: ingressHost,
				IngressRuleValue: extensionsv1beta1.IngressRuleValue{
					HTTP: &extensionsv1beta1.HTTPIngressRuleValue{
						Paths: []extensionsv1beta1.HTTPIngressPath{{
							Path: "/",
							Backend: extensionsv1beta1.IngressBackend{
								ServiceName: jobManagerServiceName,
								ServicePort: jobManagerServiceUIPort,
							},
						}},
					},
				},
			}},
		},
	}

	return jobManagerIngress
}

// Gets the desired TaskManager deployment spec from a cluster spec.
func getDesiredTaskManagerDeployment(
	flinkCluster *v1alpha1.FlinkCluster,
	now time.Time) *appsv1.Deployment {

	if flinkCluster.Status.State == v1alpha1.ClusterState.Stopped {
		return nil
	}

	if isStopDelayExpired(flinkCluster.Status, delayDeleteClusterMinutes, now) {
		return nil
	}

	var clusterNamespace = flinkCluster.ObjectMeta.Namespace
	var clusterName = flinkCluster.ObjectMeta.Name
	var imageSpec = flinkCluster.Spec.Image
	var taskManagerSpec = flinkCluster.Spec.TaskManager
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
	var envVars = []corev1.EnvVar{
		{
			Name:  "JOB_MANAGER_RPC_ADDRESS",
			Value: jobManagerDeploymentName,
		},
		{
			Name: "TASK_MANAGER_CPU_LIMIT",
			ValueFrom: &corev1.EnvVarSource{
				ResourceFieldRef: &corev1.ResourceFieldSelector{
					ContainerName: "taskmanager",
					Resource:      "limits.cpu",
					Divisor:       resource.MustParse("1m"),
				},
			},
		},
		{
			Name: "TASK_MANAGER_MEMORY_LIMIT",
			ValueFrom: &corev1.EnvVarSource{
				ResourceFieldRef: &corev1.ResourceFieldSelector{
					ContainerName: "taskmanager",
					Resource:      "limits.memory",
					Divisor:       resource.MustParse("1Mi"),
				},
			},
		},
		{
			Name:  "FLINK_PROPERTIES",
			Value: getFlinkProperties(flinkCluster.Spec.FlinkProperties),
		},
	}
	envVars = append(envVars, flinkCluster.Spec.EnvVars...)
	var containers = []corev1.Container{corev1.Container{
		Name:            "taskmanager",
		Image:           imageSpec.Name,
		ImagePullPolicy: imageSpec.PullPolicy,
		Args:            []string{"taskmanager"},
		Ports: []corev1.ContainerPort{
			dataPort, rpcPort, queryPort},
		Resources:    taskManagerSpec.Resources,
		Env:          envVars,
		VolumeMounts: taskManagerSpec.Mounts,
	}}
	containers = append(containers, taskManagerSpec.Sidecars...)
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
					Containers:       containers,
					Volumes:          taskManagerSpec.Volumes,
					NodeSelector:     taskManagerSpec.NodeSelector,
					ImagePullSecrets: imageSpec.PullSecrets,
				},
			},
		},
	}
	return taskManagerDeployment
}

// Gets the desired job spec from a cluster spec.
func getDesiredJob(
	flinkCluster *v1alpha1.FlinkCluster) *batchv1.Job {
	var jobSpec = flinkCluster.Spec.Job
	if jobSpec == nil {
		return nil
	}

	var imageSpec = flinkCluster.Spec.Image
	var jobManagerSpec = flinkCluster.Spec.JobManager
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

	var envVars = []corev1.EnvVar{}
	envVars = append(envVars, flinkCluster.Spec.EnvVars...)

	// If the JAR file is remote, put the URI in the env variable
	// FLINK_JOB_JAR_URI and rewrite the JAR path to a local path. The entrypoint
	// script of the container will download it before submitting it to Flink.
	var jarPath = jobSpec.JarFile
	if strings.Contains(jobSpec.JarFile, "://") {
		var parts = strings.Split(jobSpec.JarFile, "/")
		jarPath = "/opt/flink/job/" + parts[len(parts)-1]
		envVars = append(envVars, corev1.EnvVar{
			Name:  "FLINK_JOB_JAR_URI",
			Value: jobSpec.JarFile,
		})
	}
	jobArgs = append(jobArgs, jarPath)

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
							ImagePullPolicy: imageSpec.PullPolicy,
							Args:            jobArgs,
							Env:             envVars,
							VolumeMounts:    jobSpec.Mounts,
						},
					},
					RestartPolicy:    *jobSpec.RestartPolicy,
					Volumes:          jobSpec.Volumes,
					ImagePullSecrets: imageSpec.PullSecrets,
				},
			},
		},
	}
	return job
}

// Converts the FlinkCluster as owner reference for its child resources.
func toOwnerReference(
	flinkCluster *v1alpha1.FlinkCluster) metav1.OwnerReference {
	return metav1.OwnerReference{
		APIVersion:         flinkCluster.APIVersion,
		Kind:               flinkCluster.Kind,
		Name:               flinkCluster.Name,
		UID:                flinkCluster.UID,
		Controller:         &[]bool{true}[0],
		BlockOwnerDeletion: &[]bool{false}[0],
	}
}

// Gets Flink properties
func getFlinkProperties(properties map[string]string) string {
	var builder strings.Builder
	for key, value := range properties {
		builder.WriteString(fmt.Sprintf("%s: %s\n", key, value))
	}
	return builder.String()
}

var jobManagerIngressHostRegex = regexp.MustCompile("{{\\s*[$]clusterName\\s*}}")

func getJobManagerIngressHost(ingressHostFormat string, clusterName string) string {
	// TODO: Validating webhook should verify hostFormat
	return jobManagerIngressHostRegex.ReplaceAllString(ingressHostFormat, clusterName)
}

func isStopDelayExpired(
	clusterStatus v1alpha1.FlinkClusterStatus,
	delayMinutes int32,
	now time.Time) bool {
	if clusterStatus.State != v1alpha1.ClusterState.Stopping ||
		len(clusterStatus.LastUpdateTime) == 0 {
		return false
	}
	var tc = &TimeConverter{}
	var lastUpdateTime = tc.FromString(clusterStatus.LastUpdateTime)
	return now.After(
		lastUpdateTime.Add(time.Duration(delayMinutes) * time.Minute))
}
