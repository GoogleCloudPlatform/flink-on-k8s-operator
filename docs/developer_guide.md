# Developer Guide

## Project overview

[Kubernetes](https://kubernetes.io/) Operator for [Apache Flink](https://flink.apache.org)
is built on top of the Kubernetes [controller-runtime](https://github.com/kubernetes-sigs/controller-runtime)
library. The project structure and boilerplate files are generated with
[Kubebuilder](https://github.com/kubernetes-sigs/kubebuilder). Knowledge of
controller-runtime and Kubebuilder is required to understand this project.

The Flink custom resource is defined in Go struct [FlinkCluster](../api/v1beta1/flinkcluster_types.go),
then Kubebuild generates related Go files and YAML files, e.g.
[flinkclusters.yaml](../config/crd/bases/flinkoperator.k8s.io_flinkclusters.yaml).
The custom logic for reconciling a Flink custom resource is inside of the
[controllers](../controllers) directory, e.g., [flinkcluster_controller.go](../controllers/flinkcluster_controller.go).

[Dockerfile](../Dockerfile) defines the steps of building the Flink Operator
image.

[Makefile](../Makefile) includes various actions you can take to generate
code, build the Flink Operator binary, run unit tests, build and push docker
image, deploy the Flink Operator to a Kubernetes cluster.

You might want to watch this tech talk ([video](https://www.youtube.com/watch?v=MXj4lo8XHUE), [slides](https://github.com/GoogleCloudPlatform/flink-on-k8s-operator/blob/master/docs/apache-flink-on-kubernetes-operator-20200212.pdf)) to get a high-level overview of the project.

## Dependencies

The following dependencies are required to build the Flink Operator binary and
run unit tests:

* [Go v1.14+](https://golang.org/)
* [Kubebuilder v2+](https://github.com/kubernetes-sigs/kubebuilder)

But you don't have to install them on your local machine, because this project
includes a [builder Docker image](../Dockerfile.builder) with the dependencies
installed. Build and unit test can happen inside of the builder container. This
is the recommended way for local development.

But to create the Flink Operator Docker image and deploy it to a Kubernetes
cluster, the following dependencies are required on your local machine:

* [Docker](https://docs.docker.com/install/)
* [kubectl v1.14+](https://kubernetes.io/docs/tasks/tools/install-kubectl/)

## Local build and test

To build the Flink Operator binary and run unit tests, run:

### In Docker (recommended)

```bash
make test-in-docker
```

### Non-Docker (not recommended)

```bash
make test
```

## Build and push the operator image

Build a Docker image for the Flink Operator and then push it to an image
registry with

```bash
make operator-image push-operator-image IMG=<tag>
```

For example, if you are using [Google Container Registry](https://cloud.google.com/container-registry/docs/),
follow the instructions [here](https://cloud.google.com/container-registry/docs/pushing-and-pulling)
to set it up, then build the image and push it to GCR with:

```bash
PROJECT=<gcp-project>
IMAGE_TAG=gcr.io/${PROJECT}/flink-operator:latest
make operator-image push-operator-image IMG=${IMAGE_TAG}
```

After building the image, the image tag will be automatically saved in
`config/default/manager_image_patch.yaml`, when you run `make deploy` later,
it knows what image to use.

## Deploy the operator and run jobs

Now you can follow the [User Guide](./user_guide.md) to deploy the operator
and run jobs.
