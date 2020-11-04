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
	"errors"
	"fmt"
	"github.com/go-logr/logr"
	v1beta1 "github.com/googlecloudplatform/flink-operator/api/v1beta1"
	"github.com/googlecloudplatform/flink-operator/controllers/flinkclient"
	"github.com/googlecloudplatform/flink-operator/controllers/history"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strings"
	"time"
)

// ClusterStateObserver gets the observed state of the cluster.
type ClusterStateObserver struct {
	k8sClient   client.Client
	flinkClient flinkclient.FlinkClient
	request     ctrl.Request
	context     context.Context
	log         logr.Logger
	history     history.Interface
}

// ObservedClusterState holds observed state of a cluster.
type ObservedClusterState struct {
	cluster            *v1beta1.FlinkCluster
	revisions          []*appsv1.ControllerRevision
	configMap          *corev1.ConfigMap
	jmDeployment       *appsv1.Deployment
	jmService          *corev1.Service
	jmIngress          *extensionsv1beta1.Ingress
	tmDeployment       *appsv1.Deployment
	job                *batchv1.Job
	flinkJobList       *flinkclient.JobStatusList
	flinkRunningJobIDs []string
	flinkJobID         *string
	savepoint          *flinkclient.SavepointStatus
	revisionStatus     *RevisionStatus
	savepointErr       error
	observeTime        time.Time
}

// Observes the state of the cluster and its components.
// NOT_FOUND error is ignored because it is normal, other errors are returned.
func (observer *ClusterStateObserver) observe(
	observed *ObservedClusterState) error {
	var err error
	var log = observer.log

	// Cluster state.
	var observedCluster = new(v1beta1.FlinkCluster)
	err = observer.observeCluster(observedCluster)
	if err != nil {
		if client.IgnoreNotFound(err) != nil {
			log.Error(err, "Failed to get the cluster resource")
			return err
		}
		log.Info("Observed cluster", "cluster", "nil")
		observedCluster = nil
	} else {
		log.Info("Observed cluster", "cluster", *observedCluster)
		observed.cluster = observedCluster
	}

	// Revisions.
	var observedRevisions []*appsv1.ControllerRevision
	err = observer.observeRevisions(&observedRevisions, observedCluster)
	if err != nil {
		if client.IgnoreNotFound(err) != nil {
			log.Error(err, "Failed to get the controllerRevision resource list")
			return err
		}
		log.Info("Observed controllerRevisions", "controllerRevisions", "nil")
	} else {
		var b strings.Builder
		for _, cr := range observedRevisions {
			fmt.Fprintf(&b, "{name: %v, revision: %v},", cr.Name, cr.Revision)
		}
		log.Info("Observed controllerRevisions", "controllerRevisions", fmt.Sprintf("[%v]", b.String()))
		observed.revisions = observedRevisions
	}

	// ConfigMap.
	var observedConfigMap = new(corev1.ConfigMap)
	err = observer.observeConfigMap(observedConfigMap)
	if err != nil {
		if client.IgnoreNotFound(err) != nil {
			log.Error(err, "Failed to get configMap")
			return err
		}
		log.Info("Observed configMap", "state", "nil")
		observedConfigMap = nil
	} else {
		log.Info("Observed configMap", "state", *observedConfigMap)
		observed.configMap = observedConfigMap
	}

	// JobManager deployment.
	var observedJmDeployment = new(appsv1.Deployment)
	err = observer.observeJobManagerDeployment(observedJmDeployment)
	if err != nil {
		if client.IgnoreNotFound(err) != nil {
			log.Error(err, "Failed to get JobManager deployment")
			return err
		}
		log.Info("Observed JobManager deployment", "state", "nil")
		observedJmDeployment = nil
	} else {
		log.Info("Observed JobManager deployment", "state", *observedJmDeployment)
		observed.jmDeployment = observedJmDeployment
	}

	// JobManager service.
	var observedJmService = new(corev1.Service)
	err = observer.observeJobManagerService(observedJmService)
	if err != nil {
		if client.IgnoreNotFound(err) != nil {
			log.Error(err, "Failed to get JobManager service")
			return err
		}
		log.Info("Observed JobManager service", "state", "nil")
		observedJmService = nil
	} else {
		log.Info("Observed JobManager service", "state", *observedJmService)
		observed.jmService = observedJmService
	}

	// (Optional) JobManager ingress.
	var observedJmIngress = new(extensionsv1beta1.Ingress)
	err = observer.observeJobManagerIngress(observedJmIngress)
	if err != nil {
		if client.IgnoreNotFound(err) != nil {
			log.Error(err, "Failed to get JobManager ingress")
			return err
		}
		log.Info("Observed JobManager ingress", "state", "nil")
		observedJmIngress = nil
	} else {
		log.Info("Observed JobManager ingress", "state", *observedJmIngress)
		observed.jmIngress = observedJmIngress
	}

	// TaskManager deployment.
	var observedTmDeployment = new(appsv1.Deployment)
	err = observer.observeTaskManagerDeployment(observedTmDeployment)
	if err != nil {
		if client.IgnoreNotFound(err) != nil {
			log.Error(err, "Failed to get TaskManager deployment")
			return err
		}
		log.Info("Observed TaskManager deployment", "state", "nil")
		observedTmDeployment = nil
	} else {
		log.Info("Observed TaskManager deployment", "state", *observedTmDeployment)
		observed.tmDeployment = observedTmDeployment
	}

	// (Optional) Savepoint.
	// Savepoint observe error do not affect deploy reconciliation loop.
	observer.observeSavepoint(observed)

	// (Optional) job.
	err = observer.observeJob(observed)

	observed.observeTime = time.Now()

	return err
}

