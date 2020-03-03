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
deployment. (This step will not be needed in the future after we have a Helm
chart.)

To execute the install scripts, you'll also need [kubectl v1.14+](https://kubernetes.io/docs/tasks/tools/install-kubectl/)
and [go v1.12+](https://golang.org/doc/install) installed on your local machine.

## Deploy the operator to a Kubernetes cluster

Deploy the Flink Operator to the Kubernetes cluster with [Helm Chart Installation Instruction](https://github.com/GoogleCloudPlatform/flink-on-k8s-operator/blob/master/helm-chart/flink-operator/README.md) or with

```bash
make deploy [FLINK_OPERATOR_NAMESPACE=<namespace>]
```

By default, the operator will be deployed to namespace `flink-operator-system`,
but you can configure it with the environment variable
`FLINK_OPERATOR_NAMESPACE`.

After that, you can verify CRD `flinkclusters.flinkoperator.k8s.io` has been
created with

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
