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
	"testing"
	"time"

	"github.com/google/go-cmp/cmp/cmpopts"
	v1alpha1 "github.com/googlecloudplatform/flink-operator/api/v1alpha1"
	"gotest.tools/assert"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func TestGetDesiredClusterState(t *testing.T) {
	var controller = true
	var blockOwnerDeletion = false
	var parallelism int32 = 2
	var jmRPCPort int32 = 6123
	var jmBlobPort int32 = 6124
	var jmQueryPort int32 = 6125
	var jmUIPort int32 = 8081
	var useTLS bool = true
	var tmDataPort int32 = 6121
	var tmRPCPort int32 = 6122
	var tmQueryPort int32 = 6125
	var replicas int32 = 42
	var restartPolicy = v1alpha1.JobRestartPolicyFromSavepointOnFailure
	var className = "org.apache.flink.examples.java.wordcount.WordCount"
	var hostFormat = "{{$clusterName}}.example.com"
	var memoryOffHeapRatio int32 = 25
	var memoryOffHeapMin = resource.MustParse("600M")
	var jobBackoffLimit int32 = 0
	var jmProbe = corev1.Probe{
		Handler: corev1.Handler{
			TCPSocket: &corev1.TCPSocketAction{
				Port: intstr.FromInt(int(jmRPCPort)),
			},
		},
		TimeoutSeconds:      10,
		InitialDelaySeconds: 30,
		PeriodSeconds:       60,
		FailureThreshold:    5,
	}
	var tmProbe = corev1.Probe{
		Handler: corev1.Handler{
			TCPSocket: &corev1.TCPSocketAction{
				Port: intstr.FromInt(int(tmRPCPort)),
			},
		},
		TimeoutSeconds:      10,
		InitialDelaySeconds: 30,
		PeriodSeconds:       60,
		FailureThreshold:    5,
	}
	// Setup.
	var cluster = &v1alpha1.FlinkCluster{
		TypeMeta: metav1.TypeMeta{
			Kind:       "FlinkCluster",
			APIVersion: "flinkoperator.k8s.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "flinkjobcluster-sample",
			Namespace: "default",
		},
		Spec: v1alpha1.FlinkClusterSpec{
			Image: v1alpha1.ImageSpec{Name: "flink:1.8.1"},
			Job: &v1alpha1.JobSpec{
				Args:          []string{"--input", "./README.txt"},
				ClassName:     &className,
				JarFile:       "/cache/my-job.jar",
				Parallelism:   &parallelism,
				RestartPolicy: &restartPolicy,
				Volumes: []corev1.Volume{
					{
						Name: "cache-volume",
						VolumeSource: corev1.VolumeSource{
							EmptyDir: &corev1.EmptyDirVolumeSource{},
						},
					},
				},
				VolumeMounts: []corev1.VolumeMount{
					{Name: "cache-volume", MountPath: "/cache"},
				},
				InitContainers: []corev1.Container{
					{
						Name:    "gcs-downloader",
						Image:   "google/cloud-sdk",
						Command: []string{"gsutil"},
						Args: []string{
							"cp", "gs://my-bucket/my-job.jar", "/cache/my-job.jar",
						},
					},
				},
			},
			JobManager: v1alpha1.JobManagerSpec{
				AccessScope: v1alpha1.AccessScope.VPC,
				Ingress: &v1alpha1.JobManagerIngressSpec{
					HostFormat: &hostFormat,
					Annotations: map[string]string{
						"kubernetes.io/ingress.class":                "nginx",
						"certmanager.k8s.io/cluster-issuer":          "letsencrypt-stg",
						"nginx.ingress.kubernetes.io/rewrite-target": "/",
					},
					UseTLS: &useTLS,
				},
				Ports: v1alpha1.JobManagerPorts{
					RPC:   &jmRPCPort,
					Blob:  &jmBlobPort,
					Query: &jmQueryPort,
					UI:    &jmUIPort,
				},
				Resources: corev1.ResourceRequirements{
					Requests: map[corev1.ResourceName]resource.Quantity{
						corev1.ResourceCPU:    resource.MustParse("100m"),
						corev1.ResourceMemory: resource.MustParse("256Mi"),
					},
					Limits: map[corev1.ResourceName]resource.Quantity{
						corev1.ResourceCPU:    resource.MustParse("200m"),
						corev1.ResourceMemory: resource.MustParse("512Mi"),
					},
				},
				MemoryOffHeapRatio: &memoryOffHeapRatio,
				MemoryOffHeapMin:   memoryOffHeapMin,
			},
			TaskManager: v1alpha1.TaskManagerSpec{
				Replicas: 42,
				Ports: v1alpha1.TaskManagerPorts{
					Data:  &tmDataPort,
					RPC:   &tmRPCPort,
					Query: &tmQueryPort,
				},
				Resources: corev1.ResourceRequirements{
					Requests: map[corev1.ResourceName]resource.Quantity{
						corev1.ResourceCPU:    resource.MustParse("200m"),
						corev1.ResourceMemory: resource.MustParse("512Mi"),
					},
					Limits: map[corev1.ResourceName]resource.Quantity{
						corev1.ResourceCPU:    resource.MustParse("500m"),
						corev1.ResourceMemory: resource.MustParse("1Gi"),
					},
				},
				MemoryOffHeapRatio: &memoryOffHeapRatio,
				MemoryOffHeapMin:   memoryOffHeapMin,
				Sidecars:           []corev1.Container{{Name: "sidecar", Image: "alpine"}},
				Volumes: []corev1.Volume{
					{
						Name: "cache-volume",
						VolumeSource: corev1.VolumeSource{
							EmptyDir: &corev1.EmptyDirVolumeSource{},
						},
					},
				},
				VolumeMounts: []corev1.VolumeMount{
					{Name: "cache-volume", MountPath: "/cache"},
				},
			},
			FlinkProperties: map[string]string{"taskmanager.numberOfTaskSlots": "1"},
			EnvVars:         []corev1.EnvVar{{Name: "FOO", Value: "abc"}},
			HadoopConfig: &v1alpha1.HadoopConfig{
				ConfigMapName: "hadoop-configmap",
				MountPath:     "/etc/hadoop/conf",
			},
			GCPConfig: &v1alpha1.GCPConfig{
				ServiceAccount: &v1alpha1.GCPServiceAccount{
					SecretName: "gcp-service-account-secret",
					KeyFile:    "gcp_service_account_key.json",
					MountPath:  "/etc/gcp_service_account/",
				},
			},
		},
	}

	// Run.
	var desiredState = getDesiredClusterState(cluster, time.Now())

	// Verify.

	// JmDeployment
	var expectedDesiredJmDeployment = appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "flinkjobcluster-sample-jobmanager",
			Namespace: "default",
			Labels: map[string]string{
				"app":       "flink",
				"cluster":   "flinkjobcluster-sample",
				"component": "jobmanager",
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         "flinkoperator.k8s.io/v1alpha1",
					Kind:               "FlinkCluster",
					Name:               "flinkjobcluster-sample",
					Controller:         &controller,
					BlockOwnerDeletion: &blockOwnerDeletion,
				},
			},
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app":       "flink",
					"cluster":   "flinkjobcluster-sample",
					"component": "jobmanager",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app":       "flink",
						"cluster":   "flinkjobcluster-sample",
						"component": "jobmanager",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "jobmanager",
							Image: "flink:1.8.1",
							Args:  []string{"jobmanager"},
							Ports: []corev1.ContainerPort{
								{Name: "rpc", ContainerPort: jmRPCPort},
								{Name: "blob", ContainerPort: jmBlobPort},
								{Name: "query", ContainerPort: jmQueryPort},
								{Name: "ui", ContainerPort: jmUIPort},
							},
							LivenessProbe:  &jmProbe,
							ReadinessProbe: &jmProbe,
							Env: []corev1.EnvVar{
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
								{Name: "HADOOP_CONF_DIR", Value: "/etc/hadoop/conf"},
								{
									Name:  "GOOGLE_APPLICATION_CREDENTIALS",
									Value: "/etc/gcp_service_account/gcp_service_account_key.json",
								},
								{
									Name:  "FOO",
									Value: "abc",
								},
							},
							Resources: corev1.ResourceRequirements{
								Requests: map[corev1.ResourceName]resource.Quantity{
									corev1.ResourceCPU:    resource.MustParse("100m"),
									corev1.ResourceMemory: resource.MustParse("256Mi"),
								},
								Limits: map[corev1.ResourceName]resource.Quantity{
									corev1.ResourceCPU:    resource.MustParse("200m"),
									corev1.ResourceMemory: resource.MustParse("512Mi"),
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "flink-config-volume",
									MountPath: "/opt/flink/conf",
								},
								{
									Name:      "hadoop-config-volume",
									MountPath: "/etc/hadoop/conf",
									ReadOnly:  true,
								},
								{
									Name:      "gcp-service-account-volume",
									MountPath: "/etc/gcp_service_account/",
									ReadOnly:  true,
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "flink-config-volume",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "flinkjobcluster-sample-configmap",
									},
								},
							},
						},
						{
							Name: "hadoop-config-volume",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "hadoop-configmap",
									},
								},
							},
						},
						{
							Name: "gcp-service-account-volume",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: "gcp-service-account-secret",
								},
							},
						},
					},
				},
			},
		},
	}

	assert.Assert(t, desiredState.JmDeployment != nil)
	assert.DeepEqual(
		t,
		*desiredState.JmDeployment,
		expectedDesiredJmDeployment,
		cmpopts.IgnoreUnexported(resource.Quantity{}))

	// JmService
	var expectedDesiredJmService = corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "flinkjobcluster-sample-jobmanager",
			Namespace: "default",
			Labels: map[string]string{
				"app":       "flink",
				"cluster":   "flinkjobcluster-sample",
				"component": "jobmanager",
			},
			Annotations: map[string]string{
				"cloud.google.com/load-balancer-type": "Internal",
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         "flinkoperator.k8s.io/v1alpha1",
					Kind:               "FlinkCluster",
					Name:               "flinkjobcluster-sample",
					Controller:         &controller,
					BlockOwnerDeletion: &blockOwnerDeletion,
				},
			},
		},
		Spec: v1.ServiceSpec{
			Type: "LoadBalancer",
			Selector: map[string]string{
				"app":       "flink",
				"cluster":   "flinkjobcluster-sample",
				"component": "jobmanager",
			},
			Ports: []v1.ServicePort{
				{Name: "rpc", Port: 6123, TargetPort: intstr.FromString("rpc")},
				{Name: "blob", Port: 6124, TargetPort: intstr.FromString("blob")},
				{Name: "query", Port: 6125, TargetPort: intstr.FromString("query")},
				{Name: "ui", Port: 8081, TargetPort: intstr.FromString("ui")},
			},
		},
	}
	assert.Assert(t, desiredState.JmService != nil)
	assert.DeepEqual(
		t,
		*desiredState.JmService,
		expectedDesiredJmService)

	// JmIngress
	var expectedDesiredJmIngress = extensionsv1beta1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "flinkjobcluster-sample-jobmanager",
			Namespace: "default",
			Labels: map[string]string{
				"app":       "flink",
				"cluster":   "flinkjobcluster-sample",
				"component": "jobmanager",
			},
			Annotations: map[string]string{
				"kubernetes.io/ingress.class":                "nginx",
				"certmanager.k8s.io/cluster-issuer":          "letsencrypt-stg",
				"nginx.ingress.kubernetes.io/rewrite-target": "/",
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         "flinkoperator.k8s.io/v1alpha1",
					Kind:               "FlinkCluster",
					Name:               "flinkjobcluster-sample",
					Controller:         &controller,
					BlockOwnerDeletion: &blockOwnerDeletion,
				},
			},
		},
		Spec: extensionsv1beta1.IngressSpec{
			Rules: []extensionsv1beta1.IngressRule{{
				Host: "flinkjobcluster-sample.example.com",
				IngressRuleValue: extensionsv1beta1.IngressRuleValue{
					HTTP: &extensionsv1beta1.HTTPIngressRuleValue{
						Paths: []extensionsv1beta1.HTTPIngressPath{{
							Path: "/",
							Backend: extensionsv1beta1.IngressBackend{
								ServiceName: "flinkjobcluster-sample-jobmanager",
								ServicePort: intstr.FromString("ui"),
							}},
						}},
				},
			}},
			TLS: []extensionsv1beta1.IngressTLS{{
				Hosts: []string{"flinkjobcluster-sample.example.com"},
			}},
		},
	}

	assert.Assert(t, desiredState.JmIngress != nil)
	assert.DeepEqual(
		t,
		*desiredState.JmIngress,
		expectedDesiredJmIngress)

	// TmDeployment
	var expectedDesiredTmDeployment = appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "flinkjobcluster-sample-taskmanager",
			Namespace: "default",
			Labels: map[string]string{
				"app":       "flink",
				"cluster":   "flinkjobcluster-sample",
				"component": "taskmanager",
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         "flinkoperator.k8s.io/v1alpha1",
					Kind:               "FlinkCluster",
					Name:               "flinkjobcluster-sample",
					Controller:         &controller,
					BlockOwnerDeletion: &blockOwnerDeletion,
				},
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app":       "flink",
					"cluster":   "flinkjobcluster-sample",
					"component": "taskmanager",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app":       "flink",
						"cluster":   "flinkjobcluster-sample",
						"component": "taskmanager",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						corev1.Container{
							Name:  "taskmanager",
							Image: "flink:1.8.1",
							Args:  []string{"taskmanager"},
							Ports: []corev1.ContainerPort{
								{Name: "data", ContainerPort: 6121},
								{Name: "rpc", ContainerPort: 6122},
								{Name: "query", ContainerPort: 6125},
							},
							LivenessProbe:  &tmProbe,
							ReadinessProbe: &tmProbe,
							Env: []corev1.EnvVar{
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
								{Name: "HADOOP_CONF_DIR", Value: "/etc/hadoop/conf"},
								{
									Name:  "GOOGLE_APPLICATION_CREDENTIALS",
									Value: "/etc/gcp_service_account/gcp_service_account_key.json",
								},
								{
									Name:  "FOO",
									Value: "abc",
								},
							},
							Resources: corev1.ResourceRequirements{
								Requests: map[corev1.ResourceName]resource.Quantity{
									corev1.ResourceCPU:    resource.MustParse("200m"),
									corev1.ResourceMemory: resource.MustParse("512Mi"),
								},
								Limits: map[corev1.ResourceName]resource.Quantity{
									corev1.ResourceCPU:    resource.MustParse("500m"),
									corev1.ResourceMemory: resource.MustParse("1Gi"),
								},
							},
							VolumeMounts: []v1.VolumeMount{
								{Name: "cache-volume", MountPath: "/cache"},
								{Name: "flink-config-volume", MountPath: "/opt/flink/conf"},
								{
									Name:      "hadoop-config-volume",
									MountPath: "/etc/hadoop/conf",
									ReadOnly:  true,
								},
								{
									Name:      "gcp-service-account-volume",
									MountPath: "/etc/gcp_service_account/",
									ReadOnly:  true,
								},
							},
						},
						corev1.Container{Name: "sidecar", Image: "alpine"},
					},
					Volumes: []corev1.Volume{
						{
							Name: "cache-volume",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
						{
							Name: "flink-config-volume",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "flinkjobcluster-sample-configmap",
									},
								},
							},
						},
						{
							Name: "hadoop-config-volume",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "hadoop-configmap",
									},
								},
							},
						},
						{
							Name: "gcp-service-account-volume",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: "gcp-service-account-secret",
								},
							},
						},
					},
				},
			},
		},
	}

	assert.Assert(t, desiredState.TmDeployment != nil)
	assert.DeepEqual(
		t,
		*desiredState.TmDeployment,
		expectedDesiredTmDeployment,
		cmpopts.IgnoreUnexported(resource.Quantity{}))

	// Job
	var expectedDesiredJob = batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "flinkjobcluster-sample-job",
			Namespace: "default",
			Labels: map[string]string{
				"app": "flink", "cluster": "flinkjobcluster-sample"},
			OwnerReferences: []metav1.OwnerReference{
				{APIVersion: "flinkoperator.k8s.io/v1alpha1",
					Kind:               "FlinkCluster",
					Name:               "flinkjobcluster-sample",
					Controller:         &controller,
					BlockOwnerDeletion: &blockOwnerDeletion,
				},
			},
		},
		Spec: batchv1.JobSpec{
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "flink", "cluster": "flinkjobcluster-sample"},
				},
				Spec: v1.PodSpec{
					InitContainers: []v1.Container{
						{
							Name:    "gcs-downloader",
							Image:   "google/cloud-sdk",
							Command: []string{"gsutil"},
							Args: []string{
								"cp", "gs://my-bucket/my-job.jar", "/cache/my-job.jar",
							},
							VolumeMounts: []v1.VolumeMount{
								{Name: "cache-volume", MountPath: "/cache"},
							},
						},
					},
					Containers: []v1.Container{
						{
							Name:  "main",
							Image: "flink:1.8.1",
							Args: []string{
								"/opt/flink/bin/flink",
								"run",
								"--jobmanager",
								"flinkjobcluster-sample-jobmanager:8081",
								"--class",
								"org.apache.flink.examples.java.wordcount.WordCount",
								"--parallelism",
								"2",
								"/cache/my-job.jar",
								"--input",
								"./README.txt",
							},
							Env: []v1.EnvVar{
								{Name: "HADOOP_CONF_DIR", Value: "/etc/hadoop/conf"},
								{
									Name:  "GOOGLE_APPLICATION_CREDENTIALS",
									Value: "/etc/gcp_service_account/gcp_service_account_key.json",
								},
								{Name: "FOO", Value: "abc"},
							},
							VolumeMounts: []v1.VolumeMount{
								{Name: "cache-volume", MountPath: "/cache"},
								{
									Name:      "hadoop-config-volume",
									MountPath: "/etc/hadoop/conf",
									ReadOnly:  true,
								},
								{
									Name:      "gcp-service-account-volume",
									MountPath: "/etc/gcp_service_account/",
									ReadOnly:  true,
								},
							},
						},
					},
					RestartPolicy: v1alpha1.JobRestartPolicyNever,
					Volumes: []corev1.Volume{
						{
							Name: "cache-volume",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
						{
							Name: "hadoop-config-volume",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "hadoop-configmap",
									},
								},
							},
						},
						{
							Name: "gcp-service-account-volume",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: "gcp-service-account-secret",
								},
							},
						},
					},
				},
			},
			BackoffLimit: &jobBackoffLimit,
		},
	}

	assert.Assert(t, desiredState.Job != nil)
	assert.DeepEqual(
		t,
		*desiredState.Job,
		expectedDesiredJob)

	// ConfigMap
	var flinkConfYaml = `blob.server.port: 6124
jobmanager.rpc.address: flinkjobcluster-sample-jobmanager
jobmanager.rpc.port: 6123
query.server.port: 6125
rest.port: 8081
taskmanager.heap.size: 474m
taskmanager.numberOfTaskSlots: 1
taskmanager.rpc.port: 6122
`
	var expectedConfigMap = corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "flinkjobcluster-sample-configmap",
			Namespace: "default",
			Labels: map[string]string{
				"app": "flink", "cluster": "flinkjobcluster-sample",
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         "flinkoperator.k8s.io/v1alpha1",
					Kind:               "FlinkCluster",
					Name:               "flinkjobcluster-sample",
					Controller:         &controller,
					BlockOwnerDeletion: &blockOwnerDeletion,
				},
			},
		},
		Data: map[string]string{
			"flink-conf.yaml":          flinkConfYaml,
			"log4j-console.properties": getLogConf()["log4j-console.properties"],
			"logback-console.xml":      getLogConf()["logback-console.xml"],
		},
	}
	assert.Assert(t, desiredState.ConfigMap != nil)
	assert.DeepEqual(
		t,
		*desiredState.ConfigMap,
		expectedConfigMap)
}