func (observer *ClusterStateObserver) observeJob(
	observed *ObservedClusterState) error {
	var err error
	var log = observer.log

	// Either the cluster has been deleted or it is a session cluster.
	if observed.cluster == nil || observed.cluster.Spec.Job == nil {
		return nil
	}

	// Flink job status list can be available before there is any job
	// submitted.
	observer.observeFlinkJobs(observed)

	// Job resource.
	var observedJob = new(batchv1.Job)
	err = observer.observeJobResource(observedJob)
	if err != nil {
		if client.IgnoreNotFound(err) != nil {
			log.Error(err, "Failed to get job")
			return err
		}
		log.Info("Observed job", "state", "nil")
		observedJob = nil
	} else {
		log.Info("Observed job", "state", *observedJob)
		observed.job = observedJob
	}

	return nil
}

// Observes Flink jobs through Flink API (instead of Kubernetes jobs through
// Kubernetes API).
//
// This needs to be done after the cluster is running and before the job is
// submitted, because we use it to detect whether the Flink API server is up
// and running.
func (observer *ClusterStateObserver) observeFlinkJobs(
	observed *ObservedClusterState) {
	var log = observer.log

	// Wait until the cluster is running.
	if observed.cluster.Status.State !=
		v1beta1.ClusterStateRunning {
		log.Info(
			"Skip getting Flink job status.",
			"clusterState",
			observed.cluster.Status.State)
		return
	}

	// Get Flink job status list.
	var flinkAPIBaseURL = getFlinkAPIBaseURL(observed.cluster)
	var jobList = &flinkclient.JobStatusList{}
	var err = observer.flinkClient.GetJobStatusList(flinkAPIBaseURL, jobList)
	if err != nil {
		// It is normal in many cases, not an error.
		log.Info("Failed to get Flink job status list.", "error", err)
		jobList = nil
	}
	observed.flinkJobList = jobList

	if jobList == nil {
		return
	}

	log.Info("Observed Flink job status list", "jobs", jobList.Jobs)

	// Get running jobs.
	for _, job := range jobList.Jobs {
		if job.Status == "RUNNING" {
			observed.flinkRunningJobIDs =
				append(observed.flinkRunningJobIDs, job.ID)
		}
	}

	// Extract Flink job ID.
	// It is okay if there are multiple jobs, but at most one of them is
	// expected to be running. This is typically caused by job client
	// timed out and exited but the job submission was actually
	// successfully. When retrying, it first cancels the existing running
	// job which it has lost track of, then submit the job again.
	var flinkJobID *string
	if len(observed.flinkRunningJobIDs) > 1 {
		log.Error(
			errors.New("more than one running job were found"),
			"", "jobs", observed.flinkRunningJobIDs)
	} else if len(observed.flinkRunningJobIDs) == 1 {
		flinkJobID = &observed.flinkRunningJobIDs[0]
	} else if len(jobList.Jobs) > 1 {
		log.Error(
			errors.New("more than one non-running job were found"),
			"",
			"jobs",
			jobList.Jobs)
	} else if len(jobList.Jobs) == 1 {
		flinkJobID = &jobList.Jobs[0].ID
	}
	observed.flinkJobID = flinkJobID
	if flinkJobID != nil {
		log.Info("Observed Flink job ID", "ID", *flinkJobID)
	}
}

