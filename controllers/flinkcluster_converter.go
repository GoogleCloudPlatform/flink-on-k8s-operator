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
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/api/resource"

	v1beta1 "github.com/googlecloudplatform/flink-operator/api/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// Converter which converts the FlinkCluster spec to the desired
// underlying Kubernetes resource specs.

const (
	delayDeleteClusterMinutes int32 = 5
	flinkConfigMapPath              = "/opt/flink/conf"
	flinkConfigMapVolume            = "flink-config-volume"
	gcpServiceAccountVolume         = "gcp-service-account-volume"
	hadoopConfigVolume              = "hadoop-config-volume"
)

var flinkSysProps = map[string]struct{}{
	"jobmanager.rpc.address": {},
	"jobmanager.rpc.port":    {},
	"blob.server.port":       {},
	"query.server.port":      {},
	"rest.port":              {},
}

// DesiredClusterState holds desired state of a cluster.
type DesiredClusterState struct {
	JmDeployment            *appsv1.Deployment
	JmService               *corev1.Service
	JmIngress               *extensionsv1beta1.Ingress
	TmDeployment            *appsv1.Deployment
	ConfigMap               *corev1.ConfigMap
	Job                     *batchv1.Job
	NativeClusterSessionJob *batchv1.Job
}

// Gets the desired state of a cluster.
func getDesiredClusterState(
	cluster *v1beta1.FlinkCluster,
	now time.Time) DesiredClusterState {
	// The cluster has been deleted, all resources should be cleaned up.
	if cluster == nil {
		return DesiredClusterState{}
	}
	return DesiredClusterState{
		ConfigMap:               getDesiredConfigMap(cluster),
		JmDeployment:            getDesiredJobManagerDeployment(cluster),
		JmService:               getDesiredJobManagerService(cluster),
		JmIngress:               getDesiredJobManagerIngress(cluster),
		TmDeployment:            getDesiredTaskManagerDeployment(cluster),
		Job:                     getDesiredJob(cluster),
		NativeClusterSessionJob: getDesiredNativeClusterSessionJob(cluster),
	}
}

// Gets the desired JobManager deployment spec from the FlinkCluster spec.
func getDesiredJobManagerDeployment(
	flinkCluster *v1beta1.FlinkCluster) *appsv1.Deployment {

	if shouldCleanup(flinkCluster, "JobManagerDeployment") {
		return nil
	}

	if flinkCluster.Spec.NativeSessionClusterJob != nil {
		//It's a native flink session cluster
		return nil
	}

	var clusterNamespace = flinkCluster.ObjectMeta.Namespace
	var clusterName = flinkCluster.ObjectMeta.Name
	var clusterSpec = flinkCluster.Spec
	var imageSpec = clusterSpec.Image
	var jobManagerSpec = clusterSpec.JobManager
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
	// Make Volume, VolumeMount to use configMap data for flink-conf.yaml, if flinkProperties is provided.
	var volumes []corev1.Volume
	var volumeMounts []corev1.VolumeMount
	var confVol *corev1.Volume
	var confMount *corev1.VolumeMount
	confVol, confMount = convertFlinkConfig(clusterName)
	volumes = append(jobManagerSpec.Volumes, *confVol)
	volumeMounts = append(jobManagerSpec.VolumeMounts, *confMount)
	var envVars = []corev1.EnvVar{
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
	}
	var readinessProbe = corev1.Probe{
		Handler: corev1.Handler{
			TCPSocket: &corev1.TCPSocketAction{
				Port: intstr.FromInt(int(*jobManagerSpec.Ports.RPC)),
			},
		},
		TimeoutSeconds:      10,
		InitialDelaySeconds: 5,
		PeriodSeconds:       5,
		FailureThreshold:    60,
	}
	var livenessProbe = corev1.Probe{
		Handler: corev1.Handler{
			TCPSocket: &corev1.TCPSocketAction{
				Port: intstr.FromInt(int(*jobManagerSpec.Ports.RPC)),
			},
		},
		TimeoutSeconds:      10,
		InitialDelaySeconds: 5,
		PeriodSeconds:       60,
		FailureThreshold:    5,
	}

	// Hadoop config.
	var hcVolume, hcMount, hcEnv = convertHadoopConfig(clusterSpec.HadoopConfig)
	if hcVolume != nil {
		volumes = append(volumes, *hcVolume)
	}
	if hcMount != nil {
		volumeMounts = append(volumeMounts, *hcMount)
	}
	if hcEnv != nil {
		envVars = append(envVars, *hcEnv)
	}

	// GCP service account config.
	var saVolume, saMount, saEnv = convertGCPConfig(clusterSpec.GCPConfig)
	if saVolume != nil {
		volumes = append(volumes, *saVolume)
	}
	if saMount != nil {
		volumeMounts = append(volumeMounts, *saMount)
	}
	if saEnv != nil {
		envVars = append(envVars, *saEnv)
	}

	envVars = append(envVars, flinkCluster.Spec.EnvVars...)
	var containers = []corev1.Container{corev1.Container{
		Name:            "jobmanager",
		Image:           imageSpec.Name,
		ImagePullPolicy: imageSpec.PullPolicy,
		Args:            []string{"jobmanager"},
		Ports: []corev1.ContainerPort{
			rpcPort, blobPort, queryPort, uiPort},
		LivenessProbe:  &livenessProbe,
		ReadinessProbe: &readinessProbe,
		Resources:      jobManagerSpec.Resources,
		Env:            envVars,
		VolumeMounts:   volumeMounts,
	}}

	containers = append(containers, jobManagerSpec.Sidecars...)

	var podSpec = corev1.PodSpec{
		Containers:       containers,
		Volumes:          volumes,
		NodeSelector:     jobManagerSpec.NodeSelector,
		ImagePullSecrets: imageSpec.PullSecrets,
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
				Spec: podSpec,
			},
		},
	}
	return jobManagerDeployment
}

