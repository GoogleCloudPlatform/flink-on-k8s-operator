# Developer Guide

## Project overview

The Flink Operator is built on top of the Kubernetes [controller-runtime](https://github.com/kubernetes-sigs/controller-runtime)
library, and the project structure and boilerplate files are generated with [Kubebuilder](https://github.com/kubernetes-sigs/kubebuilder).
Knowledge of controller-runtime and Kubebuilder is required to better understand this project.

The Flink custom resources are defined in Go structs, e.g. [FlinkSessionCluster](../api/v1alpha1/flinksessioncluster_types.go),
then Kubebuild generates related other Go files and YAML files, e.g.
[flinksessionclusters.yaml](../config/crd/bases/flinkoperator.k8s.io_flinksessionclusters.yaml).
The custom logic for reconciling a Flink custom resource is inside of the [controllers](../controllers) directory, e.g.,
[flinksessioncluster_controller.go](../controllers/flinksessioncluster_controller.go).

The [Dockerfile](../Dockerfile) defines the steps of building the Flink Operator image.

The [Makefile](../Makefile) includes various actions you can take to generate code, build the Flink Operator binary, run
unit tests, build and push docker image, deploy the Flink Operator to a Kubernetes cluster.

## Local build and test

Run

```bash
make flink-operator
```

or simply

```bash
make
```

to build the Flink Operator binary locally. This is useful to detect compliation errors after you change the source
code. ```make flink-operator``` automatically runs ```make generate``` which generates Go source code and YAML files
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

Depending on which image registry you want to use, choose a tag accordingly, e.g., if you are using [Google Container Registry](https://cloud.google.com/container-registry/docs/) you want to use a tag like `gcr.io/<project>/flink-operator:latest`.

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

After that, you can verify CRDs are created with

```bash
kubectl get crds | grep flink
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

After deploying the Flink CRDs and the Flink Operator to a Kubernetes cluster, the operator serves as a control plane for Flink. In other words, previously the cluster only understands the language of Kubernetes, now it understands the language of Flink. You can then create custom resources representing Flink session clusters or job clusters, the operator will detect the custom resources automatically, then create the actual clusters optionally run jobs, and update status in the custom resources.

Deploy the sample Flink clusters with the following command

```bash
make samples
```

After that you can check the operator logs with

```bash
kubectl logs -n flink-operator-system -l app=flink-operator --all-containers -f --tail=1000
```

or check the custom resources with

```bash
kubectl describe flinksessionclusters
kubectl describe flinkjobclusters
```

## Undeploy the operator

Undeploy the operator from the Kubernetes cluster with

```
make undeploy
```
