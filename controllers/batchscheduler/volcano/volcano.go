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
	"fmt"

	"golang.org/x/net/context"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd"
	scheduling "volcano.sh/volcano/pkg/apis/scheduling/v1beta1"
	volcanoclient "volcano.sh/volcano/pkg/client/clientset/versioned"

	"github.com/googlecloudplatform/flink-operator/api/v1beta1"
	schedulerinterface "github.com/googlecloudplatform/flink-operator/controllers/batchscheduler/interface"
	"github.com/googlecloudplatform/flink-operator/controllers/model"
)

const (
	schedulerName = "volcano"
)

// volcano scheduler implements the BatchScheduler interface.
type VolcanoBatchScheduler struct {
	volcanoClient volcanoclient.Interface
}

// Create volcano BatchScheduler
func New() (schedulerinterface.BatchScheduler, error) {
	config, err := clientcmd.BuildConfigFromFlags("", "")
	if err != nil {
		return nil, err
	}

	vcClient, err := volcanoclient.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize volcano client with error %v", err)
	}
	return &VolcanoBatchScheduler{
		volcanoClient: vcClient,
	}, nil
}

// Name returns the current scheduler name.
func (v *VolcanoBatchScheduler) Name() string {
	return schedulerName
}

// Schedule reconciles batch scheduling
func (v *VolcanoBatchScheduler) Schedule(cluster *v1beta1.FlinkCluster, state *model.DesiredClusterState) error {
	res, size := getClusterResource(state)
	if err := v.syncPodGroup(cluster, size, res); err != nil {
		return err
	}
	v.setSchedulerMeta(cluster, state)
	return nil
}

func (v *VolcanoBatchScheduler) setSchedulerMeta(cluster *v1beta1.FlinkCluster, state *model.DesiredClusterState) {
	podgroupName := v.getPodGroupName(cluster)
	if state.TmStatefulSet != nil {
		state.TmStatefulSet.Spec.Template.Spec.SchedulerName = v.Name()
		if state.TmStatefulSet.Spec.Template.Annotations == nil {
			state.TmStatefulSet.Spec.Template.Annotations = make(map[string]string)
		}
		state.TmStatefulSet.Spec.Template.Annotations[scheduling.KubeGroupNameAnnotationKey] = podgroupName
	}
	if state.JmStatefulSet != nil {
		state.JmStatefulSet.Spec.Template.Spec.SchedulerName = v.Name()
		if state.JmStatefulSet.Spec.Template.Annotations == nil {
			state.JmStatefulSet.Spec.Template.Annotations = make(map[string]string)
		}
		state.JmStatefulSet.Spec.Template.Annotations[scheduling.KubeGroupNameAnnotationKey] = podgroupName
	}
	if state.Job != nil {
		state.Job.Spec.Template.Spec.SchedulerName = v.Name()
	}
}

func (v *VolcanoBatchScheduler) getPodGroupName(cluster *v1beta1.FlinkCluster) string {
	return fmt.Sprintf("flink-%s", cluster.Name)
}

// Converts the FlinkCluster as owner reference for its child resources.
func newOwnerReference(flinkCluster *v1beta1.FlinkCluster) metav1.OwnerReference {
	return metav1.OwnerReference{
		APIVersion:         flinkCluster.APIVersion,
		Kind:               flinkCluster.Kind,
		Name:               flinkCluster.Name,
		UID:                flinkCluster.UID,
		Controller:         &[]bool{true}[0],
		BlockOwnerDeletion: &[]bool{false}[0],
	}
}

func (v *VolcanoBatchScheduler) syncPodGroup(cluster *v1beta1.FlinkCluster, size int32, minResource corev1.ResourceList) error {
	var err error
	podGroupName := v.getPodGroupName(cluster)
	if pg, err := v.volcanoClient.SchedulingV1beta1().PodGroups(cluster.Namespace).Get(context.TODO(), podGroupName, metav1.GetOptions{}); err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
		pg := scheduling.PodGroup{
			ObjectMeta: metav1.ObjectMeta{
				Namespace:       cluster.Namespace,
				Name:            podGroupName,
				OwnerReferences: []metav1.OwnerReference{newOwnerReference(cluster)},
			},
			Spec: scheduling.PodGroupSpec{
				MinMember:    size,
				MinResources: &minResource,
			},
		}

		_, err = v.volcanoClient.SchedulingV1beta1().PodGroups(pg.Namespace).Create(context.TODO(), &pg, metav1.CreateOptions{})
	} else {
		if pg.Spec.MinMember != size {
			pg.Spec.MinMember = size
			_, err = v.volcanoClient.SchedulingV1beta1().PodGroups(pg.Namespace).Update(context.TODO(), pg, metav1.UpdateOptions{})
		}
	}
	if err != nil {
		return fmt.Errorf("failed to sync PodGroup with error: %s. Abandon schedule pods via volcano", err)
	}
	return nil
}

func getClusterResource(state *model.DesiredClusterState) (corev1.ResourceList, int32) {
	resource := corev1.ResourceList{}
	var size int32

	if state.JmStatefulSet != nil {
		size += *state.JmStatefulSet.Spec.Replicas
		for i := int32(0); i < *state.JmStatefulSet.Spec.Replicas; i++ {
			jmResource := getPodResource(&state.JmStatefulSet.Spec.Template.Spec)
			addResourceList(resource, jmResource, nil)
		}
	}

	if state.TmStatefulSet != nil {
		size += *state.TmStatefulSet.Spec.Replicas
		for i := int32(0); i < *state.TmStatefulSet.Spec.Replicas; i++ {
			tmResource := getPodResource(&state.TmStatefulSet.Spec.Template.Spec)
			addResourceList(resource, tmResource, nil)
		}
	}

	return resource, size
}

func getPodResource(spec *corev1.PodSpec) corev1.ResourceList {
	resource := corev1.ResourceList{}
	for _, container := range spec.Containers {
		addResourceList(resource, container.Resources.Requests, container.Resources.Limits)
	}
	return resource
}

func addResourceList(list, req, limit corev1.ResourceList) {
	for name, quantity := range req {

		if value, ok := list[name]; !ok {
			list[name] = quantity.DeepCopy()
		} else {
			value.Add(quantity)
			list[name] = value
		}
	}

	// If Requests is omitted for a container,
	// it defaults to Limits if that is explicitly specified.
	for name, quantity := range limit {
		if _, ok := list[name]; !ok {
			list[name] = quantity.DeepCopy()
		}
	}
}