// Gets the desired JobManager service spec from a cluster spec.
func getDesiredJobManagerService(
	flinkCluster *v1beta1.FlinkCluster) *corev1.Service {

	if shouldCleanup(flinkCluster, "JobManagerService") {
		return nil
	}

	if flinkCluster.Spec.NativeSessionClusterJob != nil {
		//It's a native flink session cluster
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
	case v1beta1.AccessScopeCluster:
		jobManagerService.Spec.Type = corev1.ServiceTypeClusterIP
	case v1beta1.AccessScopeVPC:
		jobManagerService.Spec.Type = corev1.ServiceTypeLoadBalancer
		jobManagerService.Annotations =
			map[string]string{"cloud.google.com/load-balancer-type": "Internal"}
	case v1beta1.AccessScopeExternal:
		jobManagerService.Spec.Type = corev1.ServiceTypeLoadBalancer
	case v1beta1.AccessScopeNodePort:
		jobManagerService.Spec.Type = corev1.ServiceTypeNodePort
	default:
		panic(fmt.Sprintf(
			"Unknown service access cope: %v", jobManagerSpec.AccessScope))
	}
	return jobManagerService
}

// Gets the desired JobManager ingress spec from a cluster spec.
func getDesiredJobManagerIngress(
	flinkCluster *v1beta1.FlinkCluster) *extensionsv1beta1.Ingress {
	var jobManagerIngressSpec = flinkCluster.Spec.JobManager.Ingress
	if jobManagerIngressSpec == nil {
		return nil
	}

	if shouldCleanup(flinkCluster, "JobManagerIngress") {
		return nil
	}

	if flinkCluster.Spec.NativeSessionClusterJob != nil {
		//It's a native flink session cluster
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
	flinkCluster *v1beta1.FlinkCluster) *appsv1.Deployment {

	if shouldCleanup(flinkCluster, "TaskManagerDeployment") {
		return nil
	}

	if flinkCluster.Spec.NativeSessionClusterJob != nil {
		//It's a native flink session cluster
		return nil
	}

	var clusterNamespace = flinkCluster.ObjectMeta.Namespace
	var clusterName = flinkCluster.ObjectMeta.Name
	var clusterSpec = flinkCluster.Spec
	var imageSpec = flinkCluster.Spec.Image
	var taskManagerSpec = flinkCluster.Spec.TaskManager
	var dataPort = corev1.ContainerPort{Name: "data", ContainerPort: *taskManagerSpec.Ports.Data}
	var rpcPort = corev1.ContainerPort{Name: "rpc", ContainerPort: *taskManagerSpec.Ports.RPC}
	var queryPort = corev1.ContainerPort{Name: "query", ContainerPort: *taskManagerSpec.Ports.Query}
	var taskManagerDeploymentName = getTaskManagerDeploymentName(clusterName)
	var labels = map[string]string{
		"cluster":   clusterName,
		"app":       "flink",
		"component": "taskmanager",
	}
	// Make Volume, VolumeMount to use configMap data for flink-conf.yaml
	var volumes []corev1.Volume
	var volumeMounts []corev1.VolumeMount

	// Flink config.
	var confVol, confMount = convertFlinkConfig(clusterName)
	volumes = append(taskManagerSpec.Volumes, *confVol)
	volumeMounts = append(taskManagerSpec.VolumeMounts, *confMount)

	var envVars = []corev1.EnvVar{
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
	}
	var readinessProbe = corev1.Probe{
		Handler: corev1.Handler{
			TCPSocket: &corev1.TCPSocketAction{
				Port: intstr.FromInt(int(*taskManagerSpec.Ports.RPC)),
			},
		},
		TimeoutSeconds:      10,
		InitialDelaySeconds: 5,
		PeriodSeconds:       5,
		FailureThreshold:    60,
	}
	var livenessProbe = corev1.Probe{
		Handler: corev1.Handler{
			TCPSocket: &corev1.TCPSocketAction{
				Port: intstr.FromInt(int(*taskManagerSpec.Ports.RPC)),
			},
		},
		TimeoutSeconds:      10,
		InitialDelaySeconds: 5,
		PeriodSeconds:       60,
		FailureThreshold:    5,
	}

	// Hadoop config.
	var hcVolume, hcMount, hcEnv = convertHadoopConfig(clusterSpec.HadoopConfig)
	if hcVolume != nil {
		volumes = append(volumes, *hcVolume)
	}
	if hcMount != nil {
		volumeMounts = append(volumeMounts, *hcMount)
	}
	if hcEnv != nil {
		envVars = append(envVars, *hcEnv)
	}

	// GCP service account config.
	var saVolume, saMount, saEnv = convertGCPConfig(clusterSpec.GCPConfig)
	if saVolume != nil {
		volumes = append(volumes, *saVolume)
	}
	if saMount != nil {
		volumeMounts = append(volumeMounts, *saMount)
	}
	if saEnv != nil {
		envVars = append(envVars, *saEnv)
	}
	envVars = append(envVars, flinkCluster.Spec.EnvVars...)

	var containers = []corev1.Container{corev1.Container{
		Name:            "taskmanager",
		Image:           imageSpec.Name,
		ImagePullPolicy: imageSpec.PullPolicy,
		Args:            []string{"taskmanager"},
		Ports: []corev1.ContainerPort{
			dataPort, rpcPort, queryPort},
		LivenessProbe:  &livenessProbe,
		ReadinessProbe: &readinessProbe,
		Resources:      taskManagerSpec.Resources,
		Env:            envVars,
		VolumeMounts:   volumeMounts,
	}}
	containers = append(containers, taskManagerSpec.Sidecars...)
	var podSpec = corev1.PodSpec{
		Containers:       containers,
		Volumes:          volumes,
		NodeSelector:     taskManagerSpec.NodeSelector,
		ImagePullSecrets: imageSpec.PullSecrets,
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
				Spec: podSpec,
			},
		},
	}
	return taskManagerDeployment
}

