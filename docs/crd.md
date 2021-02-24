# FlinkCluster Custom Resource Definition

The Kubernetes Operator for Apache Flink uses  [CustomResourceDefinition](https://kubernetes.io/docs/concepts/api-extension/custom-resources/)
named `FlinkCluster` for specifying a Flink job cluster ([sample](../config/samples/flinkoperator_v1beta1_flinkjobcluster.yaml))
or Flink session cluster ([sample](../config/samples/flinkoperator_v1beta1_flinksessioncluster.yaml)), depending on
whether the job spec is specified. Similarly to other kinds of Kubernetes resources, the custom resource consists of a
resource `Metadata`, a specification in a `Spec` field and a `Status` field. The definitions are organized in the
following structure. The v1beta1 version of the API definition is implemented [here](../api/v1beta1/flinkcluster_types.go).

```
FlinkCluster
|__ metadata
|__ spec
    |__ image
        |__ name
        |__ pullPolicy
        |__ pullSecrets
    |__ batchSchedulerName
    |__ serviceAccountName
    |__ jobManager
        |__ accessScope
        |__ ports
            |__ rpc
            |__ blob
            |__ query
            |__ ui
        |__ extraPorts
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
        |__ volumeClaimTemplates
        |__ initContainers
        |__ nodeSelector
        |__ tolerations
        |__ affinity
        |__ sidecars
        |__ podAnnotations
        |__ podLabels
        |__ securityContext
    |__ taskManager
        |__ replicas
        |__ ports
            |__ data
            |__ rpc
            |__ query
        |__ extraPorts
        |__ resources
        |__ memoryOffHeapRatio
        |__ memoryOffHeapMin
        |__ volumes
        |__ volumeMounts
        |__ volumeClaimTemplates
        |__ initContainers
        |__ nodeSelector
        |__ tolerations
        |__ affinity
        |__ sidecars
        |__ podAnnotations
        |__ podLabels
        |__ securityContext
    |__ job
        |__ jarFile
        |__ className
        |__ args
        |__ fromSavepoint
        |__ allowNonRestoredState
        |__ takeSavepointOnUpgrade
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
            |__ afterJobCancelled
        |__ cancelRequested
        |__ podAnnotations
        |__ podLabels
        |__ securityContext
    |__ envVars
    |__ envFrom
    |__ flinkProperties
    |__ hadoopConfig
        |__ configMapName
        |__ mountPath
    |__ gcpConfig
        |__ serviceAccount
            |__ secretName
            |__ keyFile
            |__ mountPath
    |__ logConfig
    |__ revisionHistoryLimit
    |__ recreateOnUpdate
|__ status
    |__ state
    |__ components
        |__ jobManagerStatefulSet
            |__ name
            |__ state
        |__ jobManagerService
            |__ name
            |__ state
            |__ nodePort
        |__ jobManagerIngress
            |__ name
            |__ state
            |__ urls
        |__ taskManagerStatefulSet
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
    |__ control
        |__ name
        |__ details
        |__ state
        |__ message
        |__ updateTime
    |__ savepoint
        |__ jobID
        |__ triggerID
        |__ triggerTime
        |__ triggerReason
        |__ state
        |__ message
    |__ currentRevision
    |__ nextRevision
    |__ collisionCount
    |__ lastUpdateTime
```

* **FlinkCluster**:
  * **metadata** (required): Resource metadata (name, namespace, labels, etc).
  * **spec** (required): Flink job or session cluster spec.
    * **image** (required): Flink image for JobManager, TaskManager and job containers.
      * **name** (required): Image name.
      * **pullPolicy** (optional): Image pull policy.
      * **pullSecrets** (optional): Secrets for image pull.
    * **batchSchedulerName** (optional): BatchSchedulerName specifies the batch scheduler name for JobManager, TaskManager.
      If empty, no batch scheduling is enabled.
    * **serviceAccountName** (optional): the name of the service account(which must already exist in the namespace).
      If empty, the default service account in the namespace will be used.
    * **jobManager** (required): JobManager spec.
      * **accessScope** (optional): Access scope of the JobManager service. `enum("Cluster", "VPC", "External", 
      "NodePort", "Headless")`. `Cluster`: accessible from within the same cluster; `VPC`: accessible from within the same VPC; 
      `External`: accessible from the internet. `NodePort`: accessible through node port; `Headless`: pod IPs assumed to 
      be routable and advertised directly with `clusterIP: None`.  
      Currently `VPC` and `External` are only available for GKE.
      * **ports** (optional): Ports that JobManager listening on.
        * **rpc** (optional): RPC port, default: 6123.
        * **blob** (optional): Blob port, default: 6124.
        * **query** (optional): Query port, default: 6125.
        * **ui** (optional): UI port, default: 8081.
      * **extraPorts** (optional): Extra ports to be exposed. For example, Flink metrics reporter ports: Prometheus,
        JMX and so on. Each port number and name must be unique among ports and extraPorts. ContainerPort is required
        and name and protocol are optional.
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
      * **volumeClaimTemplates** (optional): A template for persistent volume claim each requested and mounted to JobManager pod,  
        This can be used to mount an external volume with a specific storageClass or larger captivity (for larger/faster state backend)
      * **initContainers** (optional): Init containers of the JobManager pod.
        See [more info](https://kubernetes.io/docs/concepts/workloads/pods/init-containers/) about init containers.
      * **nodeSelector** (optional): Selector which must match a node's labels for the JobManager pod
        to be scheduled on that node.
        See [more info](https://kubernetes.io/docs/concepts/configuration/assign-pod-node/)
      * **tolerations** (optional): Allows the JobManager pod to run on a tainted node
        in the cluster.
        See [more info](https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/)
      * **affinity** (optional): Allows the JobManager pod to set node affinity & inter-pod affinity & anti-affinity
        in the cluster.
        See [more info](hhttps://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#affinity-and-anti-affinity)
      * **sidecars** (optional): Sidecar containers running alongside with the JobManager container in the pod.
        See [more info](https://kubernetes.io/docs/concepts/containers/) about containers.
      * **podAnnotations** (optional): Pod template annotations for the JobManager StatefulSet.
        See [more info](https://kubernetes.io/docs/concepts/overview/working-with-objects/annotations/) about annotations.
      * **podLabels** (optional): Pod template labels for the JobManager StatefulSet.
      * **securityContext** (optional): PodSecurityContext for the JobManager pod. 
      See [more info](https://kubernetes.io/docs/tasks/configure-pod-container/security-context/#set-the-security-context-for-a-pod).
    * **taskManager** (required): TaskManager spec.
      * **replicas** (required): The number of TaskManager replicas.
      * **ports** (optional): Ports that TaskManager listening on.
        * **data** (optional): Data port.
        * **rpc** (optional): RPC port.
        * **query** (optional): Query port.
      * **extraPorts** (optional): Extra ports to be exposed. For example, Flink metrics reporter ports: Prometheus,
        JMX and so on. Each port number and name must be unique among ports and extraPorts. ContainerPort is required
        and name and protocol are optional.
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
      * **volumeClaimTemplates** (optional): A template for persistent volume claim each requested and mounted to each TaskManager pod,  
        This can be used to mount an external volume with a specific storageClass or larger captivity (for larger/faster state backend)
        See [more info](https://kubernetes.io/docs/concepts/workloads/controllers/statefulset/) about VolumeClaimTemplates in StatefulSet.
      * **initContainers** (optional): Init containers of the TaskManager pod.
        See [more info](https://kubernetes.io/docs/concepts/workloads/pods/init-containers/) about init containers.
      * **nodeSelector** (optional): Selector which must match a node's labels for the TaskManager pod to
        be scheduled on that node.
        See [more info](https://kubernetes.io/docs/concepts/configuration/assign-pod-node/)
      * **tolerations** (optional): Allows the TaskManager pod to run on a tainted node
        in the cluster.
        See [more info](https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/)
      * **affinity** (optional): Allows the TaskManager pod to set node affinity & inter-pod affinity & anti-affinity
        in the cluster.
        See [more info](hhttps://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#affinity-and-anti-affinity)
      * **sidecars** (optional): Sidecar containers running alongside with the TaskManager container in the pod.
        See [more info](https://kubernetes.io/docs/concepts/containers/) about containers.
      * **podAnnotations** (optional): Pod template annotations for the TaskManager StatefulSet.
        See [more info](https://kubernetes.io/docs/concepts/overview/working-with-objects/annotations/) about annotations.
      * **podLabels** (optional): Pod template labels for the TaskManager StatefulSet.
      * **securityContext** (optional): PodSecurityContext for the TaskManager pods. 
        See [more info](https://kubernetes.io/docs/tasks/configure-pod-container/security-context/#set-the-security-context-for-a-pod).
    * **job** (optional): Job spec. If specified, the cluster is a Flink job cluster; otherwise, it is a Flink
      session cluster.
      * **jarFile** (required): JAR file of the job. It could be a local file or remote URI, depending on which
        protocols (e.g., `https://`, `gs://`) are supported by the Flink image.
      * **className** (required): Fully qualified Java class name of the job.
      * **args** (optional): Command-line args of the job.
      * **fromSavepoint** (optional): Savepoint where to restore the job from.
        If Flink job must be restored from the latest available savepoint when Flink job updating,
        this field must be unspecified.
      * **autoSavepointSeconds** (optional): Automatically take a savepoint to the `savepointsDir` every n seconds.
      * **savepointsDir** (optional): Savepoints dir where to store automatically taken savepoints.
      * **allowNonRestoredState** (optional):  Allow non-restored state, default: false.
      * **takeSavepointOnUpgrade** (optional):  Should take savepoint before upgrading the job, default: false.
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
          together with `autoSavepointSeconds` and `savepointsDir`.
      * **cleanupPolicy** (optional): The action to take after job finishes.
        * **afterJobSucceeds** (required): The action to take after job succeeds,
          `enum("KeepCluster", "DeleteCluster", "DeleteTaskManager")`, default `"DeleteCluster"`.
        * **afterJobFails** (required): The action to take after job fails,
          `enum("KeepCluster", "DeleteCluster", "DeleteTaskManager")`, default `"KeepCluster"`.
        * **afterJobCancelled** (required): The action to take after job cancelled,
          `enum("KeepCluster", "DeleteCluster", "DeleteTaskManager")`, default `"DeleteCluster"`.
      * **cancelRequested** (optional): Request the job to be cancelled. Only applies to running jobs. If
        `savePointsDir` is provided, a savepoint will be taken before stopping the job.
      * **podAnnotations** (optional): Pod template annotations for the job.
        See [more info](https://kubernetes.io/docs/concepts/overview/working-with-objects/annotations/) about annotations.
      * **podLabels** (optional): Pod template labels for the job.
      * **securityContext** (optional): PodSecurityContext for the Job pod. 
            See [more info](https://kubernetes.io/docs/tasks/configure-pod-container/security-context/#set-the-security-context-for-a-pod).
    * **envVars** (optional): Environment variables shared by all JobManager, TaskManager and job containers.
    * **envFrom** (optional): Environment variables from ConfigMaps or Secrets shared by all JobManager, TaskManager and job containers.
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
    * **logConfig** (optional): The logging configuration, a string-to-string map that becomes the ConfigMap mounted at 
      `/opt/flink/conf` in launched Flink pods. See below for some possible keys to include:
      * **log4j-console.properties**: The contents of the log4j properties file to use. If not provided, a default that 
        logs only to stdout will be provided.
      * **logback-console.xml**: The contents of the logback XML file to use. If not provided, a default that logs only 
        to stdout will be provided.
      * Other arbitrary keys are also allowed, and will become part of the ConfigMap.
    * **revisionHistoryLimit** (optional): The maximum number of revision history to keep, default: 10.
    * **recreateOnUpdate** (optional): Recreate components when updating flinkcluster, default: true.
  * **status**: Flink job or session cluster status.
    * **state**: The overall state of the Flink cluster.
    * **components**: The status of the components.
      * **jobManagerStatefulSet**: The status of the JobManager StatefulSet.
        * **name**: The resource name of the JobManager StatefulSet.
        * **state**: The state of the JobManager StatefulSet.
      * **jobManagerService**: The status of the JobManager service.
        * **name**: The resource name of the JobManager service.
        * **state**: The state of the JobManager service.
        * **nodePort** (optional): The node port, present when `accessScope` is `NodePort`.
      * **jobManagerIngress**: The status of the JobManager ingress.
        * **name**: The resource name of the JobManager ingress.
        * **state**: The state of the JobManager ingress.
        * **urls**: The generated URLs for JobManager.
      * **taskManagerStatefulSet**: The status of the TaskManager StatefulSet.
        * **name**: The resource name of the TaskManager StatefulSet.
        * **state**: The state of the TaskManager StatefulSet.
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
    * **control**: The status of control requested by user.
      * **name**: Requested control name.
      * **details**: Control details.
      * **state**: Control state.
      * **message**: Control message.
      * **updateTime**: The updated time of control status.
    * **savepoint**: The status of savepoint progress.
      * **jobID**: The ID of the Flink job.
      * **triggerID**: Savepoint trigger ID.
      * **triggerTime**: Savepoint triggered time.
      * **triggerReason**: Savepoint triggered reason.
      * **state**: Savepoint state.
      * **message**: Savepoint message.
    * **currentRevision**: CurrentRevision indicates the version of FlinkCluster.
    * **nextRevision**: NextRevision indicates the version of FlinkCluster updating.
    * **collisionCount**: collisionCount is the count of hash collisions for the FlinkCluster.
      The controller uses this field as a collision avoidance mechanism
      when it needs to create the name for the newest ControllerRevision.
    * **lastUpdateTime**: Last update timestamp of this status.
