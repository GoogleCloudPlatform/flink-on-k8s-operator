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

package v1alpha1

import (
	"testing"

	"gotest.tools/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestValidateCreate(t *testing.T) {
	var jmReplicas int32 = 1
	var rpcPort int32 = 8001
	var blobPort int32 = 8002
	var queryPort int32 = 8003
	var uiPort int32 = 8004
	var dataPort int32 = 8005
	var parallelism int32 = 2
	var restartPolicy = corev1.RestartPolicyOnFailure
	var cluster = FlinkCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mycluster",
			Namespace: "default",
		},
		Spec: FlinkClusterSpec{
			Image: ImageSpec{
				Name:       "flink:1.8.1",
				PullPolicy: corev1.PullPolicy("Always"),
			},
			JobManager: JobManagerSpec{
				Replicas:    &jmReplicas,
				AccessScope: AccessScope.VPC,
				Ports: JobManagerPorts{
					RPC:   &rpcPort,
					Blob:  &blobPort,
					Query: &queryPort,
					UI:    &uiPort,
				},
			},
			TaskManager: TaskManagerSpec{
				Replicas: 3,
				Ports: TaskManagerPorts{
					RPC:   &rpcPort,
					Data:  &dataPort,
					Query: &queryPort,
				},
			},
			Job: &JobSpec{
				JarFile:       "gs://my-bucket/myjob.jar",
				Parallelism:   &parallelism,
				RestartPolicy: &restartPolicy,
				CleanupPolicy: &CleanupPolicy{
					AfterJobSucceeds: CleanupActionKeepCluster,
					AfterJobFails:    CleanupActionDeleteTaskManager,
				},
			},
		},
	}
	var validator = &Validator{}
	var err = validator.ValidateCreate(&cluster)
	assert.NilError(t, err, "create validation failed unexpectedly")
}

func TestInvalidImageSpec(t *testing.T) {
	var validator = &Validator{}

	var cluster = FlinkCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mycluster",
			Namespace: "default",
		},
		Spec: FlinkClusterSpec{},
	}
	var err = validator.ValidateCreate(&cluster)
	var expectedErr = "image name is unspecified"
	assert.Equal(t, err.Error(), expectedErr)

	cluster = FlinkCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mycluster",
			Namespace: "default",
		},
		Spec: FlinkClusterSpec{
			Image: ImageSpec{
				Name:       "flink:1.8.1",
				PullPolicy: corev1.PullPolicy("XXX"),
			},
		},
	}
	err = validator.ValidateCreate(&cluster)
	expectedErr = "invalid image pullPolicy: XXX"
	assert.Equal(t, err.Error(), expectedErr)
}

func TestInvalidJobManagerSpec(t *testing.T) {
	var jmReplicas1 int32 = 1
	var jmReplicas2 int32 = 2
	var rpcPort int32 = 8001
	var blobPort int32 = 8002
	var queryPort int32 = 8003
	var uiPort int32 = 8004

	var cluster = FlinkCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mycluster",
			Namespace: "default",
		},
		Spec: FlinkClusterSpec{
			Image: ImageSpec{
				Name:       "flink:1.8.1",
				PullPolicy: corev1.PullPolicy("Always"),
			},
			JobManager: JobManagerSpec{
				Replicas:    &jmReplicas2,
				AccessScope: AccessScope.VPC,
				Ports: JobManagerPorts{
					RPC:   &rpcPort,
					Blob:  &blobPort,
					Query: &queryPort,
					UI:    &uiPort,
				},
			},
		},
	}
	var err = validator.ValidateCreate(&cluster)
	var expectedErr = "invalid JobManager replicas, it must be 1"
	assert.Equal(t, err.Error(), expectedErr)

	cluster = FlinkCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mycluster",
			Namespace: "default",
		},
		Spec: FlinkClusterSpec{
			Image: ImageSpec{
				Name:       "flink:1.8.1",
				PullPolicy: corev1.PullPolicy("Always"),
			},
			JobManager: JobManagerSpec{
				Replicas:    &jmReplicas1,
				AccessScope: "XXX",
				Ports: JobManagerPorts{
					RPC:   &rpcPort,
					Blob:  &blobPort,
					Query: &queryPort,
					UI:    &uiPort,
				},
			},
		},
	}
	err = validator.ValidateCreate(&cluster)
	expectedErr = "invalid JobManager access scope: XXX"
	assert.Equal(t, err.Error(), expectedErr)

	cluster = FlinkCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mycluster",
			Namespace: "default",
		},
		Spec: FlinkClusterSpec{
			Image: ImageSpec{
				Name:       "flink:1.8.1",
				PullPolicy: corev1.PullPolicy("Always"),
			},
			JobManager: JobManagerSpec{
				Replicas:    &jmReplicas1,
				AccessScope: AccessScope.VPC,
				Ports: JobManagerPorts{
					RPC:   nil,
					Blob:  &blobPort,
					Query: &queryPort,
					UI:    &uiPort,
				},
			},
		},
	}
	err = validator.ValidateCreate(&cluster)
	expectedErr = "jobmanager rpc port is unspecified"
	assert.Equal(t, err.Error(), expectedErr)
}