// Gets the desired configMap.
func getDesiredConfigMap(
	flinkCluster *v1beta1.FlinkCluster) *corev1.ConfigMap {

	if shouldCleanup(flinkCluster, "ConfigMap") {
		return nil
	}

	if flinkCluster.Spec.NativeSessionClusterJob != nil {
		//It's a native flink session cluster
		return nil
	}

	var clusterNamespace = flinkCluster.ObjectMeta.Namespace
	var clusterName = flinkCluster.ObjectMeta.Name
	var flinkProperties = flinkCluster.Spec.FlinkProperties
	var jmPorts = flinkCluster.Spec.JobManager.Ports
	var tmPorts = flinkCluster.Spec.TaskManager.Ports
	var configMapName = getConfigMapName(clusterName)
	var labels = map[string]string{
		"cluster": clusterName,
		"app":     "flink",
	}
	var flinkHeapSize = calFlinkHeapSize(flinkCluster)
	// Properties which should be provided from real deployed environment.
	var flinkProps = map[string]string{
		"jobmanager.rpc.address": getJobManagerServiceName(clusterName),
		"jobmanager.rpc.port":    strconv.FormatInt(int64(*jmPorts.RPC), 10),
		"blob.server.port":       strconv.FormatInt(int64(*jmPorts.Blob), 10),
		"query.server.port":      strconv.FormatInt(int64(*jmPorts.Query), 10),
		"rest.port":              strconv.FormatInt(int64(*jmPorts.UI), 10),
		"taskmanager.rpc.port":   strconv.FormatInt(int64(*tmPorts.RPC), 10),
	}
	if flinkHeapSize["jobmanager.heap.size"] != "" {
		flinkProps["jobmanager.heap.size"] = flinkHeapSize["jobmanager.heap.size"]
	}
	if flinkHeapSize["taskmanager.heap.size"] != "" {
		flinkProps["taskmanager.heap.size"] = flinkHeapSize["taskmanager.heap.size"]
	}
	// Add custom Flink properties.
	for k, v := range flinkProperties {
		// Do not allow to override properties from real deployment.
		if _, ok := flinkSysProps[k]; ok {
			continue
		}
		flinkProps[k] = v
	}
	// TODO: Provide logging options: log4j-console.properties and log4j.properties
	var log4jPropName = "log4j-console.properties"
	var logbackXMLName = "logback-console.xml"
	var configMap = &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: clusterNamespace,
			Name:      configMapName,
			OwnerReferences: []metav1.OwnerReference{
				toOwnerReference(flinkCluster)},
			Labels: labels,
		},
		Data: map[string]string{
			"flink-conf.yaml": getFlinkProperties(flinkProps),
			log4jPropName:     getLogConf()[log4jPropName],
			logbackXMLName:    getLogConf()[logbackXMLName],
			"submit-job.sh":   submitJobScript,
		},
	}

	return configMap
}

