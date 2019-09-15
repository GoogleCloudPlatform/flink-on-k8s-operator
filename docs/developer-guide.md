# Developer Guide

## Project overview

[Kubernetes](https://kubernetes.io/) Operator for [Apache Flink](https://flink.apache.org) is built on top of the
Kubernetes [controller-runtime](https://github.com/kubernetes-sigs/controller-runtime) library. The project structure
and boilerplate files are generated with [Kubebuilder](https://github.com/kubernetes-sigs/kubebuilder). Knowledge of
controller-runtime and Kubebuilder is required to better understand this project.

The Flink custom resource is defined in Go struct [FlinkCluster](../api/v1alpha1/flinkcluster_types.go),
then Kubebuild generates related Go files and YAML files, e.g.
[flinkclusters.yaml](../config/crd/bases/flinkoperator.k8s.io_flinkclusters.yaml).
The custom logic for reconciling a Flink custom resource is inside of the [controllers](../controllers) directory, e.g.,
[flinkcluster_controller.go](../controllers/flinkcluster_controller.go).

The [Dockerfile](../Dockerfile) defines the steps of building the Flink Operator image.

The [Makefile](../Makefile) includes various actions you can take to generate code, build the Flink Operator binary, run
unit tests, build and push docker image, deploy the Flink Operator to a Kubernetes cluster.

## Dependencies

The following dependencies are required on your local machine to generate, build and deploy the operator:

* [Go v1.12+](https://golang.org/)
* [Docker](https://www.docker.com/)
* [kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl/)
* [Kustomize v3.1+](https://github.com/kubernetes-sigs/kustomize)
* [Kubebuilder v2+](https://github.com/kubernetes-sigs/kubebuilder)

## Local build and test

To build the Flink Operator binary locally, run

```bash
make flink-operator
```

or simply

```bash
make
```

```make flink-operator``` automatically runs ```make generate``` which generates Go source code and YAML files
with KubeBuilder.

Run unit tests with

```bash
make test
```

## Build and push docker image

Build a Docker image for the Flink Operator and then push it to an image
registry with

```bash
make docker-build docker-push IMG=<tag>
```

Depending on which image registry you want to use, choose a tag accordingly, e.g., if you are using
[Google Container Registry](https://cloud.google.com/container-registry/docs/) you want to use a tag like
`gcr.io/<project>/flink-operator:latest`.

After building the image, it automatically saves the image tag in `config/default/manager_image_patch.yaml`, so that
when you deploy the Flink operator later, it knows what image to use.

## Deploy the operator to a running Kubernetes cluster

Assume you have built and pushed the Flink Operator image, then you need to have a running Kubernetes cluster. Verify
the cluster info with

```bash
kubectl cluster-info
```

Deploy the Flink Custom Resource Definitions and the Flink Operator to the cluster with

```bash
make deploy
```

After that, you can verify CRD `flinkclusters.flinkoperator.k8s.io` has been created with

```bash
kubectl get crds | grep flinkclusters.flinkoperator.k8s.io
```

You can also view the details of the CRD with

```bash
kubectl describe crds/flinkclusters.flinkoperator.k8s.io
```

The operator runs as a Kubernetes Deployment, you can find out the deployment with

```bash
kubectl get deployments -n flink-operator-system
```

or verify the operator pod is up and running.

```bash
kubectl get pods -n flink-operator-system
```

You can also check the operator logs with

```bash
kubectl logs -n flink-operator-system -l app=flink-operator --all-containers
```

## Create a sample Flink cluster

After deploying the Flink CRDs and the Flink Operator to a Kubernetes cluster, the operator serves as a control plane
for Flink. In other words, previously the cluster only understands the language of Kubernetes, now it understands the
language of Flink. You can then create custom resources representing Flink session clusters or job clusters, the
operator will detect the custom resources automatically, then create the actual clusters optionally run jobs, and update
status in the custom resources.

Create a [sample Flink session cluster](../config/samples/flinkoperator_v1alpha1_flinksessioncluster.yaml) custom
resource with

```bash
kubectl apply -f config/samples/flinkoperator_v1alpha1_flinksessioncluster.yaml
```

and a [sample Flink job cluster](../config/samples/flinkoperator_v1alpha1_flinkjobcluster.yaml) custom resource with

```bash
kubectl apply -f config/samples/flinkoperator_v1alpha1_flinkjobcluster.yaml
```

## Monitoring

### Operator

You can check the operator logs with

```bash
kubectl logs -n flink-operator-system -l app=flink-operator --all-containers -f --tail=1000
```

### Flink cluster and job

After deploying a Flink cluster with the operator, you can find the cluster custom resource with

```bash
kubectl get flinkclusters
```

check the cluster status with

```bash
kubectl describe flinkclusters <CLUSTER-NAME>
```

In a job cluster, check the Flink job status and logs with

```bash
kubectl describe jobs <CLUSTER-NAME>-job
kubectl logs jobs/<CLUSTER-NAME>-job -f --tail=1000
```

You can also access the Flink web UI via a proxy, run the following command in a terminal

```bash
kubectl proxy
```

then navigate to

```
http://localhost:8001/api/v1/namespaces/default/services/<CLUSTER-NAME>-jobmanager:ui/proxy
```

in your browser.

## Undeploy the operator

Undeploy the operator and CRDs from the Kubernetes cluster with

```
make undeploy
```
