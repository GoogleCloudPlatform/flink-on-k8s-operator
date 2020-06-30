# Integration with Volcano for Batch Scheduling

[Volcano](https://github.com/volcano-sh/volcano) is a batch system built on Kubernetes. It provides a suite of mechanisms
currently missing from Kubernetes that are commonly required by many classes
of batch & elastic workloads.
With the integration with Volcano, Flink job and task managers can be scheduled simultaneously, which is particularly suitable for 
clusters with resource shortage.

## Prerequisites

## Install Volcano

- Install from provided demo

Run the following 
```bash
kubectl apply -f https://raw.githubusercontent.com/volcano-sh/volcano/master/installer/volcano-development.yaml
```

- Install with advanced settings

Please refer to [Volcano Official Guide](https://volcano.sh/docs/getting-started/)

### Verify Volcano is up and running

```bash
$ kubectl get pod -n volcano-system
NAME                                      READY   STATUS      RESTARTS   AGE
pod/volcano-admission-75688c79bf-b8fmj    1/1     Running     0          52s
pod/volcano-admission-init-d684j          0/1     Completed   0          53s
pod/volcano-controllers-d87bdbd7c-q6ds6   1/1     Running     0          52s
pod/volcano-scheduler-5476779fd9-8rslv    1/1     Running     0          52s

```
 
## Install Flink Operator with Volcano enabled

Please refer to [Deploy the operator to a Kubernetes cluster](./user_guide.md#deploy-the-operator-to-a-kubernetes-cluster)

# Create a sample Flink session cluster with batch scheduling enabled

Create a sample Flink session cluster with:

```bash
$ kubectl apply -f config/samples/flinkoperator_v1beta1_flinksessioncluster_volcano.yaml
```

and verify the pod is up and running with

```bash
$ kubectl get pods,svc -n default | grep "flinksessioncluster"
pod/flinksessioncluster-sample-jobmanager-6d84cd6959-b68dn    1/1     Running   0          21h
pod/flinksessioncluster-sample-taskmanager-7c6b8cdc64-x44r4   2/2     Running   7          21h
service/flinksessioncluster-sample-jobmanager   ClusterIP   10.106.15.205   <none>        6123/TCP,6124/TCP,6125/TCP,8081/TCP   21h
```

verify `job manager` and `task manager` are scheduled by volcano

```bash
$ kubectl get podgroup flink-flinksessioncluster-sample -oyaml
apiVersion: scheduling.volcano.sh/v1beta1
kind: PodGroup
metadata:
  creationTimestamp: "2020-06-18T09:34:01Z"
  generation: 6
  name: flink-flinksessioncluster-sample
  namespace: default
  ownerReferences:
  - apiVersion: flinkoperator.k8s.io/v1beta1
    blockOwnerDeletion: false
    controller: true
    kind: FlinkCluster
    name: flinksessioncluster-sample
    uid: 25bcb47f-e4eb-4619-87ba-c848c42a5d90
  resourceVersion: "1915"
  selfLink: /apis/scheduling.volcano.sh/v1beta1/namespaces/default/podgroups/flink-flinksessioncluster-sample
  uid: 5b7ea6bb-ccf3-483c-8ea3-36007a4aeeb2
spec:
  minMember: 2
  minResources:
    cpu: 400m
    memory: 2Gi
status:
  phase: Running
  running: 
```

As shown above, the podgroup has two pods in running phase and the min required number is 2, that means if the cluster has no enough resources to run both the job manager and task manager, then they are not scheduled.

Also you can check the job manager and task manager's scheduler name is now set to `volcano`

```bash
$ kubectl get pod flinksessioncluster-sample-taskmanager-7c6b8cdc64-x44r4 -ojsonpath={'.spec.schedulerName'}
volcano
```

**Note**: the job's pod is not included in the podgroup.

# Create a sample Flink job cluster with batch scheduling enabled

Create a sample Flink job cluster with:

```bash
$ kubectl apply -f config/samples/flinkoperator_v1beta1_flinkjobcluster_volcano.yaml
```

and verify the pod is up and running with

```bash
$ kubectl get pod,svc |grep flinkjobcluster
  pod/flinkjobcluster-sample-job-xt4k7                         1/1     Running   0          34s
  pod/flinkjobcluster-sample-jobmanager-6c955f9b4-6mfvk        1/1     Running   0          65s
  pod/flinkjobcluster-sample-taskmanager-77c7bb8778-hmvzm      1/1     Running   0          65s
  pod/flinkjobcluster-sample-taskmanager-77c7bb8778-rp9m4      1/1     Running   0          65s
  service/flinkjobcluster-sample-jobmanager       ClusterIP   10.109.99.119   <none>        6123/TCP,6124/TCP,6125/TCP,8081/TCP   65s
```

verify `job manager` and `task manager` are scheduled by volcano

```bash
$ kubectl get podgroup flink-flinkjobcluster-sample -oyaml
  apiVersion: scheduling.volcano.sh/v1beta1
  kind: PodGroup
  metadata:
    creationTimestamp: "2020-06-29T03:39:48Z"
    generation: 5
    name: flink-flinkjobcluster-sample
    namespace: default
    ownerReferences:
    - apiVersion: flinkoperator.k8s.io/v1beta1
      blockOwnerDeletion: false
      controller: true
      kind: FlinkCluster
      name: flinkjobcluster-sample
      uid: dfb78a1b-6eeb-4bc8-89c5-73a8de4f53e8
    resourceVersion: "70799"
    selfLink: /apis/scheduling.volcano.sh/v1beta1/namespaces/default/podgroups/flink-flinkjobcluster-sample
    uid: a87d9b05-7d33-4529-8e18-f7d6eb42e7aa
  spec:
    minMember: 3
    minResources:
      cpu: 600m
      memory: 3Gi
  status:
    phase: Running
    running: 3
```

As shown above, the podgroup has two pods in running phase and the min required number is 3, that means if the cluster has no enough resources to run both the job manager and task manager, then they are not scheduled.

Also you can check the job manager and task manager's scheduler name is now set to `volcano`

```bash
$ kubectl get pod flinkjobcluster-sample-jobmanager-6c955f9b4-6mfvk -ojsonpath={'.spec.schedulerName'}
volcano

$ kubectl get pod flinkjobcluster-sample-taskmanager-77c7bb8778-hmvzm -ojsonpath={'.spec.schedulerName'}
volcano
```

**Note**: the job's pod is not included in the podgroup.