// Gets the desired job spec from a cluster spec.
func getDesiredJob(
	flinkCluster *v1beta1.FlinkCluster) *batchv1.Job {
	var jobSpec = flinkCluster.Spec.Job

	if jobSpec == nil {
		return nil
	}

	if flinkCluster.Spec.NativeSessionClusterJob != nil {
		//It's a native flink session cluster
		return nil
	}

	var controlStatus = flinkCluster.Status.Control
	// We need to watch whether job is cancelled already if jobSpec.CancelRequested is deprecated
	if (flinkCluster.Status.Components.Job != nil && flinkCluster.Status.Components.Job.State == v1beta1.JobStateCancelled) ||
		(jobSpec.CancelRequested != nil && *jobSpec.CancelRequested) ||
		(controlStatus != nil && controlStatus.Name == v1beta1.ControlNameCancel && controlStatus.State == v1beta1.ControlStateProgressing) {
		return nil
	}

	var clusterSpec = flinkCluster.Spec
	var imageSpec = clusterSpec.Image
	var jobManagerSpec = clusterSpec.JobManager
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
	var jobArgs = []string{"bash", "/opt/flink-operator/submit-job.sh"}
	jobArgs = append(jobArgs, "--jobmanager", jobManagerAddress)
	if jobSpec.ClassName != nil {
		jobArgs = append(jobArgs, "--class", *jobSpec.ClassName)
	}

	var jobStatus = flinkCluster.Status.Components.Job
	var fromSavepoint = convertFromSavepoint(jobSpec, jobStatus)
	if fromSavepoint != nil {
		jobArgs = append(jobArgs, "--fromSavepoint", *fromSavepoint)
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
	jobArgs = append(jobArgs, "--detached")

	var envVars = []corev1.EnvVar{}

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

	var volumes []corev1.Volume
	var volumeMounts []corev1.VolumeMount
	volumes = append(volumes, jobSpec.Volumes...)
	volumeMounts = append(volumeMounts, jobSpec.VolumeMounts...)

	// Submit job script config.
	var sbsVolume *corev1.Volume
	var sbsMount *corev1.VolumeMount
	sbsVolume, sbsMount = convertSubmitJobScript(clusterName)
	volumes = append(volumes, *sbsVolume)
	volumeMounts = append(volumeMounts, *sbsMount)

	// Hadoop config.
	var hcVolume, hcMount, hcEnv = convertHadoopConfig(clusterSpec.HadoopConfig)
	if hcVolume != nil {
		volumes = append(volumes, *hcVolume)
	}
	if hcMount != nil {
		volumeMounts = append(volumeMounts, *hcMount)
	}
	if hcEnv != nil {
		envVars = append(envVars, *hcEnv)
	}

	// GCP service account config.
	var saVolume, saMount, saEnv = convertGCPConfig(clusterSpec.GCPConfig)
	if saVolume != nil {
		volumes = append(volumes, *saVolume)
	}
	if saMount != nil {
		volumeMounts = append(volumeMounts, *saMount)
	}
	if saEnv != nil {
		envVars = append(envVars, *saEnv)
	}

	envVars = append(envVars, flinkCluster.Spec.EnvVars...)

	var podSpec = corev1.PodSpec{
		InitContainers: convertJobInitContainers(jobSpec),
		Containers: []corev1.Container{
			corev1.Container{
				Name:            "main",
				Image:           imageSpec.Name,
				ImagePullPolicy: imageSpec.PullPolicy,
				Args:            jobArgs,
				Env:             envVars,
				VolumeMounts:    volumeMounts,
			},
		},
		RestartPolicy:    corev1.RestartPolicyNever,
		Volumes:          volumes,
		ImagePullSecrets: imageSpec.PullSecrets,
	}

	// Disable the retry mechanism of k8s Job, all retires should be initiated
	// by the operator based on the job restart policy. This is because Flink
	// jobs are stateful, if a job fails after running for 10 hours, we probably
	// don't want to start over from the beginning, instead we want to resume
	// the job from the latest savepoint which means strictly speaking it is no
	// longer the same job as the previous one because the `--fromSavepoint`
	// parameter has changed.
	var backoffLimit int32 = 0
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
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec:       podSpec,
			},
			BackoffLimit: &backoffLimit,
		},
	}
	return job
}