func TestCalFlinkHeapSize(t *testing.T) {
	var memoryOffHeapRatio int32 = 25
	var memoryOffHeapMin = resource.MustParse("600M")

	// Case 1: Heap sizes are computed from memoryOffHeapMin or memoryOffHeapRatio
	cluster := &v1alpha1.FlinkCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mycluster",
			Namespace: "default",
		},
		Spec: v1alpha1.FlinkClusterSpec{
			JobManager: v1alpha1.JobManagerSpec{
				Resources: corev1.ResourceRequirements{
					Limits: map[corev1.ResourceName]resource.Quantity{
						corev1.ResourceMemory: resource.MustParse("1Gi"),
					},
				},
				MemoryOffHeapRatio: &memoryOffHeapRatio,
				MemoryOffHeapMin:   memoryOffHeapMin,
			},
			TaskManager: v1alpha1.TaskManagerSpec{
				Resources: corev1.ResourceRequirements{
					Limits: map[corev1.ResourceName]resource.Quantity{
						corev1.ResourceMemory: resource.MustParse("4Gi"),
					},
				},
				MemoryOffHeapRatio: &memoryOffHeapRatio,
				MemoryOffHeapMin:   memoryOffHeapMin,
			},
		},
	}

	flinkHeapSize := calFlinkHeapSize(cluster)
	expectedFlinkHeapSize := map[string]string{
		"jobmanager.heap.size":  "474m",  // get values calculated with limit - memoryOffHeapMin
		"taskmanager.heap.size": "3222m", // get values calculated with limit - limit * memoryOffHeapRatio / 100
	}
	assert.Assert(t, len(flinkHeapSize) == 2)
	assert.DeepEqual(
		t,
		flinkHeapSize,
		expectedFlinkHeapSize)

	// Case 2: No values when memory limits are missing or insufficient
	cluster = &v1alpha1.FlinkCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mycluster",
			Namespace: "default",
		},
		Spec: v1alpha1.FlinkClusterSpec{
			JobManager: v1alpha1.JobManagerSpec{
				Resources: corev1.ResourceRequirements{
					Limits: map[corev1.ResourceName]resource.Quantity{
						corev1.ResourceMemory: resource.MustParse("500Mi"),
					},
				},
				MemoryOffHeapRatio: &memoryOffHeapRatio,
				MemoryOffHeapMin:   memoryOffHeapMin,
			},
			TaskManager: v1alpha1.TaskManagerSpec{
				MemoryOffHeapRatio: &memoryOffHeapRatio,
				MemoryOffHeapMin:   memoryOffHeapMin,
			},
		},
	}

	flinkHeapSize = calFlinkHeapSize(cluster)
	assert.Assert(t, len(flinkHeapSize) == 0)
}
