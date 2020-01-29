# Running Apache Beam Python jobs with the Flink Operator

## Overview

To run [Apache Beam](https://beam.apache.org) Python pipelines on Flink on Kubernetes, it requires several components
depending on the version of Beam:

### Pior to Beam 2.18

1. a Flink session cluster as the actual workload runner.
2. a Beam JobServer which accepts job submission from the client.
3. Beam Python SDK harness workers which run the Python UDFs deserialized from the Flink TaskManagers.

### Beam 2.18+**

1. a Flink session cluster as the actual workload runner.
2. Beam Python SDK harness workers which run the Python UDFs deserialized from the Flink TaskManagers.

The hard part for running Beam on Flink is the Beam Python SDK harness workers. Beam supports several modes to the
workers:

1) **Process mode**. Beam runner in each Flink TM will automatically launch a Beam SDK harness worker process from the
executable. This requires a custom Flink image with Beam SDK builtin, which is not as user-friendly as Beam’s own
[container images](https://beam.apache.org/documentation/runtime/environments/).

2) **Docker mode**. Beam runner in each Flink TM will automatically launch a Beam SDK harness worker container. This
mode works well in a non-Kubernetes environment, but in Kubernetes cluster, it requires running Docker in Docker which
is not supported by default, technically challenging and with some major drawbacks, see more details in this
[article](https://jpetazzo.github.io/2015/09/03/do-not-use-docker-in-docker-for-ci/).

3) **External mode**. Beam runner doesn't launch Beam SDK harness worker by itself, but sends a request to a WorkerPool
service to launch one. This fits well with Kubernetes. We can model the WorkerPool service as a sidecar container of the
Flink TaskManager container, which is a feature of the Flink Operator. The benefit of sidecar containers is that they
are loosely coupled with the main container and we can control their resources separately. **This is the mode of
choice** for running Beam with the Flink Operator.

## Steps

As a prerequisite, you need to deploy the Flink Operator to your Kubernetes cluster by following the
[user guide](./user_guide.md).

Then depending on whether you use JobServer or not, take the following 3 or 2 steps to run a Beam WordCount Python
example job with the Flink Operator. You can write a script to automate the process.

### With JobServer

1. Create a Flink session cluster with Beam WorkerPool as sidecar containers of Flink TaskManager containers with:

  ```bash
  kubectl apply -f examples/beam/with_job_server/beam_flink_cluster.yaml
  ```

2. Start Beam JobServer with:

  ```bash
  kubectl apply -f examples/beam/with_job_server/beam_job_server.yaml
  ```

3. After both Flink cluster and Beam JobServer are up and running, submit the job with:

  ```bash
  kubectl apply -f examples/beam/with_job_server/beam_wordcount_py.yaml
  ```

### Without JobServer (Beam 2.18+ only)

1. Create a Flink session cluster with Beam WorkerPool as sidecar containers of Flink TaskManager containers with:

  ```bash
  kubectl apply -f examples/beam/with_job_server/beam_flink_cluster.yaml
  ```

2. After the Flink cluster is up and running, submit the job with:

  ```bash
  kubectl apply -f examples/beam/without_job_server/beam_wordcount_py.yaml
  ```

## Known issues

Currently there are 2 known issues with running Beam jobs without JobServer:

1. Sometimes it also fails with the error `TypeError: GetJobMetrics() missing 1 required positional argument:
'context'`, but after retry it succeeds.

2. Sometimes the job has changed state to DONE but the process doesn't exit as expected.

## Roadmap

In the future, we plan to support Beam Python job as a first class job type in this operator. After that you will not
need to manage the lifecycle of Flink session cluster by yourself, it would be the same as Flink job cluster.