func getDesiredNativeClusterSessionJob(
	flinkCluster *v1beta1.FlinkCluster) *batchv1.Job {
	var jobSpec = flinkCluster.Spec.NativeSessionClusterJob

	if jobSpec == nil {
		return nil
	}
	var clusterSpec = flinkCluster.Spec
	var imageSpec = clusterSpec.Image
	var clusterNamespace = flinkCluster.ObjectMeta.Namespace
	var clusterName = flinkCluster.ObjectMeta.Name
	var jobName = getJobName(clusterName)
	var labels = map[string]string{
		"cluster": clusterName,
		"app":     "flinkNativeSessionCluster",
	}

	var jobArgs = []string{"/opt/flink/bin/kubernetes-session.sh"}

	jobArgs = append(jobArgs, "-Dkubernetes.cluster-id="+clusterName)

	if jobSpec.EntryPath != nil {
		jobArgs = append(jobArgs, *jobSpec.EntryPath)
	}

	if jobSpec.CongfigDir != nil {
		jobArgs = append(jobArgs, *jobSpec.CongfigDir)
	}

	if jobSpec.FlinkClusterSA != nil {
		jobArgs = append(jobArgs, *jobSpec.FlinkClusterSA)
	}
	//TODO: check all properties and append to jobArgs.

	var envVars = []corev1.EnvVar{}
	var volumes []corev1.Volume
	var volumeMounts []corev1.VolumeMount

	// Hadoop config.
	var hcVolume, hcMount, hcEnv = convertHadoopConfig(clusterSpec.HadoopConfig)
	if hcVolume != nil {
		volumes = append(volumes, *hcVolume)
	}
	if hcMount != nil {
		volumeMounts = append(volumeMounts, *hcMount)
	}
	if hcEnv != nil {
		envVars = append(envVars, *hcEnv)
	}

	// GCP service account config.
	var saVolume, saMount, saEnv = convertGCPConfig(clusterSpec.GCPConfig)
	if saVolume != nil {
		volumes = append(volumes, *saVolume)
	}
	if saMount != nil {
		volumeMounts = append(volumeMounts, *saMount)
	}
	if saEnv != nil {
		envVars = append(envVars, *saEnv)
	}

	envVars = append(envVars, flinkCluster.Spec.EnvVars...)

	var podSpec = corev1.PodSpec{
		Containers: []corev1.Container{
			corev1.Container{
				Name:            "main",
				Image:           imageSpec.Name,
				ImagePullPolicy: imageSpec.PullPolicy,
				Args:            jobArgs,
				Env:             envVars,
				VolumeMounts:    volumeMounts,
			},
		},
		RestartPolicy:    corev1.RestartPolicyNever,
		Volumes:          volumes,
		ImagePullSecrets: imageSpec.PullSecrets,
	}

	// Disable the retry mechanism of k8s Job, all retires should be initiated
	// by the operator based on the job restart policy. This is because Flink
	// jobs are stateful, if a job fails after running for 10 hours, we probably
	// don't want to start over from the beginning, instead we want to resume
	// the job from the latest savepoint which means strictly speaking it is no
	// longer the same job as the previous one because the `--fromSavepoint`
	// parameter has changed.
	var backoffLimit int32 = 0
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
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec:       podSpec,
			},
			BackoffLimit: &backoffLimit,
		},
	}
	return job
}

