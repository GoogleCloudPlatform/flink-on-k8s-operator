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
	"fmt"
	"github.com/go-logr/logr"
	v1beta1 "github.com/googlecloudplatform/flink-operator/api/v1beta1"
	"github.com/googlecloudplatform/flink-operator/controllers/flinkclient"
	"github.com/googlecloudplatform/flink-operator/controllers/history"
	yaml "gopkg.in/yaml.v2"
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
	cluster           *v1beta1.FlinkCluster
	revisions         []*appsv1.ControllerRevision
	configMap         *corev1.ConfigMap
	jmStatefulSet     *appsv1.StatefulSet
	jmService         *corev1.Service
	jmIngress         *extensionsv1beta1.Ingress
	tmStatefulSet     *appsv1.StatefulSet
	flinkJob          FlinkJob
	flinkJobSubmitter FlinkJobSubmitter
	savepoint         Savepoint
	revision          Revision
	observeTime       time.Time
	updateState       UpdateState
}

type FlinkJob struct {
	status     *flinkclient.JobStatus
	list       *flinkclient.JobStatusList
	unexpected []string
}

type FlinkJobSubmitter struct {
	job *batchv1.Job
	pod *corev1.Pod
	log *SubmitterLog
}

type SubmitterLog struct {
	JobID   string `yaml:"jobID,omitempty"`
	Message string `yaml:"message"`
}

type Savepoint struct {
	status *flinkclient.SavepointStatus
	error  error
}

type Revision struct {
	currentRevision *appsv1.ControllerRevision
	nextRevision    *appsv1.ControllerRevision
	collisionCount  int32
}

func (o *ObservedClusterState) isClusterUpdating() bool {
	return o.updateState == UpdateStateInProgress
}

