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

	"github.com/google/go-cmp/cmp/cmpopts"
	flinkoperatorv1alpha1 "github.com/googlecloudplatform/flink-operator/api/v1alpha1"
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
	var restartPolicy = corev1.RestartPolicy("OnFailure")
	var className = "org.apache.flink.examples.java.wordcount.WordCount"
	var hostFormat = "{{$clusterName}}.example.com"

	// Setup.
	var cluster = &flinkoperatorv1alpha1.FlinkCluster{
		TypeMeta: metav1.TypeMeta{
			Kind:       "FlinkCluster",
			APIVersion: "flinkoperator.k8s.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "flinkjobcluster-sample",
			Namespace: "default",
		},
		Spec: flinkoperatorv1alpha1.FlinkClusterSpec{
			ImageSpec: flinkoperatorv1alpha1.ImageSpec{Name: "flink:1.8.1"},
			JobSpec: &flinkoperatorv1alpha1.JobSpec{
				Args:          []string{"--input", "./README.txt"},
				ClassName:     &className,
				JarFile:       "./examples/batch/WordCount.jar",
				Parallelism:   &parallelism,
				RestartPolicy: &restartPolicy,
			},
			JobManagerSpec: flinkoperatorv1alpha1.JobManagerSpec{
				AccessScope: flinkoperatorv1alpha1.AccessScope.VPC,
				Ingress: &flinkoperatorv1alpha1.JobManagerIngressSpec{
					HostFormat: &hostFormat,
					Annotations: map[string]string{
						"kubernetes.io/ingress.class":                "nginx",
						"certmanager.k8s.io/cluster-issuer":          "letsencrypt-stg",
						"nginx.ingress.kubernetes.io/rewrite-target": "/",
					},
					UseTLS: &useTLS,
				},
				Ports: flinkoperatorv1alpha1.JobManagerPorts{
					RPC:   &jmRPCPort,
					Blob:  &jmBlobPort,
					Query: &jmQueryPort,
					UI:    &jmUIPort,
				},
				Resources: corev1.ResourceRequirements{
					Requests: map[corev1.ResourceName]resource.Quantity{
						"CPU":    resource.MustParse("100m"),
						"Memory": resource.MustParse("256Mi"),
					},
					Limits: map[corev1.ResourceName]resource.Quantity{
						"CPU":    resource.MustParse("200m"),
						"Memory": resource.MustParse("512Mi"),
					},
				},
			},
			TaskManagerSpec: flinkoperatorv1alpha1.TaskManagerSpec{
				Replicas: 42,
				Ports: flinkoperatorv1alpha1.TaskManagerPorts{
					Data:  &tmDataPort,
					RPC:   &tmRPCPort,
					Query: &tmQueryPort,
				},
				Resources: corev1.ResourceRequirements{
					Requests: map[corev1.ResourceName]resource.Quantity{
						"CPU":    resource.MustParse("200m"),
						"Memory": resource.MustParse("512Mi"),
					},
					Limits: map[corev1.ResourceName]resource.Quantity{
						"CPU":    resource.MustParse("500m"),
						"Memory": resource.MustParse("1Gi"),
					},
				},
				Sidecars: []corev1.Container{{Name: "sidecar", Image: "alpine"}},
				Volumes: []corev1.Volume{
					{
						Name: "cache-volume",
						VolumeSource: corev1.VolumeSource{
							EmptyDir: &corev1.EmptyDirVolumeSource{},
						},
					},
				},
				Mounts: []corev1.VolumeMount{
					{Name: "cache-volume", MountPath: "/cache"},
				},
			},
			FlinkProperties: map[string]string{"taskmanager.numberOfTaskSlots": "1"},
			EnvVars:         []corev1.EnvVar{{Name: "FOO", Value: "abc"}},
		},
	}

	// Run.
	var desiredState = getDesiredClusterState(cluster)

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
							Env: []corev1.EnvVar{
								{
									Name:  "JOB_MANAGER_RPC_ADDRESS",
									Value: "flinkjobcluster-sample-jobmanager",
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
									Value: "taskmanager.numberOfTaskSlots: 1\n",
								},
								{
									Name:  "FOO",
									Value: "abc",
								},
							},
							Resources: corev1.ResourceRequirements{
								Requests: map[corev1.ResourceName]resource.Quantity{
									"CPU":    resource.MustParse("100m"),
									"Memory": resource.MustParse("256Mi"),
								},
								Limits: map[corev1.ResourceName]resource.Quantity{
									"CPU":    resource.MustParse("200m"),
									"Memory": resource.MustParse("512Mi"),
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
							Env: []corev1.EnvVar{
								{
									Name:  "JOB_MANAGER_RPC_ADDRESS",
									Value: "flinkjobcluster-sample-jobmanager",
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
									Value: "taskmanager.numberOfTaskSlots: 1\n",
								},
								{
									Name:  "FOO",
									Value: "abc",
								},
							},
							Resources: corev1.ResourceRequirements{
								Requests: map[corev1.ResourceName]resource.Quantity{
									"CPU":    resource.MustParse("200m"),
									"Memory": resource.MustParse("512Mi"),
								},
								Limits: map[corev1.ResourceName]resource.Quantity{
									"CPU":    resource.MustParse("500m"),
									"Memory": resource.MustParse("1Gi"),
								},
							},
							VolumeMounts: []v1.VolumeMount{
								{Name: "cache-volume", MountPath: "/cache"},
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
					Containers: []v1.Container{
						{
							Name:  "main",
							Image: "flink:1.8.1",
							Args: []string{
								"./bin/flink",
								"run",
								"--jobmanager",
								"flinkjobcluster-sample-jobmanager:8081",
								"--class",
								"org.apache.flink.examples.java.wordcount.WordCount",
								"--parallelism",
								"2",
								"./examples/batch/WordCount.jar",
								"--input",
								"./README.txt",
							},
							Env: []v1.EnvVar{{Name: "FOO", Value: "abc"}},
						},
					},
					RestartPolicy: "OnFailure",
				},
			},
		},
	}

	assert.Assert(t, desiredState.Job != nil)
	assert.DeepEqual(
		t,
		*desiredState.Job,
		expectedDesiredJob)
}
