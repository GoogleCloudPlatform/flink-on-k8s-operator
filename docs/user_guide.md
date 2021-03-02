# User Guide

This document will walk you through the steps of deploying the Flink Operator to
a Kubernetes cluster and running a sample Flink job.

## Prerequisites

* Get a running Kubernetes cluster, you can verify the cluster info with

  ```bash
  kubectl cluster-info
  ```

* Clone the Flink Operator repo to your local machine:

  ```bash
  git clone git@github.com:GoogleCloudPlatform/flink-on-k8s-operator.git
  ```

  then switch to the repo directory, we need to use the scripts in the repo for
  deployment. (This step is not needed if you choose to install through Helm Chart).

  To execute the install scripts, you'll also need [kubectl v1.14+](https://kubernetes.io/docs/tasks/tools/install-kubectl/)
  and [go v1.12+](https://golang.org/doc/install) installed on your local machine.

* (Optional) Choose a Flink Operator image.

  By default, the deployment uses the image `gcr.io/flink-operator/flink-operator:latest`,
  but you can find and choose other released images by running the following command:

  ```bash
  gcloud container images list-tags gcr.io/flink-operator/flink-operator
  ```

  You can also follow the [Deverloper Guide](./developer_guide.md) to build your
  own image from the source code and push the image to a registry where your
  Kubernetes cluster can access.

## Deploy the operator to a Kubernetes cluster

You can deploy the Flink Operator to the Kubernetes cluster through one of the
following 2 ways:

* **Option 1: Make deploy**

  Simply run
  
  ```base
  make deploy
  ```
  
  from the source repo to deploy the operator. There are some flags which you
  can use to configure the deployment:

  ```bash
  make deploy
      [IMG=<operator-image>] \
      [FLINK_OPERATOR_NAMESPACE=<namespace-to-deploy-operator>] \
      [RESOURCE_PREFIX=<kuberntes-resource-name-prefix>] \
      [WATCH_NAMESPACE=<namespace-to-watch>]
  ```

  * `IMG`: The Flink Operator image. The default value is `gcr.io/flink-operator/flink-operator:latest`.
  * `FLINK_OPERATOR_NAMESPACE`: the namespace of the operator. The default value is
    `flink-operator-system`.
  * `RESOURCE_PREFIX`: the prefix to avoid conflict of cluster-scoped resources.
    The default value is `flink-operator-`.
  * `WATCH_NAMESPACE`: the namespace of the `FlinkCluster` CRs which the operator
    watches. The default value is empty string which means all namespaces.
  
  **Note:** It is highly recommended to just use the default values unless you want to
  deploy multiple instances of the operator in a cluster, see more details in the
  How-to section of this doc.

* **Option 2: Helm Chart**

  Follow the [Helm Chart Installation Guide](../helm-chart/flink-operator/README.md) to
  install the operator through Helm Chart.

Note, for kubernetes cluster with private cluster domain, you should add a CLUSTER\_DOMAIN environment to the operator deployment.

## Verify the deployment

After deploying the operator, you can verify CRD `flinkclusters.flinkoperator.k8s.io`
has been created:

```bash
kubectl get crds | grep flinkclusters.flinkoperator.k8s.io
```

View the details of the CRD:

```bash
kubectl describe crds/flinkclusters.flinkoperator.k8s.io
```

Find out the deployment:

```bash
kubectl get deployments -n flink-operator-system
```

Verify the operator Pod is up and running:

```bash
kubectl get pods -n flink-operator-system
```

Check the operator logs:

```bash
kubectl logs -n flink-operator-system -l app=flink-operator --all-containers
```

you should be able see logs like:

```
INFO    setup   Starting manager
INFO    controller-runtime.certwatcher  Starting certificate watcher
INFO    controller-runtime.controller   Starting workers        {"controller": "flinkcluster", "worker count": 1}
```

## Create a sample Flink cluster

After deploying the Flink CRDs and the Flink Operator to a Kubernetes cluster,
the operator serves as a control plane for Flink. In other words, previously the
cluster only understands the language of Kubernetes, now it understands the
language of Flink. You can then create custom resources representing Flink
session clusters or job clusters, and the operator will detect the custom
resources automatically, then create the actual clusters optionally run jobs,
and update status in the custom resources.

Create a [sample Flink session cluster](../config/samples/flinkoperator_v1beta1_flinksessioncluster.yaml)
custom resource with

```bash
kubectl apply -f config/samples/flinkoperator_v1beta1_flinksessioncluster.yaml
```

Flink will deploy Flink session cluster's Pods, Services, etc.. on `default`
namespace, and you can find out with

```bash
kubectl get pods,svc -n default
```

or verify the Pod is up and running with

```
kubectl get pods,svc -n default | grep "flinksessioncluster"
```

and a [sample Flink job cluster](../config/samples/flinkoperator_v1beta1_flinkjobcluster.yaml)

custom resource with

```bash
kubectl apply -f config/samples/flinkoperator_v1beta1_flinkjobcluster.yaml
```

and verify the pod is up and running with

```
kubectl get pods,svc -n default | grep "flinkjobcluster"
```

By default, Flink Job Cluster's TaskManager will get terminated once
the sample job is completed (in this case it takes around 5 minutes for the
Pod to terminate)

## Submit a job

There are several ways to submit jobs to a session cluster.

* **Flink web UI**

  You can submit jobs through the Flink web UI. See instructions in the
  Monitoring section on how to setup a proxy to the Flink web UI.

* **From within the cluster**

  You can submit jobs through a client Pod in the same cluster, for example:

  ```bash
  cat <<EOF | kubectl apply --filename -
  apiVersion: batch/v1
  kind: Job
  metadata:
    name: my-job-submitter
  spec:
    template:
      spec:
        containers:
        - name: wordcount
          image: flink:1.8.1
          args:
          - /opt/flink/bin/flink
          - run
          - -m
          - flinksessioncluster-sample-jobmanager:8081
          - /opt/flink/examples/batch/WordCount.jar
          - --input
          - /opt/flink/README.txt
        restartPolicy: Never
  EOF
  ```

* **From outside the cluster**

  If you have configured the access scope of JobManager as `External` or `VPC`,
  you can submit jobs from a machine which is in the scope, for example:

  ```bash
  flink run -m <jobmanager-service-ip>:8081 \
      examples/batch/WordCount.jar --input /opt/flink/README.txt
  ```

  Or if the access scope is `Cluster` which is the default, you can use port
  forwarding to establish a tunnel from a machine which has access to the
  Kubernetes API service (typically your local machine) to the JobManager service
  first, for example:

  ```bash
  kubectl port-forward service/flinksessioncluster-sample-jobmanager 8081:8081
  ```

  then submit jobs through the tunnel, for example:

  ```bash
  flink run -m localhost:8081 \
      examples/batch/WordCount.jar --input ./README.txt
  ```

## Monitoring

### Operator

You can check the operator logs with

```bash
kubectl logs -n flink-operator-system -l app=flink-operator --all-containers -f --tail=1000
```

### Flink cluster

After deploying a Flink cluster with the operator, you can find the cluster
custom resource with

```bash
kubectl get flinkclusters
```

check the cluster status with

```bash
kubectl describe flinkclusters <CLUSTER-NAME>
```

### Flink job

In a job cluster, the job is automatically submitted by the operator.
The operator creates a submitter for a Flink job.
The job submitter itself is created as a Kubernetes job.

When the job submitter starts, it first checks the status of Flink job manager.
And it submits a Flink job when confirmed that Flink job manager is ready and then terminates.

You can check the Flink job submission status and logs with

```bash
kubectl describe jobs <CLUSTER-NAME>-job-submitter
kubectl logs jobs/<CLUSTER-NAME>-job-submitter -f
```

In a session cluster, depending on how you submit the job, you can check the
job status and logs accordingly.

### Flink web UI, REST API, and CLI

You can also access the Flink web UI, [REST API](https://ci.apache.org/projects/flink/flink-docs-stable/monitoring/rest_api.html)
and [CLI](https://ci.apache.org/projects/flink/flink-docs-stable/ops/cli.html)
by first creating a port forward from you local machine to the JobManager
service UI port (8081 by default).

```bash
kubectl port-forward svc/[FLINK_CLUSTER_NAME]-jobmanager 8081:8081
```

then access the web UI with your browser through the following URL:

```bash
http://localhost:8081
```

call the Flink REST API, e.g., list jobs:

```bash
curl http://localhost:8081/jobs
```

run the Flink CLI, e.g., list jobs:

```bash
flink list -m localhost:8081
```

## Delete a Flink cluster

You can delete a Flink job or session cluster with the following command
regardless of its current status, the operator will try to take savepoint
if possible then cancel the job.

```
kubectl delete flinkclusters <name>
```

## Undeploy the operator

Undeploy the operator and CRDs from the Kubernetes cluster with

```
make undeploy [FLINK_OPERATOR_NAMESPACE=<namespace>]
```

## How-to

### Deploy multiple instances of the operator in a cluster

The Flink operator basically detects and processes all FlinkCluster resources
created in one kubernetes cluster. However, depending on the usage environment,
such as a multi-tenant cluster, the namespace to be managed by the operator
may need to be limited. In this case, dedicated operators must be deployed
for each namespace, and multiple operators may be deployed in one cluster.

Deploy by specifying the namespace to manage and prefix to avoid duplication
of cluster-scoped resources:

```bash
make deploy
    IMG=<operator-image> \
    FLINK_OPERATOR_NAMESPACE=<namespace-to-deploy-operator> \
    RESOURCE_PREFIX=<kuberntes-resource-name-prefix> \
    WATCH_NAMESPACE=<namespace-to-watch>
```

### Cancel running Flink job

If you want to cancel a running Flink job, attach control annotation to your FlinkCluster's metadata:

```
metadata:
  annotations:
    flinkclusters.flinkoperator.k8s.io/user-control: job-cancel
```

You can attach the annotation:

```bash
kubectl annotate flinkclusters <CLUSTER-NAME> flinkclusters.flinkoperator.k8s.io/user-control=job-cancel
```

When canceling, all Pods that make up the Flink cluster are basically terminated.
If you want to leave the cluster, configure spec.job.cleanupPolicy.afterJobCancelled
according to the [FlinkCluster Custom Resource Definition](./crd.md).

When job cancellation is finished, the control annotation disappears and the progress
can be checked in FlinkCluster status:

```bash
kubectl describe flinkcluster <CLUSTER-NAME>

...

Status:
  Control:
    Details:
      Job ID:        e689263060695231f62fa8b00f97b383
    Name:            job-cancel
    State:           Succeeded
    Update Time:     2020-04-03T10:04:50+09:00
```

### Monitoring with Prometheus

Flink cluster can be monitored with Prometheus in various ways. Here, we introduce the method using PodMonitor
custom resource of [Prometheus operator](https://github.com/coreos/prometheus-operator).
First, create a FlinkCluster with the metric exporter activated and its port exposed.
Next, create a PodMonitor which will be used to generate service discovery configurations and register it to Prometheus.
Exposed Flink metric port must be set as the endpoint of the PodMonitor. See the
[Prometheus API docs](https://github.com/coreos/prometheus-operator/blob/master/Documentation/api.md) for details.

You can create Prometheus metric exporter activated [FlinkCluster](../examples/prometheus/flink_metric_cluster.yaml)
and [PodMonitor](../examples/prometheus/pod-monitor.yaml) like this.

```bash
kubectl apply -f examples/prometheus/flink_metric_cluster.yaml
kubectl apply -f examples/prometheus/pod-monitor.yaml
```

If the service discovery configuration is generated and registered successfully by the Promethues operator,
you can see the item named "flink-pod-monitor" in the "Service Discovery" section of your Prometheus Web UI.
(`http://<Your-Prometheus-Web-UI-base-URL>/service-discovery`)

### Manage savepoints

See this [doc](./savepoints_guide.md) on how to manage savepoints with the operator.

### Update Flink clusters and jobs

To update a running Flink job's program or execution settings, you can create new savepoint, terminate the job,
and create a new FlinkCluster with the new program, settings, and savepoint. 
However, in some cases, you may want to update the Flink job while maintaining the logical continuity
of the Flink job with the FlinkCluster custom resource. In this case, you can continuously update
the FlinkCluster custom resource, and the Flink operator takes care of the process required to update
the Flink job and cluster.

There are several points to note when using the job update feature.
* To use the job update feature, `savepointsDir` must be set and the value of this field cannot be deleted when updating.
This is because the Flink operator requires it to create a savepoint for job updates.
* You can resume Flink job from your desired savepoint by updating `fromSavepoint`.
If you want to resume the updated job from the latest savepoint, `fromSavepoint` must be unspecified.
* `cancelRequested` and `savepointGeneration` are not allowed to update at the same time with other fields
due to functional characteristics.

There are some behavioral characteristics in update.
* Whenever the FlinkCluster spec is updated, the Flink operator creates
a [ControllerRevision](https://godoc.org/k8s.io/api/apps/v1#ControllerRevision) resource
that stores the changed spec. ControllerRevisions can be used to check the editing history.
* If update is triggered while the cluster is running, all components are re-created after terminated.
If update is triggered in terminated state, all components are re-created as well.
* When job is to be updated, the Flink operator will restore the job from the latest savepoint available
- `savepointLocation` or `fromSavepoint` in job status.
If those are not available, the job is restarted from the beginning.

For example, you can create [wordcount job v1.9.2](../examples/update/wordcount-1.9.2.yaml)
and update it to [wordcount job v1.9.3](../examples/update/wordcount-1.9.3.yaml) like this.

```bash
kubectl apply -f examples/update/wordcount-1.9.2.yaml
kubectl apply -f examples/update/wordcount-1.9.3.yaml
```

In this example, Flink cluster image, job jar and job arguments are updated and the task manager is scaled from 1 to 2.
You can check the list of revisions and their contents like this:

```bash
kubectl get controllerrevision
kubectl get controllerrevision <REVISION-NAME> -o yaml
```
### Scale A Flink Job

The operator supports scaling up and down of a Job Cluster by exposing a 
[scale subresource](https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/#scale-subresource).
This allows the number of taskmanagers to be scaled either manually, or by using an something such as a horizontal pod 
autoscaler (HPA).

To enable scaling of a job, it is also neccessary to allow for the parallelism of the job to be scalable. To enable this
there is an alternative way of specifying `parallelism` of a Job using `paralleismPerTaskManager`:
```bash
job:
  paralleismPerTaskManager: 1
```
A sensible default for this would be to set it to be equal to the number of cpu available to your TaskManager, but the 
perfect value will depend on your own job.

**Note**: `parallelism` and `paralleismPerTaskManager` are mutually exclusive, you cannot set both.



### Control Logging Behavior

The default logging configuration provided by the operator sends logs from JobManager and TaskManager to `stdout`. This
has the effect of making it so that logging from Flink workloads running on Kubernetes behaves like every other 
Kubernetes pod. Your Flink logs should be stored wherever you generally expect to see your container logs in your 
environment.

Sometimes, however, this is not a good fit. An example of when you might want to customize logging behavior is to 
restore the visibility of logs in the Flink JobManager web interface. Or you might want to ship logs directly to a 
different sink, or using a different formatter. 

You can use the `spec.logConfig` field to fully control the log4j and logback configuration. It is a string-to-string map,
whose keys and values become filenames and contents (respectively) in the folder `/opt/flink/conf` in each container. 
The default Flink docker entrypoint expects this directory to contain two files: `log4j-console.properties` and 
`logback-console.xml`.

An example of using this parameter to make logs visible in both the Flink UI and on stdout 
[can be found here](../examples/log_config.yaml).

### Control Security and Permissions in Pods
You can set various security-related attributes of the JobManager, TaskManager, and Job Pods using a 
[PodSecurityContext](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.19/#podsecuritycontext-v1-core)
object. It is possible to run the entrypoint of the container process as a different user or group,
and to modify ownership of mounted volumes. 

You can set the SecurityContext in the FlinkCluster spec, within the JobManager, TaskManager, and Job fields, like this: 
```yaml
taskManager:
  ...
  securityContext:
    runAsUser: 9999
    runAsGroup: 1000
    fsGroup: 2000
```
You can set different SecurityContexts for the TaskManager, JobManager StatefulSets and the Job, but all TaskManager pods
will share the same one.
Examples and explanations of the available options can be found [here](https://kubernetes.io/docs/tasks/configure-pod-container/security-context/#set-the-security-context-for-a-pod).

### Mounting external volumes to the pods
If your deployment requires larger storage captivity, or a faster access to the state backend you can use `volumeClaimTemplates` option in TaskManager config
to create a new claim template and then mount it in `volumeMounts`  
Check the [FlinkCluster Custom Resource Definition](./crd.md) and [StatefulSet's doc](https://kubernetes.io/docs/concepts/workloads/controllers/statefulset/) for more info