// Job submitter status.
func (s *FlinkJobSubmitter) getState() JobSubmitState {
	switch {
	case s.job == nil:
		break
	case s.job.Status.Succeeded == 0 && s.job.Status.Failed == 0:
		return JobDeployStateInProgress
	case s.job.Status.Failed > 0:
		return JobDeployStateFailed
	case s.job.Status.Succeeded > 0:
		if s.log != nil && s.log.JobID != "" {
			return JobDeployStateSucceeded
		}
	}
	return JobDeployStateUnknown
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

	// JobManager StatefulSet.
	var observedJmStatefulSet = new(appsv1.StatefulSet)
	err = observer.observeJobManagerStatefulSet(observedJmStatefulSet)
	if err != nil {
		if client.IgnoreNotFound(err) != nil {
			log.Error(err, "Failed to get JobManager StatefulSet")
			return err
		}
		log.Info("Observed JobManager StatefulSet", "state", "nil")
		observedJmStatefulSet = nil
	} else {
		log.Info("Observed JobManager StatefulSet", "state", *observedJmStatefulSet)
		observed.jmStatefulSet = observedJmStatefulSet
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

	// TaskManager StatefulSet.
	var observedTmStatefulSet = new(appsv1.StatefulSet)
	err = observer.observeTaskManagerStatefulSet(observedTmStatefulSet)
	if err != nil {
		if client.IgnoreNotFound(err) != nil {
			log.Error(err, "Failed to get TaskManager StatefulSet")
			return err
		}
		log.Info("Observed TaskManager StatefulSet", "state", "nil")
		observedTmStatefulSet = nil
	} else {
		log.Info("Observed TaskManager StatefulSet", "state", *observedTmStatefulSet)
		observed.tmStatefulSet = observedTmStatefulSet
	}

	// (Optional) Savepoint.
	var observedSavepoint Savepoint
	err = observer.observeSavepoint(observed.cluster, &observedSavepoint)
	if err != nil {
		log.Error(err, "Failed to get Flink job savepoint status")
	} else {
		log.Info("Observed Flink job savepoint status", "status", observedSavepoint.status)
	}
	observed.savepoint = observedSavepoint

	// (Optional) job.
	err = observer.observeJob(observed)
	if err != nil {
		log.Error(err, "Failed to get Flink job status")
		return err
	}

	observed.observeTime = time.Now()
	observed.updateState = getUpdateState(observed)

	return nil
}

func (observer *ClusterStateObserver) observeJob(
	observed *ObservedClusterState) error {
	// Either the cluster has been deleted or it is a session cluster.
	if observed.cluster == nil || observed.cluster.Spec.Job == nil {
		return nil
	}
	var log = observer.log
	var recorded = observed.cluster.Status
	var err error

	// Observe the Flink job submitter.
	var submitter FlinkJobSubmitter
	err = observer.observeSubmitter(&submitter)
	if err != nil {
		log.Error(err, "Failed to get the status of the job submitter")
	}
	observed.flinkJobSubmitter = submitter

	// Observe the Flink job status.
	var flinkJobID string
	// Get the ID from the job submitter.
	if submitter.log != nil && submitter.log.JobID != "" {
		flinkJobID = submitter.log.JobID
	} else
	// Or get the job ID from the recorded job status which is written in previous iteration.
	if recorded.Components.Job != nil {
		flinkJobID = recorded.Components.Job.ID
	}
	var observedFlinkJob FlinkJob
	observer.observeFlinkJobStatus(observed, flinkJobID, &observedFlinkJob)
	observed.flinkJob = observedFlinkJob

	return nil
}

func (observer *ClusterStateObserver) observeSubmitter(submitter *FlinkJobSubmitter) error {
	var log = observer.log
	var err error

	// Observe following
	var job *batchv1.Job
	var pod *corev1.Pod
	var podLog *SubmitterLog

	// Job resource.
	job = new(batchv1.Job)
	err = observer.observeSubmitterJob(job)
	if err != nil {
		if client.IgnoreNotFound(err) != nil {
			log.Error(err, "Failed to get the submitter job")
			return err
		}
		log.Info("Observed submitter job", "state", "nil")
		job = nil
	} else {
		log.Info("Observed submitter job", "state", *job)
	}
	submitter.job = job

	// Get the job submission log.
	// When the recorded job state is pending or updating, and the actual submission is completed,
	// extract the job submission log from the pod termination log.
	if submitter.job == nil {
		return nil
	}
	// Get job submitter pod resource.
	pod = new(corev1.Pod)
	err = observer.observeJobSubmitterPod(pod)
	if err != nil {
		log.Error(err, "Failed to get the submitter pod")
		return err
	} else if pod == nil {
		log.Info("Observed submitter job pod", "state", "nil")
		return nil
	} else {
		log.Info("Observed submitter job pod", "state", *pod)
	}
	submitter.pod = pod

	// Extract submission result.
	var jobSubmissionCompleted = job.Status.Succeeded > 0 || job.Status.Failed > 0
	if !jobSubmissionCompleted {
		return nil
	}
	log.Info("Extracting the result of job submission because it is completed")
	podLog = new(SubmitterLog)
	err = observer.observeFlinkJobSubmitterLog(pod, podLog)
	if err != nil {
		log.Error(err, "Failed to extract the job submission result")
		podLog = nil
	} else if podLog == nil {
		log.Info("Observed submitter log", "state", "nil")
	} else {
		log.Info("Observed submitter log", "state", *podLog)
	}
	submitter.log = podLog

	return nil
}

// Observes Flink job status through Flink API (instead of Kubernetes jobs through
// Kubernetes API).
//
// This needs to be done after the job manager is ready, because we use it to detect whether the Flink API server is up
// and running.
func (observer *ClusterStateObserver) observeFlinkJobStatus(
	observed *ObservedClusterState,
	flinkJobID string,
	flinkJob *FlinkJob) {
	var log = observer.log

	// Observe following
	var flinkJobStatus *flinkclient.JobStatus
	var flinkJobList *flinkclient.JobStatusList
	var flinkJobsUnexpected []string

	// Wait until the job manager is ready.
	var jmReady = observed.jmStatefulSet != nil && getStatefulSetState(observed.jmStatefulSet) == v1beta1.ComponentStateReady
	if !jmReady {
		log.Info(
			"Skip getting Flink job status.",
			"job manager ready", jmReady)
		return
	}

	// Get Flink job status list.
	flinkJobList = &flinkclient.JobStatusList{}
	var flinkAPIBaseURL = getFlinkAPIBaseURL(observed.cluster)
	var err = observer.flinkClient.GetJobStatusList(flinkAPIBaseURL, flinkJobList)
	if err != nil {
		// It is normal in many cases, not an error.
		log.Info("Failed to get Flink job status list.", "error", err)
		return
	}
	flinkJob.list = flinkJobList

	// Extract the current job status and unexpected jobs.
	for _, job := range flinkJobList.Jobs {
		if flinkJobID == job.ID {
			flinkJobStatus = &job
		} else if getFlinkJobDeploymentState(job.Status) == v1beta1.JobStateRunning {
			flinkJobsUnexpected = append(flinkJobsUnexpected, job.ID)
		}
	}
	flinkJob.status = flinkJobStatus
	flinkJob.unexpected = flinkJobsUnexpected
	log.Info("Observed Flink job",
		"submitted job status", flinkJob.status,
		"all job list", flinkJob.list,
		"unexpected job list", flinkJob.unexpected)
	if len(flinkJobsUnexpected) > 0 {
		log.Info("More than one unexpected Flink job were found!")
	}

	return
}

func (observer *ClusterStateObserver) observeSavepoint(cluster *v1beta1.FlinkCluster, savepoint *Savepoint) error {
	if cluster == nil ||
		cluster.Status.Savepoint == nil ||
		cluster.Status.Savepoint.State != v1beta1.SavepointStateInProgress {
		return nil
	}

	// Get savepoint status in progress.
	var flinkAPIBaseURL = getFlinkAPIBaseURL(cluster)
	var recordedSavepoint = cluster.Status.Savepoint
	var jobID = recordedSavepoint.JobID
	var triggerID = recordedSavepoint.TriggerID

	savepointStatus, err := observer.flinkClient.GetSavepointStatus(flinkAPIBaseURL, jobID, triggerID)
	savepoint.status = &savepointStatus
	savepoint.error = err

	return err
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

func (observer *ClusterStateObserver) observeJobManagerStatefulSet(
	observedStatefulSet *appsv1.StatefulSet) error {
	var clusterNamespace = observer.request.Namespace
	var clusterName = observer.request.Name
	var jmStatefulSetName = getJobManagerStatefulSetName(clusterName)
	return observer.observeStatefulSet(
		clusterNamespace, jmStatefulSetName, "JobManager", observedStatefulSet)
}

func (observer *ClusterStateObserver) observeTaskManagerStatefulSet(
	observedStatefulSet *appsv1.StatefulSet) error {
	var clusterNamespace = observer.request.Namespace
	var clusterName = observer.request.Name
	var tmStatefulSetName = getTaskManagerStatefulSetName(clusterName)
	return observer.observeStatefulSet(
		clusterNamespace, tmStatefulSetName, "TaskManager", observedStatefulSet)
}

func (observer *ClusterStateObserver) observeStatefulSet(
	namespace string,
	name string,
	component string,
	observedStatefulSet *appsv1.StatefulSet) error {
	var log = observer.log.WithValues("component", component)
	var err = observer.k8sClient.Get(
		observer.context,
		types.NamespacedName{
			Namespace: namespace,
			Name:      name,
		},
		observedStatefulSet)
	if err != nil {
		if client.IgnoreNotFound(err) != nil {
			log.Error(err, "Failed to get StatefulSet")
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

func (observer *ClusterStateObserver) observeSubmitterJob(
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

// observeJobSubmitterPod observes job submitter pod.
func (observer *ClusterStateObserver) observeJobSubmitterPod(
	observedPod *corev1.Pod) error {
	var clusterNamespace = observer.request.Namespace
	var clusterName = observer.request.Name
	var podSelector = labels.SelectorFromSet(map[string]string{"job-name": getJobName(clusterName)})
	var podList = new(corev1.PodList)

	var err = observer.k8sClient.List(
		observer.context,
		podList,
		client.InNamespace(clusterNamespace),
		client.MatchingLabelsSelector{Selector: podSelector})
	if err != nil {
		return err
	}
	if len(podList.Items) == 0 {
		observedPod = nil
	} else {
		podList.Items[0].DeepCopyInto(observedPod)
	}

	return nil
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

	var cluster = observed.cluster
	var revisions = observed.revisions
	var recorded = cluster.Status
	var currentRevision, nextRevision *appsv1.ControllerRevision
	var controllerHistory = observer.history

	revisionCount := len(revisions)
	history.SortControllerRevisions(revisions)

	// Use a local copy of cluster.Status.CollisionCount to avoid modifying cluster.Status directly.
	var collisionCount int32
	if recorded.Revision.CollisionCount != nil {
		collisionCount = *recorded.Revision.CollisionCount
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
	if recorded.Revision.CurrentRevision == "" {
		currentRevision = nextRevision
		// attempt to find the revision that corresponds to the current revision
	} else {
		for i := range revisions {
			if revisions[i].Name == getCurrentRevisionName(&recorded.Revision) {
				currentRevision = revisions[i]
				break
			}
		}
	}
	if currentRevision == nil {
		return fmt.Errorf("current ControlRevision resoucre not found")
	}

	// Update revision status.
	observed.revision = Revision{
		currentRevision: currentRevision.DeepCopy(),
		nextRevision:    nextRevision.DeepCopy(),
		collisionCount:  collisionCount,
	}

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

// observeFlinkJobSubmit extract submit result from the pod termination log.
func (observer *ClusterStateObserver) observeFlinkJobSubmitterLog(observedPod *corev1.Pod, submitterLog *SubmitterLog) error {
	var log = observer.log
	var containerStatuses = observedPod.Status.ContainerStatuses
	if len(containerStatuses) == 0 ||
		containerStatuses[0].State.Terminated == nil ||
		containerStatuses[0].State.Terminated.Message == "" {
		submitterLog = nil
		log.Info("job pod found, but no termination log")
		return nil
	}

	// The job submission script writes the submission log to the pod termination log at the end of execution.
	// If the job submission is successful, the extracted job ID is also included.
	// The job submit script writes the submission result in YAML format,
	// so parse it here to get the ID - if available - and log.
	// Note: https://kubernetes.io/docs/tasks/debug-application-cluster/determine-reason-pod-failure/
	var rawJobSubmissionResult = containerStatuses[0].State.Terminated.Message
	var err = yaml.Unmarshal([]byte(rawJobSubmissionResult), submitterLog)
	return err
}