func (observer *ClusterStateObserver) observeSavepoint(observed *ObservedClusterState) error {
	var log = observer.log

	if observed.cluster == nil {
		return nil
	}

	// Get savepoint status in progress.
	var savepointStatus = observed.cluster.Status.Savepoint
	if savepointStatus != nil && savepointStatus.State == v1beta1.SavepointStateInProgress {
		var flinkAPIBaseURL = getFlinkAPIBaseURL(observed.cluster)
		var jobID = savepointStatus.JobID
		var triggerID = savepointStatus.TriggerID
		var savepoint flinkclient.SavepointStatus
		var err error

		savepoint, err = observer.flinkClient.GetSavepointStatus(flinkAPIBaseURL, jobID, triggerID)
		observed.savepoint = &savepoint
		if err == nil && len(savepoint.FailureCause.StackTrace) > 0 {
			err = fmt.Errorf("%s", savepoint.FailureCause.StackTrace)
		}
		if err != nil {
			observed.savepointErr = err
			log.Info("Failed to get savepoint.", "error", err, "jobID", jobID, "triggerID", triggerID)
		}
		return err
	}
	return nil
}

func (observer *ClusterStateObserver) observeCluster(
	cluster *v1beta1.FlinkCluster) error {
	return observer.k8sClient.Get(
		observer.context, observer.request.NamespacedName, cluster)
}

func (observer *ClusterStateObserver) observeRevisions(
	revisions *[]*appsv1.ControllerRevision,
	cluster *v1beta1.FlinkCluster) error {
	if cluster == nil {
		return nil
	}
	selector := labels.SelectorFromSet(labels.Set(map[string]string{history.ControllerRevisionManagedByLabel: cluster.GetName()}))
	controllerRevisions, err := observer.history.ListControllerRevisions(cluster, selector)
	*revisions = append(*revisions, controllerRevisions...)

	return err
}

func (observer *ClusterStateObserver) observeConfigMap(
	observedConfigMap *corev1.ConfigMap) error {
	var clusterNamespace = observer.request.Namespace
	var clusterName = observer.request.Name

	return observer.k8sClient.Get(
		observer.context,
		types.NamespacedName{
			Namespace: clusterNamespace,
			Name:      getConfigMapName(clusterName),
		},
		observedConfigMap)
}

func (observer *ClusterStateObserver) observeJobManagerDeployment(
	observedDeployment *appsv1.Deployment) error {
	var clusterNamespace = observer.request.Namespace
	var clusterName = observer.request.Name
	var jmDeploymentName = getJobManagerDeploymentName(clusterName)
	return observer.observeDeployment(
		clusterNamespace, jmDeploymentName, "JobManager", observedDeployment)
}

func (observer *ClusterStateObserver) observeTaskManagerDeployment(
	observedDeployment *appsv1.Deployment) error {
	var clusterNamespace = observer.request.Namespace
	var clusterName = observer.request.Name
	var tmDeploymentName = getTaskManagerDeploymentName(clusterName)
	return observer.observeDeployment(
		clusterNamespace, tmDeploymentName, "TaskManager", observedDeployment)
}

func (observer *ClusterStateObserver) observeDeployment(
	namespace string,
	name string,
	component string,
	observedDeployment *appsv1.Deployment) error {
	var log = observer.log.WithValues("component", component)
	var err = observer.k8sClient.Get(
		observer.context,
		types.NamespacedName{
			Namespace: namespace,
			Name:      name,
		},
		observedDeployment)
	if err != nil {
		if client.IgnoreNotFound(err) != nil {
			log.Error(err, "Failed to get deployment")
		} else {
			log.Info("Deployment not found")
		}
	}
	return err
}