func convertFromSavepoint(
	jobSpec *v1beta1.JobSpec, jobStatus *v1beta1.JobStatus) *string {
	if shouldRestartJob(jobSpec.RestartPolicy, jobStatus) {
		return &jobStatus.SavepointLocation
	}
	return jobSpec.FromSavepoint
}

func convertJobInitContainers(jobSpec *v1beta1.JobSpec) []corev1.Container {
	var initContainers = []corev1.Container{}
	// Add jobSpec level volume mounts to each init container if there is no
	// conflict.
	for _, initContainer := range jobSpec.InitContainers {
		for _, jobMount := range jobSpec.VolumeMounts {
			var conflit = false
			for _, mount := range initContainer.VolumeMounts {
				if jobMount.MountPath == mount.MountPath {
					conflit = true
					break
				}
			}
			if !conflit {
				initContainer.VolumeMounts =
					append(initContainer.VolumeMounts, jobMount)
			}
		}
		initContainers = append(initContainers, initContainer)
	}
	return initContainers
}

// Converts the FlinkCluster as owner reference for its child resources.
func toOwnerReference(
	flinkCluster *v1beta1.FlinkCluster) metav1.OwnerReference {
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
	var keys = make([]string, len(properties))
	i := 0
	for k := range properties {
		keys[i] = k
		i = i + 1
	}
	sort.Strings(keys)
	var builder strings.Builder
	for _, key := range keys {
		builder.WriteString(fmt.Sprintf("%s: %s\n", key, properties[key]))
	}
	return builder.String()
}

var jobManagerIngressHostRegex = regexp.MustCompile("{{\\s*[$]clusterName\\s*}}")

func getJobManagerIngressHost(ingressHostFormat string, clusterName string) string {
	// TODO: Validating webhook should verify hostFormat
	return jobManagerIngressHostRegex.ReplaceAllString(ingressHostFormat, clusterName)
}

