# Running Apache Beam Python jobs with the Flink Operator

## Overview

To run [Apache Beam](https://beam.apache.org) Python jobs on Flink on Kubernetes, it requires several components
depending on the version of Beam:

### Pior to Beam 2.18

1. a Flink session cluster as the actual workload runner.
2. Beam Python SDK harness workers which run the Python UDFs deserialized from the Flink TaskManagers.
3. a Beam JobServer which accepts job submission from the client.

### Beam 2.18+

1. a Flink session cluster as the actual workload runner.
2. Beam Python SDK harness workers which run the Python UDFs deserialized from the Flink TaskManagers.

When running with the operator, Beam Python SDK harness workers run as sidecar containers with the Flink TaskManagers.

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

2. Replace the `ARTIFACTS_DIR` with a directory (e.g., `gs://my-bucket/artifacts`) accessible from the cluster in
  [examples/beam/with_job_server/beam_job_server.yaml](../examples/beam/with_job_server/beam_job_server.yaml), then start a
  Beam JobServer with:

  ```bash
  kubectl apply -f examples/beam/with_job_server/beam_job_server.yaml
  ```

3. After both the Flink cluster and the Beam JobServer are up and running, submit the example job with:

  ```bash
  kubectl apply -f examples/beam/with_job_server/beam_wordcount_py.yaml
  ```

### Without JobServer (Beam 2.18+ only)

1. Create a Flink session cluster with Beam WorkerPool as sidecar containers of Flink TaskManager containers with:

  ```bash
  kubectl apply -f examples/beam/without_job_server/beam_flink_cluster.yaml
  ```

2. After the Flink cluster is up and running, submit the example job with:

  ```bash
  kubectl apply -f examples/beam/without_job_server/beam_wordcount_py.yaml
  ```

## Known issues

Currently there are 2 known issues with running Beam jobs without JobServer:

1. [BEAM-9214](https://issues.apache.org/jira/browse/BEAM-9214): sometimes the job first fails with `TypeError:
  GetJobMetrics() missing 1 required positional argument: 'context'`, but after retry it succeeds.

2. [BEAM-9225](https://issues.apache.org/jira/browse/BEAM-9225): the job process doesn't exit as expected after it has
  changed state to DONE.

## Roadmap

In the future, we plan to support Beam Python job as a first class job type in this operator. After that you will not
need to manage the lifecycle of Flink session cluster by yourself, it would be the same as Flink job cluster.
