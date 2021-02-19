# Managing savepoints with the Flink Operator

A Flink [savepoint](https://ci.apache.org/projects/flink/flink-docs-stable/ops/state/savepoints.html) is a consistent
image of the execution state of a streaming job. Users can take savepoints of a running job and restart the job from
them later. This document introduces how the Flink Operator can help you manage savepoints.

## Starting a job from a savepoint

First, you can start a job from a savepoint by specifying the `fromSavepoint` property in the job spec, for example:

```yaml
apiVersion: flinkoperator.k8s.io/v1beta1
kind: FlinkCluster
metadata:
  name: flinkjobcluster-sample
spec:
  ...
  job:
    fromSavepoint: gs://my-bucket/savepoints/savepoint-123
    allowNonRestoredState: false
    ...
```

The `allowNonRestoredState` controls whether to allow non-restored state, see more info about the property in the
[Flink CLI doc](https://ci.apache.org/projects/flink/flink-docs-stable/ops/cli.html).

## Taking savepoints for a job

There are two ways the operator can help take savepoints for your job.

### 1. Automatic savepoints

You can let the operator to take savepoints for you automatically by specifying the `autoSavepointSeconds` and
`savepointsDir` properties. In the following example, the operator will take a savepoint into the
`gs://my-bucket/savepoints/` GCS folder every 300 seconds.

```yaml
apiVersion: flinkoperator.k8s.io/v1beta1
kind: FlinkCluster
metadata:
  name: flinkjobcluster-sample
spec:
  ...
  job:
    autoSavepointSeconds: 300
    savepointsDir: gs://my-bucket/savepoints/
    ...
```

You can check the savepoint status in the job status, for example:

```bash
kubectl describe flinkclusters flinkjobcluster-sample

Name:         flinkjobcluster-sample
Namespace:    default
API Version:  flinkoperator.k8s.io/v1beta1
Kind:         FlinkCluster
Spec:
  ...
Status:
  Components:
    ...
    Job:
      Name:                       flinkjobcluster-sample-job
      Id:                         c0c55ce62eba6ab41b6bb9288ef79c12
      Savepoint Generation:       2
      Savepoint Location:         gs://my-bucket/savepoints/savepoint-c0c55c-63ed75ba89c1
      Last Savepoint Time:        2019-11-20T01:50:39Z
      State:                      Running
      ...
```

For each successful savepoint, the savepoint generation in the job status will increase by 1. The latest savepoint
location is also recorded in the job status.

### 2. Taking savepoints by updating the FlinkCluster custom resource

You can also manually take a savepoint for a running job by editing the `savepointGeneration` in the job spec to
`jobStatus.savepointGeneration + 1`, then apply the updated manifest YAML to the cluster. The operator will detect the
update and trigger a savepoint to `savepointsDir`.

For example, if the current savepoint generation in the job status is 2, you can manually trigger a savepoint by editing
the `savepointGeneration` in the job spec to 3 as below:

```yaml
apiVersion: flinkoperator.k8s.io/v1beta1
kind: FlinkCluster
metadata:
  name: flinkjobcluster-sample
spec:
  ...
  job:
    savepointsDir: gs://my-bucket/savepoints/
    savepointGeneration: 3
    ...
```

and applying it to the cluster

```bash
kubectl apply -f flinkjobcluster_sample.yaml
```

after a while you should be able to see the 3rd generation of savepoint in the job status:

```bash
kubectl describe flinkclusters flinkjobcluster-sample

Name:         flinkjobcluster-sample
Namespace:    default
API Version:  flinkoperator.k8s.io/v1beta1
Kind:         FlinkCluster
Spec:
  ...
Status:
  Components:
    ...
    Job:
      Name:                       flinkjobcluster-sample-job
      Id:                         c0c55ce62eba6ab41b6bb9288ef79c12
      Savepoint Generation:       3
      Savepoint Location:         gs://my-bucket/savepoints/savepoint-c0c55c-75ed63ba63b2
      Last Savepoint Trigger ID:  21db90b88ce7a6e201032d9f764cdd64
      Last Savepoint Time:        2019-11-20T02:10:19Z
      State:                      Running
      ...
```

### 3. Taking savepoints by attaching annotation to the FlinkCluster custom resource

You can take a savepoint by attaching control annotation to your FlinkCluster's metadata:

```
metadata:
  annotations:
    flinkclusters.flinkoperator.k8s.io/user-control: savepoint
```

You can attach the annotation with "kubectl apply" like above or "kubectl annotate":

```bash
kubectl annotate flinkclusters flinkjobcluster-sample flinkclusters.flinkoperator.k8s.io/user-control=savepoint
```

When savepoint control is finished, you can check the progress and the result in the control status and the job status
```bash
kubectl describe flinkcluster flinkjobcluster-sample

Name:         flinkjobcluster-sample
Namespace:    default
API Version:  flinkoperator.k8s.io/v1beta1
Kind:         FlinkCluster
Spec:
  ...
Status:
  Components:
    ...
    Job:
      Name:                       flinkjobcluster-sample-job
      Id:                         c0c55ce62eba6ab41b6bb9288ef79c12
      Savepoint Generation:       3
      Savepoint Location:         gs://my-bucket/savepoints/savepoint-c0c55c-75ed63ba63b2
      Last Savepoint Trigger ID:  21db90b88ce7a6e201032d9f764cdd64
      Last Savepoint Time:        2019-11-20T02:10:19Z
      State:                      Running
  ...
  Control:
    Details:
      Job ID:                e689263060695231f62fa8b00f97b383
      Savepoint Trigger ID:  240c9340399d84d2bb7bfb63ff516167
    Name:                    savepoint
    State:                   Succeeded
```

### 4. Taking savepoints with the Flink CLI or through the REST API

In some situations, e.g., you didn't specify `savepointsDir` in the FlinkCluster custom resource, you might want to
bypass the operator and take savepoints by running the [Flink CLI](https://ci.apache.org/projects/flink/flink-docs-stable/ops/cli.html)
or calling the [Flink REST API](https://ci.apache.org/projects/flink/flink-docs-stable/monitoring/rest_api.html) from
your local machine.

In this case, you can first create a port forward from your local machine to the JobManager service
UI port (8081 by default):

```bash
kubectl port-forward svc/[FLINK_CLUSTER_NAME]-jobmanager 8081:8081
```

then you need to get the Flink job ID from the job status with `kubectl get flinkclusters [FLINK_CLUSTER_NAME]` or by
running the Flink CLI command `flink list -m localhost:8081 -a` or by calling the Flink jobs API with
`curl http://localhost:8081/jobs`,

then take a savepoint with the Flink CLI:

```bash
flink savepoint -m localhost:8081 [JOB_ID] [SAVEPOINT_DIR]
```

or call the Flink API to trigger an async savepoint operation for the job:

```bash
curl -X POST -d '{"target-directory": "[SAVEPOINT_DIR]", "cancel-job": false}' http://localhost:8081/jobs/[JOB_ID]/savepoints
```

if the request is accept, it will return a trigger request ID which can be used to query the operation status:

```bash
curl http://localhost:8081/jobs/[JOB_ID]/savepoints/[TRIGGER_ID]
```

## Automatically restarting job from the latest savepoint

Long-running jobs may fail for various reasons, in such cases, if you have enabled auto savepoints or manually took
savepoints, you might want to check the latest savepoint location in the job status, then use it as `fromSavepoint` to
create a new job cluster. But this is a tedious process, fortunately the operator can help you automate it, all you need
to do is set the `restartPolicy` property to `FromSavepointOnFailure` in the job spec, for example:

```yaml
apiVersion: flinkoperator.k8s.io/v1beta1
kind: FlinkCluster
metadata:
  name: flinkjobcluster-sample
spec:
  ...
  job:
    autoSavepointSeconds: 300
    savepointsDir: gs://my-bucket/savepoints/
    restartPolicy: FromSavepointOnFailure
    ...
```

Note that

* The operator can only automatically restart a failed job when there is a savepoint recorded in the job status whether
  it is automatically or  manually taken; otherwise, the job will stay in failed state.
* The job status includes a `fromSavepoint` property which is the actual savepoint from which the job start or
  restarted. It could be different from the one you specified in the job spec in case of restart.

## Storing savepoints in remote storages

Usually you want to store savepoints in remote storages, see this [doc](../images/flink/README.md) on how you can store
savepoints in GCS.