// Checks whether the component should be deleted according to the cleanup
// policy. Always return false for session cluster.
func shouldCleanup(
	cluster *v1beta1.FlinkCluster, component string) bool {
	var jobStatus = cluster.Status.Components.Job

	// Session cluster.
	if jobStatus == nil {
		return false
	}

	var action v1beta1.CleanupAction
	switch jobStatus.State {
	case v1beta1.JobStateSucceeded:
		action = cluster.Spec.Job.CleanupPolicy.AfterJobSucceeds
	case v1beta1.JobStateFailed:
		action = cluster.Spec.Job.CleanupPolicy.AfterJobFails
	case v1beta1.JobStateCancelled:
		action = cluster.Spec.Job.CleanupPolicy.AfterJobCancelled
	default:
		return false
	}

	switch action {
	case v1beta1.CleanupActionDeleteCluster:
		return true
	case v1beta1.CleanupActionDeleteTaskManager:
		return component == "TaskManagerDeployment"
	}

	return false
}

func calFlinkHeapSize(cluster *v1beta1.FlinkCluster) map[string]string {
	if cluster.Spec.JobManager.MemoryOffHeapRatio == nil {
		return nil
	}
	var flinkHeapSize = make(map[string]string)
	var jmMemoryLimitByte = cluster.Spec.JobManager.Resources.Limits.Memory().Value()
	var tmMemLimitByte = cluster.Spec.TaskManager.Resources.Limits.Memory().Value()
	if jmMemoryLimitByte > 0 {
		jmMemoryOffHeapMinByte := cluster.Spec.JobManager.MemoryOffHeapMin.Value()
		jmMemoryOffHeapRatio := int64(*cluster.Spec.JobManager.MemoryOffHeapRatio)
		heapSizeMB := calHeapSize(
			jmMemoryLimitByte,
			jmMemoryOffHeapMinByte,
			jmMemoryOffHeapRatio)
		if heapSizeMB > 0 {
			flinkHeapSize["jobmanager.heap.size"] = strconv.FormatInt(heapSizeMB, 10) + "m"
		}
	}
	if tmMemLimitByte > 0 {
		tmMemoryOffHeapMinByte := cluster.Spec.TaskManager.MemoryOffHeapMin.Value()
		tmMemoryOffHeapRatio := int64(*cluster.Spec.TaskManager.MemoryOffHeapRatio)
		heapSizeMB := calHeapSize(
			tmMemLimitByte,
			tmMemoryOffHeapMinByte,
			tmMemoryOffHeapRatio)
		if heapSizeMB > 0 {
			flinkHeapSize["taskmanager.heap.size"] = strconv.FormatInt(heapSizeMB, 10) + "m"
		}
	}
	return flinkHeapSize
}

// Converts memory value to the format of divisor and returns ceiling of the value.
func convertResourceMemoryToInt64(memory resource.Quantity, divisor resource.Quantity) int64 {
	return int64(math.Ceil(float64(memory.Value()) / float64(divisor.Value())))
}

// Calculate heap size in MB
func calHeapSize(memSize int64, offHeapMin int64, offHeapRatio int64) int64 {
	var heapSizeMB int64
	offHeapSize := int64(math.Ceil(float64(memSize*offHeapRatio) / 100))
	if offHeapSize < offHeapMin {
		offHeapSize = offHeapMin
	}
	heapSizeCalculated := memSize - offHeapSize
	if heapSizeCalculated > 0 {
		divisor := resource.MustParse("1M")
		heapSizeQuantity := resource.NewQuantity(heapSizeCalculated, resource.DecimalSI)
		heapSizeMB = convertResourceMemoryToInt64(*heapSizeQuantity, divisor)
	}
	return heapSizeMB
}

func convertFlinkConfig(clusterName string) (*corev1.Volume, *corev1.VolumeMount) {
	var confVol *corev1.Volume
	var confMount *corev1.VolumeMount
	confVol = &corev1.Volume{
		Name: flinkConfigMapVolume,
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: getConfigMapName(clusterName),
				},
			},
		},
	}
	confMount = &corev1.VolumeMount{
		Name:      flinkConfigMapVolume,
		MountPath: flinkConfigMapPath,
	}
	return confVol, confMount
}