func TestInvalidTaskManagerSpec(t *testing.T) {
	var jmReplicas int32 = 1
	var rpcPort int32 = 8001
	var blobPort int32 = 8002
	var queryPort int32 = 8003
	var uiPort int32 = 8004
	var dataPort int32 = 8005

	var cluster = FlinkCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mycluster",
			Namespace: "default",
		},
		Spec: FlinkClusterSpec{
			Image: ImageSpec{
				Name:       "flink:1.8.1",
				PullPolicy: corev1.PullPolicy("Always"),
			},
			JobManager: JobManagerSpec{
				Replicas:    &jmReplicas,
				AccessScope: AccessScope.VPC,
				Ports: JobManagerPorts{
					RPC:   &rpcPort,
					Blob:  &blobPort,
					Query: &queryPort,
					UI:    &uiPort,
				},
			},
			TaskManager: TaskManagerSpec{
				Replicas: 0,
				Ports: TaskManagerPorts{
					RPC:   &rpcPort,
					Data:  &dataPort,
					Query: &queryPort,
				},
			},
		},
	}
	var err = validator.ValidateCreate(&cluster)
	var expectedErr = "invalid TaskManager replicas, it must >= 1"
	assert.Equal(t, err.Error(), expectedErr)

	cluster = FlinkCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mycluster",
			Namespace: "default",
		},
		Spec: FlinkClusterSpec{
			Image: ImageSpec{
				Name:       "flink:1.8.1",
				PullPolicy: corev1.PullPolicy("Always"),
			},
			JobManager: JobManagerSpec{
				Replicas:    &jmReplicas,
				AccessScope: AccessScope.VPC,
				Ports: JobManagerPorts{
					RPC:   &rpcPort,
					Blob:  &blobPort,
					Query: &queryPort,
					UI:    &uiPort,
				},
			},
			TaskManager: TaskManagerSpec{
				Replicas: 1,
				Ports: TaskManagerPorts{
					RPC:   &rpcPort,
					Data:  &dataPort,
					Query: nil,
				},
			},
		},
	}
	err = validator.ValidateCreate(&cluster)
	expectedErr = "taskmanager query port is unspecified"
	assert.Equal(t, err.Error(), expectedErr)
}

