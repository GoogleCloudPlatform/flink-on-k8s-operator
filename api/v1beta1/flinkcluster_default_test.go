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

package v1beta1

import (
	"testing"

	"github.com/google/go-cmp/cmp/cmpopts"
	"k8s.io/apimachinery/pkg/api/resource"

	"gotest.tools/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Tests default values are set as expected.
func TestSetDefault(t *testing.T) {
	var cluster = FlinkCluster{
		Spec: FlinkClusterSpec{
			Job: &JobSpec{},
			JobManager: JobManagerSpec{
				Ingress: &JobManagerIngressSpec{},
			},
			HadoopConfig: &HadoopConfig{},
		},
	}
	_SetDefault(&cluster)

	var defaultJmReplicas = int32(1)
	var defaultJmRPCPort = int32(6123)
	var defaultJmBlobPort = int32(6124)
	var defaultJmQueryPort = int32(6125)
	var defaultJmUIPort = int32(8081)
	var defaultJmIngressTLSUse = false
	var defaultTmDataPort = int32(6121)
	var defaultTmRPCPort = int32(6122)
	var defaultTmQueryPort = int32(6125)
	var defaultJobAllowNonRestoredState = false
	var defaultJobParallelism = int32(1)
	var defaultJobNoLoggingToStdout = false
	var defaultJobRestartPolicy = JobRestartPolicyNever
	var defaultMemoryOffHeapRatio = int32(25)
	var defaultMemoryOffHeapMin = resource.MustParse("600M")
	var defaultRecreateOnUpdate = true
	var expectedCluster = FlinkCluster{
		TypeMeta:   metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{},
		Spec: FlinkClusterSpec{
			Image: ImageSpec{
				Name:        "",
				PullPolicy:  "Always",
				PullSecrets: nil,
			},
			JobManager: JobManagerSpec{
				Replicas:    &defaultJmReplicas,
				AccessScope: "Cluster",
				Ingress: &JobManagerIngressSpec{
					UseTLS: &defaultJmIngressTLSUse,
				},
				Ports: JobManagerPorts{
					RPC:   &defaultJmRPCPort,
					Blob:  &defaultJmBlobPort,
					Query: &defaultJmQueryPort,
					UI:    &defaultJmUIPort,
				},
				Resources:          corev1.ResourceRequirements{},
				MemoryOffHeapRatio: &defaultMemoryOffHeapRatio,
				MemoryOffHeapMin:   defaultMemoryOffHeapMin,
				Volumes:            nil,
				VolumeMounts:       nil,
				SecurityContext:    nil,
			},
			TaskManager: TaskManagerSpec{
				Replicas: 0,
				Ports: TaskManagerPorts{
					Data:  &defaultTmDataPort,
					RPC:   &defaultTmRPCPort,
					Query: &defaultTmQueryPort,
				},
				Resources:          corev1.ResourceRequirements{},
				MemoryOffHeapRatio: &defaultMemoryOffHeapRatio,
				MemoryOffHeapMin:   defaultMemoryOffHeapMin,
				Volumes:            nil,
				SecurityContext:    nil,
			},
			Job: &JobSpec{
				AllowNonRestoredState: &defaultJobAllowNonRestoredState,
				Parallelism:           &defaultJobParallelism,
				NoLoggingToStdout:     &defaultJobNoLoggingToStdout,
				RestartPolicy:         &defaultJobRestartPolicy,
				CleanupPolicy: &CleanupPolicy{
					AfterJobSucceeds:  "DeleteCluster",
					AfterJobFails:     "KeepCluster",
					AfterJobCancelled: "DeleteCluster",
				},
				SecurityContext: nil,
			},
			FlinkProperties: nil,
			HadoopConfig: &HadoopConfig{
				MountPath: "/etc/hadoop/conf",
			},
			EnvVars:          nil,
			RecreateOnUpdate: &defaultRecreateOnUpdate,
		},
		Status: FlinkClusterStatus{},
	}

	assert.DeepEqual(
		t,
		cluster,
		expectedCluster,
		cmpopts.IgnoreUnexported(resource.Quantity{}))
}

// Tests non-default values are not overwritten unexpectedly.
func TestSetNonDefault(t *testing.T) {
	var jmReplicas = int32(2)
	var jmRPCPort = int32(8123)
	var jmBlobPort = int32(8124)
	var jmQueryPort = int32(8125)
	var jmUIPort = int32(9081)
	var jmIngressTLSUse = true
	var tmDataPort = int32(8121)
	var tmRPCPort = int32(8122)
	var tmQueryPort = int32(8125)
	var jobAllowNonRestoredState = true
	var jobParallelism = int32(2)
	var jobNoLoggingToStdout = true
	var jobRestartPolicy = JobRestartPolicyFromSavepointOnFailure
	var memoryOffHeapRatio = int32(50)
	var memoryOffHeapMin = resource.MustParse("600M")
	var recreateOnUpdate = false
	var securityContextUserGroup = int64(9999)
	var securityContext = corev1.PodSecurityContext{
		RunAsUser:  &securityContextUserGroup,
		RunAsGroup: &securityContextUserGroup,
	}
	var cluster = FlinkCluster{
		TypeMeta:   metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{},
		Spec: FlinkClusterSpec{
			Image: ImageSpec{
				Name:        "",
				PullPolicy:  "Always",
				PullSecrets: nil,
			},
			JobManager: JobManagerSpec{
				Replicas:    &jmReplicas,
				AccessScope: "Cluster",
				Ingress: &JobManagerIngressSpec{
					UseTLS: &jmIngressTLSUse,
				},
				Ports: JobManagerPorts{
					RPC:   &jmRPCPort,
					Blob:  &jmBlobPort,
					Query: &jmQueryPort,
					UI:    &jmUIPort,
				},
				Resources:          corev1.ResourceRequirements{},
				MemoryOffHeapRatio: &memoryOffHeapRatio,
				MemoryOffHeapMin:   memoryOffHeapMin,
				Volumes:            nil,
				VolumeMounts:       nil,
				SecurityContext:    &securityContext,
			},
			TaskManager: TaskManagerSpec{
				Replicas: 0,
				Ports: TaskManagerPorts{
					Data:  &tmDataPort,
					RPC:   &tmRPCPort,
					Query: &tmQueryPort,
				},
				Resources:          corev1.ResourceRequirements{},
				MemoryOffHeapRatio: &memoryOffHeapRatio,
				MemoryOffHeapMin:   memoryOffHeapMin,
				Volumes:            nil,
				SecurityContext:    &securityContext,
			},
			Job: &JobSpec{
				AllowNonRestoredState: &jobAllowNonRestoredState,
				Parallelism:           &jobParallelism,
				NoLoggingToStdout:     &jobNoLoggingToStdout,
				RestartPolicy:         &jobRestartPolicy,
				SecurityContext:       &securityContext,
				CleanupPolicy: &CleanupPolicy{
					AfterJobSucceeds:  "DeleteTaskManagers",
					AfterJobFails:     "DeleteCluster",
					AfterJobCancelled: "KeepCluster",
				},
			},
			FlinkProperties: nil,
			HadoopConfig: &HadoopConfig{
				MountPath: "/opt/flink/hadoop/conf",
			},
			EnvVars:          nil,
			RecreateOnUpdate: &recreateOnUpdate,
		},
		Status: FlinkClusterStatus{},
	}

	_SetDefault(&cluster)

	var expectedCluster = FlinkCluster{
		TypeMeta:   metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{},
		Spec: FlinkClusterSpec{
			Image: ImageSpec{
				Name:        "",
				PullPolicy:  "Always",
				PullSecrets: nil,
			},
			JobManager: JobManagerSpec{
				Replicas:    &jmReplicas,
				AccessScope: "Cluster",
				Ingress: &JobManagerIngressSpec{
					UseTLS: &jmIngressTLSUse,
				},
				Ports: JobManagerPorts{
					RPC:   &jmRPCPort,
					Blob:  &jmBlobPort,
					Query: &jmQueryPort,
					UI:    &jmUIPort,
				},
				Resources:          corev1.ResourceRequirements{},
				MemoryOffHeapRatio: &memoryOffHeapRatio,
				MemoryOffHeapMin:   memoryOffHeapMin,
				Volumes:            nil,
				VolumeMounts:       nil,
				SecurityContext:    &securityContext,
			},
			TaskManager: TaskManagerSpec{
				Replicas: 0,
				Ports: TaskManagerPorts{
					Data:  &tmDataPort,
					RPC:   &tmRPCPort,
					Query: &tmQueryPort,
				},
				Resources:          corev1.ResourceRequirements{},
				MemoryOffHeapRatio: &memoryOffHeapRatio,
				MemoryOffHeapMin:   memoryOffHeapMin,
				Volumes:            nil,
				SecurityContext:    &securityContext,
			},
			Job: &JobSpec{
				AllowNonRestoredState: &jobAllowNonRestoredState,
				Parallelism:           &jobParallelism,
				NoLoggingToStdout:     &jobNoLoggingToStdout,
				RestartPolicy:         &jobRestartPolicy,
				SecurityContext:       &securityContext,
				CleanupPolicy: &CleanupPolicy{
					AfterJobSucceeds:  "DeleteTaskManagers",
					AfterJobFails:     "DeleteCluster",
					AfterJobCancelled: "KeepCluster",
				},
			},
			FlinkProperties: nil,
			HadoopConfig: &HadoopConfig{
				MountPath: "/opt/flink/hadoop/conf",
			},
			EnvVars:          nil,
			RecreateOnUpdate: &recreateOnUpdate,
		},
		Status: FlinkClusterStatus{},
	}

	assert.DeepEqual(
		t,
		cluster,
		expectedCluster,
		cmpopts.IgnoreUnexported(resource.Quantity{}))
}
