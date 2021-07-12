/*
Copyright 2020 Google LLC.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by statelicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package volcano

import (
	"testing"

	"gotest.tools/assert"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/googlecloudplatform/flink-operator/controllers/model"
)

func TestGetClusterResource(t *testing.T) {
	jmRep := int32(1)
	replicas := int32(4)
	desiredState := &model.DesiredClusterState{
		JmStatefulSet: &appsv1.StatefulSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "flinkjobcluster-sample-jobmanager",
				Namespace: "default",
				Labels: map[string]string{
					"app":       "flink",
					"cluster":   "flinkjobcluster-sample",
					"component": "jobmanager",
				},
			},
			Spec: appsv1.StatefulSetSpec{
				Replicas:    &jmRep,
				ServiceName: "flinkjobcluster-sample-jobmanager",
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app":       "flink",
						"cluster":   "flinkjobcluster-sample",
						"component": "jobmanager",
					},
				},
				Template: v1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"app":       "flink",
							"cluster":   "flinkjobcluster-sample",
							"component": "jobmanager",
						},
						Annotations: map[string]string{
							"example.com": "example",
						},
					},
					Spec: v1.PodSpec{
						Containers: []v1.Container{
							{
								Name:  "jobmanager",
								Image: "flink:1.8.1",
								Args:  []string{"jobmanager"},
								Env: []v1.EnvVar{
									{
										Name: "JOB_MANAGER_CPU_LIMIT",
										ValueFrom: &v1.EnvVarSource{
											ResourceFieldRef: &v1.ResourceFieldSelector{
												ContainerName: "jobmanager",
												Resource:      "limits.cpu",
												Divisor:       resource.MustParse("1m"),
											},
										},
									},
									{
										Name: "JOB_MANAGER_MEMORY_LIMIT",
										ValueFrom: &v1.EnvVarSource{
											ResourceFieldRef: &v1.ResourceFieldSelector{
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
								Resources: v1.ResourceRequirements{
									Requests: map[v1.ResourceName]resource.Quantity{
										v1.ResourceCPU:    resource.MustParse("100m"),
										v1.ResourceMemory: resource.MustParse("256Mi"),
									},
									Limits: map[v1.ResourceName]resource.Quantity{
										v1.ResourceCPU:    resource.MustParse("200m"),
										v1.ResourceMemory: resource.MustParse("512Mi"),
									},
								},
								VolumeMounts: []v1.VolumeMount{
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
						Volumes: []v1.Volume{
							{
								Name: "flink-config-volume",
								VolumeSource: v1.VolumeSource{
									ConfigMap: &v1.ConfigMapVolumeSource{
										LocalObjectReference: v1.LocalObjectReference{
											Name: "flinkjobcluster-sample-configmap",
										},
									},
								},
							},
							{
								Name: "hadoop-config-volume",
								VolumeSource: v1.VolumeSource{
									ConfigMap: &v1.ConfigMapVolumeSource{
										LocalObjectReference: v1.LocalObjectReference{
											Name: "hadoop-configmap",
										},
									},
								},
							},
							{
								Name: "gcp-service-account-volume",
								VolumeSource: v1.VolumeSource{
									Secret: &v1.SecretVolumeSource{
										SecretName: "gcp-service-account-secret",
									},
								},
							},
						},
					},
				},
			},
		},
		TmStatefulSet: &appsv1.StatefulSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "flinkjobcluster-sample-taskmanager",
				Namespace: "default",
				Labels: map[string]string{
					"app":       "flink",
					"cluster":   "flinkjobcluster-sample",
					"component": "taskmanager",
				},
			},
			Spec: appsv1.StatefulSetSpec{
				Replicas:            &replicas,
				ServiceName:         "flinkjobcluster-sample-taskmanager",
				PodManagementPolicy: "Parallel",
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app":       "flink",
						"cluster":   "flinkjobcluster-sample",
						"component": "taskmanager",
					},
				},
				Template: v1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"app":       "flink",
							"cluster":   "flinkjobcluster-sample",
							"component": "taskmanager",
						},
						Annotations: map[string]string{
							"example.com": "example",
						},
					},
					Spec: v1.PodSpec{
						Containers: []v1.Container{
							v1.Container{
								Name:  "taskmanager",
								Image: "flink:1.8.1",
								Args:  []string{"taskmanager"},
								Ports: []v1.ContainerPort{
									{Name: "data", ContainerPort: 6121},
									{Name: "rpc", ContainerPort: 6122},
									{Name: "query", ContainerPort: 6125},
								},
								Env: []v1.EnvVar{
									{
										Name: "TASK_MANAGER_CPU_LIMIT",
										ValueFrom: &v1.EnvVarSource{
											ResourceFieldRef: &v1.ResourceFieldSelector{
												ContainerName: "taskmanager",
												Resource:      "limits.cpu",
												Divisor:       resource.MustParse("1m"),
											},
										},
									},
									{
										Name: "TASK_MANAGER_MEMORY_LIMIT",
										ValueFrom: &v1.EnvVarSource{
											ResourceFieldRef: &v1.ResourceFieldSelector{
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
								Resources: v1.ResourceRequirements{
									Requests: map[v1.ResourceName]resource.Quantity{
										v1.ResourceCPU:    resource.MustParse("200m"),
										v1.ResourceMemory: resource.MustParse("512Mi"),
									},
									Limits: map[v1.ResourceName]resource.Quantity{
										v1.ResourceCPU:    resource.MustParse("500m"),
										v1.ResourceMemory: resource.MustParse("1Gi"),
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
							v1.Container{Name: "sidecar", Image: "alpine"},
						},
						Volumes: []v1.Volume{
							{
								Name: "cache-volume",
								VolumeSource: v1.VolumeSource{
									EmptyDir: &v1.EmptyDirVolumeSource{},
								},
							},
							{
								Name: "flink-config-volume",
								VolumeSource: v1.VolumeSource{
									ConfigMap: &v1.ConfigMapVolumeSource{
										LocalObjectReference: v1.LocalObjectReference{
											Name: "flinkjobcluster-sample-configmap",
										},
									},
								},
							},
							{
								Name: "hadoop-config-volume",
								VolumeSource: v1.VolumeSource{
									ConfigMap: &v1.ConfigMapVolumeSource{
										LocalObjectReference: v1.LocalObjectReference{
											Name: "hadoop-configmap",
										},
									},
								},
							},
							{
								Name: "gcp-service-account-volume",
								VolumeSource: v1.VolumeSource{
									Secret: &v1.SecretVolumeSource{
										SecretName: "gcp-service-account-secret",
									},
								},
							},
						},
					},
				},
			},
		},
	}

	res, size := getClusterResource(desiredState)
	assert.Assert(t, size == 5)
	assert.Assert(t, res.Memory().String() == "2304Mi")
	assert.Assert(t, res.Cpu().MilliValue() == 900)
}