func TestInvalidJobSpec(t *testing.T) {
	var jmReplicas int32 = 1
	var rpcPort int32 = 8001
	var blobPort int32 = 8002
	var queryPort int32 = 8003
	var uiPort int32 = 8004
	var dataPort int32 = 8005
	var restartPolicy = corev1.RestartPolicyOnFailure
	var invalidRestartPolicy = corev1.RestartPolicy("XXX")
	var validator = &Validator{}
	var parallelism int32 = 2

	var cluster = FlinkCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mycluster",
			Namespace: "default",
		},
		Spec: FlinkClusterSpec{
			Image: ImageSpec{
				Name:       "flink:1.8.1",
				PullPolicy: corev1.PullPolicy("Always"),
			},
			JobManager: JobManagerSpec{
				Replicas:    &jmReplicas,
				AccessScope: AccessScope.VPC,
				Ports: JobManagerPorts{
					RPC:   &rpcPort,
					Blob:  &blobPort,
					Query: &queryPort,
					UI:    &uiPort,
				},
			},
			TaskManager: TaskManagerSpec{
				Replicas: 3,
				Ports: TaskManagerPorts{
					RPC:   &rpcPort,
					Data:  &dataPort,
					Query: &queryPort,
				},
			},
			Job: &JobSpec{
				JarFile:       "",
				RestartPolicy: &restartPolicy,
			},
		},
	}
	var err = validator.ValidateCreate(&cluster)
	var expectedErr = "job jarFile is unspecified"
	assert.Equal(t, err.Error(), expectedErr)

	cluster = FlinkCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mycluster",
			Namespace: "default",
		},
		Spec: FlinkClusterSpec{
			Image: ImageSpec{
				Name:       "flink:1.8.1",
				PullPolicy: corev1.PullPolicy("Always"),
			},
			JobManager: JobManagerSpec{
				Replicas:    &jmReplicas,
				AccessScope: AccessScope.VPC,
				Ports: JobManagerPorts{
					RPC:   &rpcPort,
					Blob:  &blobPort,
					Query: &queryPort,
					UI:    &uiPort,
				},
			},
			TaskManager: TaskManagerSpec{
				Replicas: 3,
				Ports: TaskManagerPorts{
					RPC:   &rpcPort,
					Data:  &dataPort,
					Query: &queryPort,
				},
			},
			Job: &JobSpec{
				JarFile:       "gs://my-bucket/myjob.jar",
				RestartPolicy: &restartPolicy,
			},
		},
	}
	err = validator.ValidateCreate(&cluster)
	expectedErr = "job parallelism is unspecified"
	assert.Equal(t, err.Error(), expectedErr)

	cluster = FlinkCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mycluster",
			Namespace: "default",
		},
		Spec: FlinkClusterSpec{
			Image: ImageSpec{
				Name:       "flink:1.8.1",
				PullPolicy: corev1.PullPolicy("Always"),
			},
			JobManager: JobManagerSpec{
				Replicas:    &jmReplicas,
				AccessScope: AccessScope.VPC,
				Ports: JobManagerPorts{
					RPC:   &rpcPort,
					Blob:  &blobPort,
					Query: &queryPort,
					UI:    &uiPort,
				},
			},
			TaskManager: TaskManagerSpec{
				Replicas: 3,
				Ports: TaskManagerPorts{
					RPC:   &rpcPort,
					Data:  &dataPort,
					Query: &queryPort,
				},
			},
			Job: &JobSpec{
				JarFile:       "gs://my-bucket/myjob.jar",
				Parallelism:   &parallelism,
				RestartPolicy: &invalidRestartPolicy,
			},
		},
	}
	err = validator.ValidateCreate(&cluster)
	expectedErr = "invalid job restartPolicy: XXX"
	assert.Equal(t, err.Error(), expectedErr)

	cluster = FlinkCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mycluster",
			Namespace: "default",
		},
		Spec: FlinkClusterSpec{
			Image: ImageSpec{
				Name:       "flink:1.8.1",
				PullPolicy: corev1.PullPolicy("Always"),
			},
			JobManager: JobManagerSpec{
				Replicas:    &jmReplicas,
				AccessScope: AccessScope.VPC,
				Ports: JobManagerPorts{
					RPC:   &rpcPort,
					Blob:  &blobPort,
					Query: &queryPort,
					UI:    &uiPort,
				},
			},
			TaskManager: TaskManagerSpec{
				Replicas: 3,
				Ports: TaskManagerPorts{
					RPC:   &rpcPort,
					Data:  &dataPort,
					Query: &queryPort,
				},
			},
			Job: &JobSpec{
				JarFile:       "gs://my-bucket/myjob.jar",
				Parallelism:   &parallelism,
				RestartPolicy: &restartPolicy,
				CleanupPolicy: &CleanupPolicy{
					AfterJobSucceeds: "XXX",
					AfterJobFails:    CleanupActionDeleteCluster,
				},
			},
		},
	}
	err = validator.ValidateCreate(&cluster)
	expectedErr = "invalid cleanupPolicy.afterJobSucceeds: XXX"
	assert.Equal(t, err.Error(), expectedErr)
}

func TestUpdateStatusAllowed(t *testing.T) {
	var oldCluster = FlinkCluster{Status: FlinkClusterStatus{State: "NoReady"}}
	var newCluster = FlinkCluster{Status: FlinkClusterStatus{State: "Running"}}
	var validator = &Validator{}
	var err = validator.ValidateUpdate(&oldCluster, &newCluster)
	assert.NilError(t, err, "updating status failed unexpectedly")
}

func TestUpdateSpecNotAllowed(t *testing.T) {
	var oldCluster = FlinkCluster{Spec: FlinkClusterSpec{Image: ImageSpec{Name: "flink:1.8.1"}}}
	var newCluster = FlinkCluster{Spec: FlinkClusterSpec{Image: ImageSpec{Name: "flink:1.9.0"}}}
	var validator = &Validator{}
	var err = validator.ValidateUpdate(&oldCluster, &newCluster)
	var expectedErr = "updating FlinkCluster spec is not allowed," +
		" please delete the resouce and recreate"
	assert.Equal(t, err.Error(), expectedErr)
}
