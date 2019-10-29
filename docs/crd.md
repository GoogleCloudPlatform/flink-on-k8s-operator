# FlinkCluster Custom Resource Definition

The Kubernetes Operator for Apache Flink uses  [CustomResourceDefinition](https://kubernetes.io/docs/concepts/api-extension/custom-resources/)
named `FlinkCluster` for specifying a Flink job cluster ([sample](../config/samples/flinkoperator_v1alpha1_flinkjobcluster.yaml))
or Flink session cluster ([sample](../config/samples/flinkoperator_v1alpha1_flinksessioncluster.yaml)), depending on
whether the job spec is specified. Similarly to other kinds of Kubernetes resources, the custom resource consists of a
resource `Metadata`, a specification in a `Spec` field and a `Status` field. The definitions are organized in the
following structure. The v1alpha1 version of the API definition is implemented [here](../api/v1alpha1/flinkcluster_types.go).

```
FlinkCluster
|__ Metadata
|__ Spec
    |__ ImageSpec
        |__ Name
        |__ PullPolicy
        |__ PullSecrets
    |__ JobManagerSpec
        |__ AccessScope
        |__ Ports
            |__ RPC
            |__ Blob
            |__ Query
            |__ UI
        |__ Ingress
            |__ HostFormat
            |__ Annotations
            |__ UseTLS
            |__ TLSSecretName
        |__ Resources
        |__ Volumes
        |__ Mounts
    |__ TaskManagerSpec
        |__ Replicas
        |__ Ports
            |__ Data
            |__ RPD
            |__ Query
        |__ Resources
        |__ Volumes
        |__ Mounts
    |__ JobSpec
        |__ JarFile
        |__ ClassName
        |__ Args
        |__ Savepoint
        |__ AllowNonRestoredState
        |__ Parallelism
        |__ NoLoggingToStdout
        |__ RestartPolicy
        |__ Volumes
        |__ Mounts
        |__ Sidecars
    |__ FlinkProperties
    |__ EnvVars
|__ Status
    |__ State
    |__ Components
        |__ JobManagerDeployment
            |__ Name
            |__ State
        |__ JobManagerService
            |__ Name
            |__ State
        |__ JobManagerIngress
            |__ Name
            |__ State
            |__ URLs
        |__ TaskManagerDeployment
            |__ Name
            |__ State
        |__ Job
            |__ Name
            |__ ID
            |__ State
    |__ LastUpdateTime
```

* **FlinkCluster**:
  * **Metadata** (required): Resource metadata (name, namespace, labels, etc).
  * **Spec** (required): Flink job or session cluster spec.
    * **ImageSpec** (required): Flink image for JobManager, TaskManager and job containers.
      * **Image** (required): Image name.
      * **PullPolicy** (optional): Image pull policy.
      * **PullSecrets** (optional): Secrets for image pull.
    * **JobManagerSpec** (required): JobManager spec.
      * **AccessScope** (optional): Access scope of the JobManager service. `enum("Cluster", "VPC", "External")`.
        `Cluster`: accessible from within the same cluster; `VPC`: accessible from within the same VPC; `External`:
        accessible from the internet. Currently `VPC` and `External` are only available for GKE.
      * **Ports** (optional): Ports that JobManager listening on.
        * **RPC** (optional): RPC port, default: 6123.
        * **Blob** (optional): Blob port, default: 6124.
        * **Query** (optional): Query port, default: 6125.
        * **UI** (optional): UI port, default: 8081.
      * **Ingress** (optional): Provide external access to JobManager UI/API.
        * **HostFormat** (optional): Host format for generating URLs. ex) {{$clusterName}}.example.com
        * **Annotations** (optional): Annotations for ingress configuration.
        * **UseTLS** (optional): TLS use, default: false.
        * **TLSSecretName** (optional): Kubernetes secret resource name for TLS.
      * **Resources** (optional): Compute resources required by JobManager
        container. If omitted, a default value will be used.
        More info: https://kubernetes.io/docs/concepts/configuration/manage-compute-resources-container/
      * **Volumes** (optional): Volumes in the JobManager pod.
        More info: https://kubernetes.io/docs/concepts/storage/volumes/
      * **Mounts** (optional): Volume mounts in the JobManager container.
        More info: https://kubernetes.io/docs/concepts/storage/volumes/
    * **TaskManagerSpec** (required): TaskManager spec.
      * **Replicas** (required): The number of TaskManager replicas.
      * **Ports** (optional): Ports that TaskManager listening on.
        * **Data** (optional): Data port.
        * **RPC** (optional): RPC port.
        * **Query** (optional): Query port.
      * **Resources** (optional): Compute resources required by JobManager
        container. If omitted, a default value will be used.
        More info: https://kubernetes.io/docs/concepts/configuration/manage-compute-resources-container/
      * **Volumes** (optional): Volumes in the TaskManager pod.
        More info: https://kubernetes.io/docs/concepts/storage/volumes/
      * **Mounts** (optional): Volume mounts in the TaskManager containers.
        More info: https://kubernetes.io/docs/concepts/storage/volumes/
      * **Sidecars** (optional): Sidecar containers running alongside with the TaskManager container in the pod.
        More info: https://kubernetes.io/docs/concepts/containers/
    * **JobSpec** (optional): Job spec. If specified, the cluster is a Flink job cluster; otherwise, it is a Flink
      session cluster.
      * **JarFile** (required): JAR file of the job. It could be a local file or remote URI, depending on which
        protocols (e.g., `https://`, `gs://`) are supported by the Flink image.
      * **ClassName** (required): Fully qualified Java class name of the job.
      * **Args** (optional): Command-line args of the job.
      * **Savepoint** (optional): Savepoint where to restore the job from.
      * **AutoSavepointSeconds** (optional): Automatically take a savepoint to the savepoints dir every n seconds.
      * **SavepointDir** (optional): Savepoints dir where to store automatically taken savepoints.
      * **AllowNonRestoredState** (optional):  Allow non-restored state, default: false.
      * **Parallelism** (optional): Parallelism of the job, default: 1.
      * **NoLoggingToStdout** (optional): No logging output to STDOUT, default: false.
      * **Volumes** (optional): Volumes in the Job pod.
        More info: https://kubernetes.io/docs/concepts/storage/volumes/
      * **Mounts** (optional): Volume mounts in the Job container.
        More info: https://kubernetes.io/docs/concepts/storage/volumes/
      * **RestartPolicy** (optional): Restart policy, `OnFailure` or `Never`, default: `OnFailure`.
      * **CleanupPolicy** (optional): The action to take after job finishes.
        * **AfterJobSucceeds** (required): The action to take after job succeeds,
          `enum("KeepCluster", "DeleteCluster", "DeleteTaskManager")`, default `"DeleteCluster"`.
        * **AfterJobFails** (required): The action to take after job fails,
          `enum("KeepCluster", "DeleteCluster", "DeleteTaskManager")`, default `"KeepCluster"`.
    * **FlinkProperties** (optional): Flink properties which are appened to flink-conf.yaml of the Flink image.
    * **EnvVars** (optional): Environment variables shared by all JobManager, TaskManager and job containers.
  * **Status**: Flink job or session cluster status.
    * **State**: The overall state of the Flink cluster.
    * **Components**: The status of the components.
      * **JobManagerDeployment**: The status of the JobManager deployment.
        * **Name**: The resource name of the JobManager deployment.
        * **State**: The state of the JobManager deployment.
      * **JobManagerService**: The status of the JobManager service.
        * **Name**: The resource name of the JobManager service.
        * **State**: The state of the JobManager service.
      * **JobManagerIngress**: The status of the JobManager ingress.
        * **Name**: The resource name of the JobManager ingress.
        * **State**: The state of the JobManager ingress.
        * **URLs**: The generated URLs for JobManager.
      * **TaskManagerDeployment**: The status of the TaskManager deployment.
        * **Name**: The resource name of the TaskManager deployment.
        * **State**: The state of the TaskManager deployment.
      * **Job**: The status of the job.
        * **Name**: The resource name of the job.
        * **ID**: The ID of the Flink job.
        * **State**: The state of the job.
        * **Savepoints**: Savepoint URLs.
        * **LastSavepointTriggerID**: Last savepoint trigger ID.
        * **LastSavepointTime**: Last successful or failed savepoint operation timestamp.
    * **LastUpdateTime**: Last update timestamp of this status.