func (observer *ClusterStateObserver) observeJobManagerService(
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

func (observer *ClusterStateObserver) observeJobManagerIngress(
	observedIngress *extensionsv1beta1.Ingress) error {
	var clusterNamespace = observer.request.Namespace
	var clusterName = observer.request.Name

	return observer.k8sClient.Get(
		observer.context,
		types.NamespacedName{
			Namespace: clusterNamespace,
			Name:      getJobManagerIngressName(clusterName),
		},
		observedIngress)
}

func (observer *ClusterStateObserver) observeJobResource(
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

type RevisionStatus struct {
	currentRevision *appsv1.ControllerRevision
	nextRevision    *appsv1.ControllerRevision
	collisionCount  int32
}

// syncRevisionStatus synchronizes current FlinkCluster resource and its child ControllerRevision resources.
// When FlinkCluster resource is edited, the operator creates new child ControllerRevision for it
// and updates nextRevision in FlinkClusterStatus to the name of the new ControllerRevision.
// At that time, the name of the ControllerRevision is composed with the hash string generated
// from the FlinkClusterSpec which is to be stored in it.
// Therefore the contents of the ControllerRevision resources are maintained not duplicate.
// If edited FlinkClusterSpec is the same with the content of any existing ControllerRevision resources,
// the operator will only update nextRevision of the FlinkClusterStatus to the name of the ControllerRevision
// that has the same content, instead of creating new ControllerRevision.
// Finally, it maintains the number of child ControllerRevision resources according to RevisionHistoryLimit.
func (observer *ClusterStateObserver) syncRevisionStatus(observed *ObservedClusterState) error {
	if observed.cluster == nil {
		return nil
	}

	var revisions = observed.revisions
	var cluster = observed.cluster
	var recordedStatus = cluster.Status
	var currentRevision, nextRevision *appsv1.ControllerRevision
	var controllerHistory = observer.history
	var revisionStatus = observed.revisionStatus

	revisionCount := len(revisions)
	history.SortControllerRevisions(revisions)

	// Use a local copy of cluster.Status.CollisionCount to avoid modifying cluster.Status directly.
	var collisionCount int32
	if recordedStatus.CollisionCount != nil {
		collisionCount = *recordedStatus.CollisionCount
	}

	// create a new revision from the current cluster
	nextRevision, err := newRevision(cluster, getNextRevisionNumber(revisions), &collisionCount)
	if err != nil {
		return err
	}

	// find any equivalent revisions
	equalRevisions := history.FindEqualRevisions(revisions, nextRevision)
	equalCount := len(equalRevisions)
	if equalCount > 0 && history.EqualRevision(revisions[revisionCount-1], equalRevisions[equalCount-1]) {
		// if the equivalent revision is immediately prior the next revision has not changed
		nextRevision = revisions[revisionCount-1]
	} else if equalCount > 0 {
		// if the equivalent revision is not immediately prior we will roll back by incrementing the
		// Revision of the equivalent revision
		nextRevision, err = controllerHistory.UpdateControllerRevision(
			equalRevisions[equalCount-1],
			nextRevision.Revision)
		if err != nil {
			return err
		}
	} else {
		//if there is no equivalent revision we create a new one
		nextRevision, err = controllerHistory.CreateControllerRevision(cluster, nextRevision, &collisionCount)
		if err != nil {
			return err
		}
	}

	// if the current revision is nil we initialize the history by setting it to the next revision
	if recordedStatus.CurrentRevision == "" {
		currentRevision = nextRevision
		// attempt to find the revision that corresponds to the current revision
	} else {
		for i := range revisions {
			if revisions[i].Name == getCurrentRevisionName(recordedStatus) {
				currentRevision = revisions[i]
				break
			}
		}
	}
	if currentRevision == nil {
		return fmt.Errorf("current ControlRevision resoucre not found")
	}

	// update revision status
	revisionStatus = new(RevisionStatus)
	revisionStatus.currentRevision = currentRevision.DeepCopy()
	revisionStatus.nextRevision = nextRevision.DeepCopy()
	revisionStatus.collisionCount = collisionCount
	observed.revisionStatus = revisionStatus

	// maintain the revision history limit
	err = observer.truncateHistory(observed)
	if err != nil {
		return err
	}

	return nil
}

func (observer *ClusterStateObserver) truncateHistory(observed *ObservedClusterState) error {
	var cluster = observed.cluster
	var revisions = observed.revisions
	// TODO: default limit
	var historyLimit int
	if cluster.Spec.RevisionHistoryLimit != nil {
		historyLimit = int(*cluster.Spec.RevisionHistoryLimit)
	} else {
		historyLimit = 10
	}

	nonLiveHistory := getNonLiveHistory(revisions, historyLimit)

	// delete any non-live history to maintain the revision limit.
	for i := 0; i < len(nonLiveHistory); i++ {
		if err := observer.history.DeleteControllerRevision(nonLiveHistory[i]); err != nil {
			return err
		}
	}
	return nil
}
