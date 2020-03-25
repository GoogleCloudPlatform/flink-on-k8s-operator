# User Guide

This document will walk you through the steps of deploying the Flink Operator to
a Kubernetes cluster and running a sample Flink job.

## Prerequisite

First, you need to have a running Kubernetes cluster, you can verify the cluster
info with

```bash
kubectl cluster-info
```

Second, clone the Flink Operator repo to your local machine with

```bash
git clone git@github.com:GoogleCloudPlatform/flink-on-k8s-operator.git
```

then switch to the repo directory, we need to use the scripts in the repo for
deployment. (This step is not needed if you choose to install through Helm
Chart.)

To execute the install scripts, you'll also need [kubectl v1.14+](https://kubernetes.io/docs/tasks/tools/install-kubectl/)
and [go v1.12+](https://golang.org/doc/install) installed on your local machine.

## (Optional) Get a Flink Operator image

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

### Option 1: Make

Run the following command from the source repo

```bash
make deploy [IMG=<image>] [FLINK_OPERATOR_NAMESPACE=<operator-namespace>] [WATCH_NAMESPACE=<watch-namespace>]
```

* `IMG`: The Flink Operator image. The default value is `gcr.io/flink-operator/flink-operator:latest`.
* `FLINK_OPERATOR_NAMESPACE`: the namespace of the operator. The default value is
  `flink-operator-system`. It is highly recommended to use the default namespace
  unless you want to deploy multiple instances of the operator in a cluster. In
  this case, we recommend to specify a dedicated namespace for each instance.
  The deploy/undeploy process will create/delete the namespace automatically.
* `WATCH_NAMESPACE`: the namespace of the `FlinkCluster` CRs which the operator
  watches. The default value is empty string which means all namespaces. You
  might want to set this variable when you have multiple instances of the
  operator in a cluster and each handles CRs in a particular namespace.

### Option 2: Helm Chart

Follow the [Helm Chart Installation Guide](../helm-chart/flink-operator/README.md) to install the operator through Helm Chart.

After deploying the operator, you can verify CRD `flinkclusters.flinkoperator.k8s.io`
has been created with

```bash
kubectl get crds | grep flinkclusters.flinkoperator.k8s.io
```

You can also view the details of the CRD with

```bash
kubectl describe crds/flinkclusters.flinkoperator.k8s.io
```

The operator runs as a Kubernetes Deployment, and you can find out the
deployment with

```bash
kubectl get deployments -n flink-operator-system
```

or verify the operator Pod is up and running.

```bash
kubectl get pods -n flink-operator-system
```

You can also check the operator logs with

```bash
kubectl logs -n flink-operator-system -l app=flink-operator --all-containers
```

you should be able see logs like: 

```
INFO    setup   Starting manager
INFO    controller-runtime.certwatcher  Starting certificate watcher
INFO    controller-runtime.controller   Starting workers        {"controller": "flinkcluster", "worker count": 1}
```

## Deploy multiple operators handle limited namespace.

The Flink operator basically detects and processes all FlinkCluster resources
created in one kubernetes cluster. However, depending on the usage environment,
such as a multi-tenant cluster, the namespace to be managed by the operator
may need to be limited. In this case, dedicated operators must be deployed
for each namespace, and multiple operators may be deployed in one cluster.

Deploy by specifying the namespace to manage and prefix to avoid duplication
of cluster-scoped resources:

```bash
make deploy RESOURCE_PREFIX=<kuberntes-resource-name-prefix> \
            WATCH_NAMESPACE=<namespace-to-watch> \
            FLINK_OPERATOR_NAMESPACE=<namespace-to-deploy-operator>
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

NOTE: Flink Job Cluster's TaskManager will get terminated once the sample job is
completed (in this case it take around 5 minutes for the pod to terminate) 

## Submit a job

There are several ways to submit jobs to a session cluster.

1) **Flink web UI**

You can submit jobs through the Flink web UI. See instructions in the
Monitoring section on how to setup a proxy to the Flink Web UI.

2) **From within the cluster**

You can submit jobs through a client Pod in the same cluster, for example:

```bash
kubectl run my-job-submitter --image=flink:1.8.1 --generator=run-pod/v1 -- \
    /opt/flink/bin/flink run -m flinksessioncluster-sample-jobmanager:8081 \
    /opt/flink/examples/batch/WordCount.jar --input /opt/flink/README.txt
```

3) **From outside the cluster**

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

To get a list of jobs

```bash
kubectl get jobs
```

In a job cluster, the job is automatically submitted by the operator you can
check the Flink job status and logs with

```bash
kubectl describe jobs <CLUSTER-NAME>-job
kubectl logs jobs/<CLUSTER-NAME>-job -f --tail=1000
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

## Undeploy the operator

Undeploy the operator and CRDs from the Kubernetes cluster with

```
make undeploy [FLINK_OPERATOR_NAMESPACE=<namespace>]
```
