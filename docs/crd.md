# FlinkCluster Custom Resource Definition

The Kubernetes Operator for Apache Flink uses  [CustomResourceDefinition](https://kubernetes.io/docs/concepts/api-extension/custom-resources/)
named `FlinkCluster` for specifying a Flink job cluster ([sample](../config/samples/flinkoperator_v1alpha1_flinkjobcluster.yaml))
or Flink session cluster ([sample](../config/samples/flinkoperator_v1alpha1_flinksessioncluster.yaml)), depending on
whether the job spec is specified. Similarly to other kinds of Kubernetes resources, the custom resource consists of a
resource `Metadata`, a specification in a `Spec` field and a `Status` field. The definitions are organized in the
following structure. The v1alpha1 version of the API definition is implemented [here](../api/v1alpha1/flinkcluster_types.go).

```
FlinkCluster
|__ metadata
|__ spec
    |__ image
        |__ name
        |__ pullPolicy
        |__ pullSecrets
    |__ jobManager
        |__ accessScope
        |__ ports
            |__ rpc
            |__ blob
            |__ query
            |__ ui
        |__ ingress
            |__ hostFormat
            |__ annotations
            |__ useTLS
            |__ tlsSecretName
        |__ resources
        |__ memoryOffHeapRatio
        |__ memoryOffHeapMin
        |__ volumes
        |__ volumeMounts
    |__ taskManager
        |__ replicas
        |__ ports
            |__ data
            |__ rpc
            |__ query
        |__ resources
        |__ memoryOffHeapRatio
        |__ memoryOffHeapMin
        |__ volumes
        |__ volumeMounts
        |__ sidecars
    |__ job
        |__ jarFile
        |__ className
        |__ args
        |__ fromSavepoint
        |__ allowNonRestoredState
        |__ autoSavepointSeconds
        |__ savepointsDir
        |__ savepointGeneration
        |__ parallelism
        |__ noLoggingToStdout
        |__ volumes
        |__ volumeMounts
        |__ initContainers
        |__ restartPolicy
        |__ cleanupPolicy
            |__ afterJobSucceeds
            |__ afterJobFails
        |__ cancelRequested
    |__ envVars
    |__ flinkProperties
    |__ hadoopConfig
        |__ configMapName
        |__ mountPath
    |__ gcpConfig
        |__ serviceAccount
            |__ secretName
            |__ keyFile
            |__ mountPath
|__ status
    |__ state
    |__ components
        |__ jobManagerDeployment
            |__ name
            |__ state
        |__ jobManagerService
            |__ name
            |__ state
        |__ jobManagerIngress
            |__ name
            |__ state
            |__ urls
        |__ taskManagerDeployment
            |__ name
            |__ state
        |__ job
            |__ name
            |__ id
            |__ state
            |__ fromSavepoint
            |__ savepointGeneration
            |__ savepointLocation
            |__ lastSavepointTriggerID
            |__ lastSavepointTime
            |__ restartCount
    |__ lastUpdateTime
```

* **FlinkCluster**:
  * **metadata** (required): Resource metadata (name, namespace, labels, etc).
  * **spec** (required): Flink job or session cluster spec.
    * **image** (required): Flink image for JobManager, TaskManager and job containers.
      * **name** (required): Image name.
      * **pullPolicy** (optional): Image pull policy.
      * **pullSecrets** (optional): Secrets for image pull.
    * **jobManager** (required): JobManager spec.
      * **accessScope** (optional): Access scope of the JobManager service. `enum("Cluster", "VPC", "External")`.
        `Cluster`: accessible from within the same cluster; `VPC`: accessible from within the same VPC; `External`:
        accessible from the internet. Currently `VPC` and `External` are only available for GKE.
      * **ports** (optional): Ports that JobManager listening on.
        * **rpc** (optional): RPC port, default: 6123.
        * **blob** (optional): Blob port, default: 6124.
        * **query** (optional): Query port, default: 6125.
        * **ui** (optional): UI port, default: 8081.
      * **ingress** (optional): Provide external access to JobManager UI/API.
        * **hostFormat** (optional): Host format for generating URLs. ex) {{$clusterName}}.example.com
        * **annotations** (optional): Annotations for ingress configuration.
        * **useTLS** (optional): TLS use, default: false.
        * **tlsSecretName** (optional): Kubernetes secret resource name for TLS.
      * **resources** (optional): Compute resources required by JobManager
        container. If omitted, a default value will be used.
        See [more info](https://kubernetes.io/docs/concepts/configuration/manage-compute-resources-container/) about
        resources.
      * **memoryOffHeapRatio** (optional): Percentage of off-heap memory in containers,
        as a safety margin, default: 25
      * **memoryOffHeapMin** (optional): Minimum amount of off-heap memory in containers,
        as a safety margin, default: 600M.
        You can express this value like 600M, 572Mi and 600e6.
        See [more info](https://kubernetes.io/docs/concepts/configuration/manage-compute-resources-container/#meaning-of-memory)
        about value expression.
      * **volumes** (optional): Volumes in the JobManager pod.
        See [more info](https://kubernetes.io/docs/concepts/storage/volumes/) about volumes.
      * **volumeMounts** (optional): Volume mounts in the JobManager container.
        See [more info](https://kubernetes.io/docs/concepts/storage/volumes/) volume mounts.
    * **taskManager** (required): TaskManager spec.
      * **replicas** (required): The number of TaskManager replicas.
      * **ports** (optional): Ports that TaskManager listening on.
        * **data** (optional): Data port.
        * **rpc** (optional): RPC port.
        * **query** (optional): Query port.
      * **resources** (optional): Compute resources required by JobManager
        container. If omitted, a default value will be used.
        See [more info](https://kubernetes.io/docs/concepts/configuration/manage-compute-resources-container/) about
        resources.
      * **memoryOffHeapRatio** (optional): Percentage of off-heap memory in containers,
        as a safety margin, default: 25
      * **memoryOffHeapMin** (optional): Minimum amount of off-heap memory in containers,
        as a safety margin, default: 600M.
        You can express this value like 600M, 572Mi and 600e6.
        See [more info](https://kubernetes.io/docs/concepts/configuration/manage-compute-resources-container/#meaning-of-memory)
        about value expression.
      * **volumes** (optional): Volumes in the TaskManager pod.
        See [more info](https://kubernetes.io/docs/concepts/storage/volumes/) about volumes.
      * **volumeMounts** (optional): Volume mounts in the TaskManager containers.
        See [more info](https://kubernetes.io/docs/concepts/storage/volumes/) about volume mounts.
      * **sidecars** (optional): Sidecar containers running alongside with the TaskManager container in the pod.
        See [more info](https://kubernetes.io/docs/concepts/containers/) about containers.
    * **job** (optional): Job spec. If specified, the cluster is a Flink job cluster; otherwise, it is a Flink
      session cluster.
      * **jarFile** (required): JAR file of the job. It could be a local file or remote URI, depending on which
        protocols (e.g., `https://`, `gs://`) are supported by the Flink image.
      * **className** (required): Fully qualified Java class name of the job.
      * **args** (optional): Command-line args of the job.
      * **savepoint** (optional): Savepoint where to restore the job from.
      * **autoSavepointSeconds** (optional): Automatically take a savepoint to the `savepointsDir` every n seconds.
      * **savepointDir** (optional): Savepoints dir where to store automatically taken savepoints.
      * **allowNonRestoredState** (optional):  Allow non-restored state, default: false.
      * **savepointGeneration** (optional): Update this field to `jobStatus.savepointGeneration + 1` for a running job
        cluster to trigger a new savepoint to `savepointsDir` on demand.
      * **parallelism** (optional): Parallelism of the job, default: 1.
      * **noLoggingToStdout** (optional): No logging output to STDOUT, default: false.
      * **initContainers** (optional): Init containers of the Job pod.
        See [more info](https://kubernetes.io/docs/concepts/workloads/pods/init-containers/) about init containers.
      * **volumes** (optional): Volumes in the Job pod.
        See [more info](https://kubernetes.io/docs/concepts/storage/volumes/) about volumes.
      * **volumeMounts** (optional): Volume mounts in the Job containers. If there is no confilcts, these mounts will be
        automatically added to init containers; otherwise, the mounts defined in init containers will take precedence.
        See [more info](https://kubernetes.io/docs/concepts/storage/volumes/) about volume mounts.
      * **restartPolicy** (optional): Restart policy when the job fails, `enum("Never", "FromSavepointOnFailure")`,
        default: `"Never"`.
        `"Never"` means the operator will never try to restart a failed job, manual cleanup is required.
        `"FromSavepointOnFailure"` means the operator will try to restart the failed job from the savepoint recorded in
          the job status if available; otherwise, the job will stay in failed state. This option is usually used
          together with `autoSavepointSeconds` and `savepointDir`.
      * **cleanupPolicy** (optional): The action to take after job finishes.
        * **afterJobSucceeds** (required): The action to take after job succeeds,
          `enum("KeepCluster", "DeleteCluster", "DeleteTaskManager")`, default `"DeleteCluster"`.
        * **afterJobFails** (required): The action to take after job fails,
          `enum("KeepCluster", "DeleteCluster", "DeleteTaskManager")`, default `"KeepCluster"`.
      * **cancelRequested** (optional): Request the job to be cancelled. Only applies to running jobs. If
        `savePointsDir` is provided, a savepoint will be taken before stopping the job.
    * **envVars** (optional): Environment variables shared by all JobManager, TaskManager and job containers.
    * **flinkProperties** (optional): Flink properties which are appened to flink-conf.yaml.
    * **hadoopConfig** (optional): Configs for Hadoop.
      * **configMapName**: The name of the ConfigMap which holds the Hadoop config files. The ConfigMap must be in the
        same namespace as the FlinkCluster.
      * **mountPath**: The path where to mount the Volume of the ConfigMap.
    * **gcpConfig** (optional): Configs for GCP.
      * **serviceAccount**: GCP service account.
        * **secretName**: The name of the Secret holding the GCP service account key file. The Secret must be in the
          same namespace as the FlinkCluster.
        * **keyFile**: The name of the service account key file.
        * **mountPath**: The path where to mount the Volume of the Secret.
  * **status**: Flink job or session cluster status.
    * **state**: The overall state of the Flink cluster.
    * **components**: The status of the components.
      * **jobManagerDeployment**: The status of the JobManager deployment.
        * **name**: The resource name of the JobManager deployment.
        * **state**: The state of the JobManager deployment.
      * **jobManagerService**: The status of the JobManager service.
        * **name**: The resource name of the JobManager service.
        * **state**: The state of the JobManager service.
      * **jobManagerIngress**: The status of the JobManager ingress.
        * **name**: The resource name of the JobManager ingress.
        * **state**: The state of the JobManager ingress.
        * **urls**: The generated URLs for JobManager.
      * **taskManagerDeployment**: The status of the TaskManager deployment.
        * **name**: The resource name of the TaskManager deployment.
        * **state**: The state of the TaskManager deployment.
      * **job**: The status of the job.
        * **name**: The resource name of the job.
        * **id**: The ID of the Flink job.
        * **state**: The state of the job.
        * **fromSavepoint**: The actual savepoint from which this job started.
          In case of restart, it might be different from the savepoint in the
          job spec.
        * **savepointGeneration**: The generation of the savepoint in `savepointsDir` taken by the operator. The value
          starts from 0 when there is no savepoint and increases by 1 for each
          successful savepoint.
        * **savepointLocation**: Last savepoint location.
        * **lastSavepointTriggerID**: Last savepoint trigger ID.
        * **lastSavepointTime**: Last successful or failed savepoint operation timestamp.
        * **restartCount**: The number of restarts.
    * **lastUpdateTime**: Last update timestamp of this status.
