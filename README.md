# Kubernetes Operator for Apache Flink

**This is not an officially supported Google product.**

Kubernetes Operator for Apache Flink is a control plane for running [Apache Flink](https://flink.apache.org/) on
[Kubernetes](https://kubernetes.io/).

## Project Status

*Alpha*

The operator is under active development, backward compatibility of the APIs is not guaranteed for alpha releases.

## Prerequisites

* Version >= 1.9 of Kubernetes
* Version >= 1.7 of Apache Flink

## Overview

The Kubernetes Operator for Apache Flink extends the vocabulary (e.g., Pod, Service, etc) of the Kubernetes language
with custom resource definition [FlinkCluster](docs/crd.md) and runs a
[controller](controllers/flinkcluster_controller.go) Pod to keep watching the custom resources.
Once a FlinkCluster custom resource is created and detected by the controller, the controller creates the underlying
Kubernetes resources (e.g., JobManager Pod) based on the spec of the custom resource. With the operator installed in a
cluster, users can then talk to the cluster through the Kubernetes API and Flink custom resources to manage their Flink
clusters and jobs.

The operator supports creating both [Flink job cluster](https://ci.apache.org/projects/flink/flink-docs-stable/ops/deployment/docker.html#flink-job-cluster)
and [Flink session cluster](https://ci.apache.org/projects/flink/flink-docs-stable/ops/deployment/docker.html#flink-session-cluster)
through one custom resource [FlinkCluster](config/crd/bases/flinkoperator.k8s.io_flinkclusters.yaml), depending on
whether a job spec is provided. See samples: [Flink job cluster](config/samples/flinkoperator_v1alpha1_flinkjobcluster.yaml),
[Flink session cluster](config/samples/flinkoperator_v1alpha1_flinksessioncluster.yaml).

## Installation

The operator is still under active development, there is not Helm chart available yet, please follow the
[Developer Guide](docs/developer-guide.md) to build the operator and deploy it to your Kubernetes cluster.

## Documentation

* [Developer Guide](docs/developer-guide.md)
* [FlinkCluster Custom Resource Definition](docs/crd.md)
* [Test environment setup for streaming applications](docs/streaming_test_env_setup.md)
* [Use remote job JAR in GCS](config/samples/flinkoperator_v1alpha1_remotejobjar.yaml)
* [Use GCS as remote storage with custom Flink images](images/flink/README.MD)

## Contributing

Please check [CONTRIBUTING.md](CONTRIBUTING.md) and the [Developer Guide](docs/developer-guide.md) out.