func convertSubmitJobScript(clusterName string) (*corev1.Volume, *corev1.VolumeMount) {
	var confVol *corev1.Volume
	var confMount *corev1.VolumeMount
	confVol = &corev1.Volume{
		Name: flinkConfigMapVolume,
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: getConfigMapName(clusterName),
				},
			},
		},
	}
	confMount = &corev1.VolumeMount{
		Name:      flinkConfigMapVolume,
		MountPath: "/opt/flink-operator/submit-job.sh",
		SubPath:   "submit-job.sh",
	}
	return confVol, confMount
}

func convertHadoopConfig(hadoopConfig *v1beta1.HadoopConfig) (
	*corev1.Volume, *corev1.VolumeMount, *corev1.EnvVar) {
	if hadoopConfig == nil {
		return nil, nil, nil
	}

	var volume = &corev1.Volume{
		Name: hadoopConfigVolume,
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: hadoopConfig.ConfigMapName,
				},
			},
		},
	}
	var mount = &corev1.VolumeMount{
		Name:      hadoopConfigVolume,
		MountPath: hadoopConfig.MountPath,
		ReadOnly:  true,
	}
	var env = &corev1.EnvVar{
		Name:  "HADOOP_CONF_DIR",
		Value: hadoopConfig.MountPath,
	}
	return volume, mount, env
}

func convertGCPConfig(gcpConfig *v1beta1.GCPConfig) (*corev1.Volume, *corev1.VolumeMount, *corev1.EnvVar) {
	if gcpConfig == nil {
		return nil, nil, nil
	}

	var saConfig = gcpConfig.ServiceAccount
	var saVolume = &corev1.Volume{
		Name: gcpServiceAccountVolume,
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: gcpConfig.ServiceAccount.SecretName,
			},
		},
	}
	var saMount = &corev1.VolumeMount{
		Name:      gcpServiceAccountVolume,
		MountPath: gcpConfig.ServiceAccount.MountPath,
		ReadOnly:  true,
	}
	if !strings.HasSuffix(saMount.MountPath, "/") {
		saMount.MountPath = saMount.MountPath + "/"
	}
	var saEnv = &corev1.EnvVar{
		Name:  "GOOGLE_APPLICATION_CREDENTIALS",
		Value: saMount.MountPath + saConfig.KeyFile,
	}
	return saVolume, saMount, saEnv
}

// TODO: Wouldn't it be better to create a file, put it in an operator image, and read from them?.
// Provide logging profiles
func getLogConf() map[string]string {
	var log4jConsoleProperties = `log4j.rootLogger=INFO, console
log4j.logger.akka=INFO
log4j.logger.org.apache.kafka=INFO
log4j.logger.org.apache.hadoop=INFO
log4j.logger.org.apache.zookeeper=INFO
log4j.appender.console=org.apache.log4j.ConsoleAppender
log4j.appender.console.layout=org.apache.log4j.PatternLayout
log4j.appender.console.layout.ConversionPattern=%d{yyyy-MM-dd HH:mm:ss,SSS} %-5p %-60c %x - %m%n
log4j.logger.org.apache.flink.shaded.akka.org.jboss.netty.channel.DefaultChannelPipeline=ERROR, console`
	var logbackConsoleXML = `<configuration>
    <appender name="console" class="ch.qos.logback.core.ConsoleAppender">
        <encoder>
            <pattern>%d{yyyy-MM-dd HH:mm:ss.SSS} [%thread] %-5level %logger{60} %X{sourceThread} - %msg%n</pattern>
        </encoder>
    </appender>
    <root level="INFO">
        <appender-ref ref="console"/>
    </root>
    <logger name="akka" level="INFO">
        <appender-ref ref="console"/>
    </logger>
    <logger name="org.apache.kafka" level="INFO">
        <appender-ref ref="console"/>
    </logger>
    <logger name="org.apache.hadoop" level="INFO">
        <appender-ref ref="console"/>
    </logger>
    <logger name="org.apache.zookeeper" level="INFO">
        <appender-ref ref="console"/>
    </logger>
    <logger name="org.apache.flink.shaded.akka.org.jboss.netty.channel.DefaultChannelPipeline" level="ERROR">
        <appender-ref ref="console"/>
    </logger>
</configuration>`

	return map[string]string{
		"log4j-console.properties": log4jConsoleProperties,
		"logback-console.xml":      logbackConsoleXML,
	}
}
