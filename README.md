
## Introduction
This branch is built based on the flink-operator-0.1.1 of the [flink-operator](https://github.com/GoogleCloudPlatform/flink-on-k8s-operator).

In this branch, we support the feature [flink native setup on kubernetes](https://ci.apache.org/projects/flink/flink-docs-release-1.10/ops/deployment/native_kubernetes.html), which including two kinds of flink clusters.
* Support flink native session cluster (Version >= 1.10 of Apache Flink)
* Support flink native per-job cluster (Version >= 1.11 of Apache Flink)

>Note about the feature `flink native per-job cluster`

This feature has not been released in Apache Flink 1.10 yet. Feel free to contact us if you want to test or use this feature in advance. 

### The simple comparation between the different flink cluster as below.


> Flink session cluster

JobManager and TaskManager are created in advance，the number of taskmanager is specified by user

> Flink per-job cluster

JobManager and TaskManager are created after job submitted，the number of taskmanager is specified by user

> Flink native session cluster

JobManager is created in advance，taskmanger is created on-demand

>Flink native per-job cluster

JobManager and TaskManager are created after job submitted，taskmanger is created on-demand


## **Installation** && Testing

In this section, we introduce how to install the flink-operator by using the helm charts and then we use some examples to simply demonstrate the process of creating the flink cluster. 

## 1. Helm3 installed on your local machine

You can install the helm3 from the [Helm 3+](https://helm.sh/docs/intro/install/) or follow the setups as below.

```
export HELM_VERSION=v3.2.0
export OS=linux
export ARCH=amd64
wget https://get.helm.sh/helm-${HELM_VERSION}-${OS}-${ARCH}.tar.gz
tar -zxvf helm-*.tar.gz
chmod u+x ${OS}-amd64/helm
sudo mv ${OS}-amd64/helm /usr/local/bin
```

>For MacOS amd64, the os name is `darwin`

>For windows amd64, the os name is `windows`


## 2. Flink-operator deploy

```
# add repo
helm repo add flink-operator-repo https://flink-operator.tencentcloudcr.com/chartrepo/flink-operator-chart

# search the version available
helm search repo flink-operator-repo
NAME                              	CHART VERSION	APP VERSION	DESCRIPTION
flink-operator-repo/flink-operator	0.1.1        	1.0        	A Helm chart for flink on Kubernetes operator

# install flink operator
helm install your-flink-operator-name flink-operator-repo/flink-operator

# check helm list
helm  list 

# check if the flink-operator is installed succefully
kubectl get deploy -n flink-operator-system
NAME                                READY   UP-TO-DATE   AVAILABLE   AGE
flink-operator-controller-manager   1/1     1            1           49s

```

## 3. Examples 

### 3.1 native session cluster

```
cat <<EOF | kubectl apply --filename -
apiVersion: flinkoperator.k8s.io/v1beta1
kind: FlinkCluster
metadata:
  name: native-flinksessioncluster-sample
spec:
  image:
    name: flink:1.10.0
    pullPolicy: Always
  nativeSessionClusterJob:
    flinkClusterID: native-flinksessioncluster-sample
    #entryPath: "/opt/flink/bin/kubernetes-entry.sh"
EOF
```

check the resources of the cluster
```
kubectl get svc
NAME                                     TYPE           CLUSTER-IP   EXTERNAL-IP     PORT(S)                      AGE
native-flinksessioncluster-sample        ClusterIP      10.0.0.236   <none>          8081/TCP,6123/TCP,6124/TCP   2m9s
native-flinksessioncluster-sample-rest   LoadBalancer   10.0.0.3     118.89.75.248   8081:30616/TCP               2m8s

kubectl get deploy
NAME                                READY   UP-TO-DATE   AVAILABLE   AGE
native-flinksessioncluster-sample   1/1     1            1           56s

kubectl get pod
NAME                                                 READY   STATUS      RESTARTS   AGE
native-flinksessioncluster-sample-85b5c66ff8-tpbpc   1/1     Running     0          59s
native-flinksessioncluster-sample-job-6cmzj          0/1     Completed   0          66s

```

submit job to the native session cluster
```
cat <<EOF | kubectl apply --filename -
apiVersion: batch/v1
kind: Job
metadata:
  name: my-job-submitter
spec:
  template:
    spec:
      containers:
      - name: wordcount
        image: flink:1.10.0
        args:
        - /opt/flink/bin/flink
        - run
        - -d
        - -e
        - kubernetes-session
        - -Dkubernetes.cluster-id=native-flinksessioncluster-sample
        - /opt/flink/examples/streaming/WordCount.jar
      restartPolicy: Never
EOF
```

check the resources
```
kubectl get pod|grep taskmanager
native-flinksessioncluster-sample-taskmanager-1-1    1/1     Running     0          53s

check the output
also can check it from the jobmanager web-ui -> taskmanager
2020-04-28 11:35:21,162 INFO  org.apache.flink.runtime.state.heap.HeapKeyedStateBackend     - Initializing heap keyed state backend with stream factory.
(to,1)
(be,1)
...
(sins,1)
(remember,1)
(d,4)
```

### 3.2 native job cluster
Version >= 1.11 of Apache Flink
hadoop clusted needed.
```
cat <<EOF | kubectl apply --filename -
apiVersion: flinkoperator.k8s.io/v1beta1
kind: FlinkCluster
metadata:
  name: native-flinkjobcluster-sample
spec:
  image:
    name: ccr.ccs.tencentyun.com/kinderyj/flink-test:nativeperjob1.10
    pullPolicy: IfNotPresent
  nativeJobClusterJob:
    flinkClusterID: native-flinksessioncluster-sample
    jarFile: /opt/flink/examples/streaming/WordCount.jar
  hadoopConfig:
    configMapName: hadoop
    mountPath: /etc/hadoop
EOF
```

check the resources
```
kubectl get pod
NAME                                                             READY   STATUS      RESTARTS   AGE
native-flinkjobcluster-sample-5765b5b695-6lb56                   1/1     Running     0          23s
native-flinkjobcluster-sample-native-per-job-cluster-job-slwjl   0/1     Completed   0          26s
native-flinkjobcluster-sample-taskmanager-1-1                    1/1     Running     0          14s

kubectl get svc
NAME                                 TYPE           CLUSTER-IP      EXTERNAL-IP      PORT(S)             AGE        3d3h
native-flinkjobcluster-sample        ClusterIP      None            <none>           6123/TCP,6124/TCP   26s
native-flinkjobcluster-sample-rest   LoadBalancer   172.16.253.27   ******   8081:31390/TCP      26s
```

### 3.3 session cluster

```
cat <<EOF | kubectl apply --filename -
apiVersion: flinkoperator.k8s.io/v1beta1
kind: FlinkCluster
metadata:
  name: flinksessioncluster-sample
spec:
  image:
    name: flink:1.10.0
    pullPolicy: Always
  jobManager:
    accessScope: Cluster
    ports:
      ui: 8081
    resources:
      limits:
        memory: "1024Mi"
        cpu: "200m"
  taskManager:
    replicas: 1
    resources:
      limits:
        memory: "2024Mi"
        cpu: "200m"
    volumes:
      - name: cache-volume
        emptyDir: {}
    volumeMounts:
      - mountPath: /cache
        name: cache-volume
    sidecars:
      - name: sidecar
        image: alpine
        command:
          - "sleep"
          - "10000"
  envVars:
    - name: FOO
      value: bar
  flinkProperties:
    taskmanager.numberOfTaskSlots: "1"
EOF
```

check the flink cluster, it contains 1 jobmanager and 2 taskmanagers.

```
kubectl get flinkcluster
NAME                         AGE
flinksessioncluster-sample   9s

kubectl get pod
NAME                                                     READY   STATUS      RESTARTS   AGE
flinksessioncluster-sample-jobmanager-775d469566-7hgdf   1/1     Running     0          2m41s
flinksessioncluster-sample-taskmanager-bfd8b7745-8qjm4   2/2     Running     1          2m41s
```

submit job to the cluster

```
cat <<EOF | kubectl apply --filename -
apiVersion: batch/v1
kind: Job
metadata:
  name: my-job-submitter
spec:
  template:
    spec:
      containers:
      - name: wordcount
        image: flink:1.10.0
        args:
        - /opt/flink/bin/flink
        - run
        - -m
        - flinksessioncluster-sample-jobmanager:8081
        - /opt/flink/examples/batch/WordCount.jar
        - --input
        - /opt/flink/README.txt
      restartPolicy: Never
EOF
```

check the result
```
kubectl logs my-job-submitter-cdnt6
Printing result to stdout. Use --output to specify output path.
Job has been submitted with JobID d624912f912326f1d43e9cace55d07e2
Program execution finished
Job with JobID d624912f912326f1d43e9cace55d07e2 has finished.
Job Runtime: 5804 ms
Accumulator Results:
- 2e194aa10c33c491a7e6d35c1f6fdeb2 (java.util.ArrayList) [111 elements]


(1,1)
(13,1)
...
(you,2)
(your,1)
```

delete the cluster
```
kubectl delete flinkcluster flinksessioncluster-sample
flinkcluster.flinkoperator.k8s.io "flinksessioncluster-sample" deleted
```


### 3.4 job cluster

```
cat <<EOF | kubectl apply --filename -
apiVersion: flinkoperator.k8s.io/v1beta1
kind: FlinkCluster
metadata:
  name: flinkjobcluster-sample
spec:
  image:
    name: flink:1.10.0
  jobManager:
    ports:
      ui: 8081
    resources:
      limits:
        memory: "1024Mi"
        cpu: "200m"
  taskManager:
    replicas: 2
    resources:
      limits:
        memory: "2024Mi"
        cpu: "200m"
  job:
    jarFile: /opt/flink/examples/streaming/WordCount.jar
    className: org.apache.flink.streaming.examples.wordcount.WordCount
    args: ["--input", "/opt/flink/README.txt"]
    parallelism: 2
  flinkProperties:
    taskmanager.numberOfTaskSlots: "1"
EOF
```

```
kubectl get pod
NAME                                                  READY   STATUS    RESTARTS   AGE
flinkjobcluster-sample-job-7pcgh                      1/1     Running   0          44s
flinkjobcluster-sample-jobmanager-5998599df8-hbc89    1/1     Running   0          74s
flinkjobcluster-sample-taskmanager-5d66ccd896-85xzp   1/1     Running   0          74s
flinkjobcluster-sample-taskmanager-5d66ccd896-v7kxc   1/1     Running   0          74s

the taskmanager and jobmanager are cleaned automatically when the job is completed.
kubectl get pod
NAME                                                  READY   STATUS        RESTARTS   AGE
flinkjobcluster-sample-job-7pcgh                      0/1     Completed     0          52s
flinkjobcluster-sample-jobmanager-5998599df8-hbc89    0/1     Terminating   0          82s
flinkjobcluster-sample-taskmanager-5d66ccd896-85xzp   0/1     Terminating   0          82s
flinkjobcluster-sample-taskmanager-5d66ccd896-v7kxc   0/1     Terminating   0          82s
```

```
check the result
kubectl logs flinkjobcluster-sample-taskmanager-5d66ccd896-6r9z6
...
1> (our,2)
...
1> (software,4)
```

delete the job session
```
kubectl delete flinkcluster flinkjobcluster-sample
```

### 3.5 job cluster with initContainer

```
cat <<EOF | kubectl apply --filename -
apiVersion: flinkoperator.k8s.io/v1beta1
kind: FlinkCluster
metadata:
  name: flinkjobcluster-sample
spec:
  image:
    name: flink:1.10.0
  jobManager:
    ports:
      ui: 8081
    resources:
      limits:
        memory: "1024Mi"
        cpu: "200m"
  taskManager:
    replicas: 2
    resources:
      limits:
        memory: "2024Mi"
        cpu: "200m"
  job:
    jarFile: /cache/WordCount.jar
    className: org.apache.flink.streaming.examples.wordcount.WordCount
    args: ["--input", "/opt/flink/README.txt"]
    parallelism: 2
    volumes:
      - name: cache-volume
        emptyDir: {}
    volumeMounts:
      - mountPath: /cache
        name: cache-volume
    initContainers:
      - name: tke-downloader
        image: centos
        command: ["curl"]
        args:
          - "https://flinkoperator-1251707795.cos.ap-guangzhou.myqcloud.com/WordCount.jar"
          - "-o"
          - "/cache/WordCount.jar"
  flinkProperties:
    taskmanager.numberOfTaskSlots: "1"
EOF
```

### 3.6 job cluster with initContainer and sidecar

```
cat <<EOF | kubectl apply --filename -
apiVersion: flinkoperator.k8s.io/v1beta1
kind: FlinkCluster
metadata:
  name: flinkjobcluster-sample
spec:
  image:
    name: flink:1.10.0
  jobManager:
    ports:
      ui: 8081
    resources:
      limits:
        memory: "1024Mi"
        cpu: "200m"
  taskManager:
    replicas: 2
    resources:
      limits:
        memory: "2024Mi"
        cpu: "200m"
    sidecars:
      - name: sidecar-worker
        image: centos
        volumeMounts:
        - name: shared-data
          mountPath: /pod-data
        command: ["/bin/sh"]
        args: ["-c", "echo Hello from the sidecar container > /pod-data/index.html;sleep 999d"]
    volumeMounts:
    - name: shared-data
      mountPath: /tmp/data
    volumes:
    - name: shared-data
      emptyDir: {}
  job:
    jarFile: /cache/WordCount.jar
    className: org.apache.flink.streaming.examples.wordcount.WordCount
    args: ["--input", "/opt/flink/README.txt"]
    parallelism: 2
    volumes:
      - name: cache-volume
        emptyDir: {}
    volumeMounts:
      - mountPath: /cache
        name: cache-volume
    initContainers:
      - name: tke-downloader
        image: centos
        command: ["curl"]
        args:
          - "https://flinkoperator-1251707795.cos.ap-guangzhou.myqcloud.com/WordCount.jar"
          - "-o"
          - "/cache/WordCount.jar"
  flinkProperties:
    taskmanager.numberOfTaskSlots: "1"
EOF
```

### 3.7 Testing the Flink Operator with Apache Kafka in flink's native cluster

command in the container
```
./bin/flink run -d -e kubernetes-session -Dkubernetes.cluster-id=native-flinksessioncluster-sample ./StateMachineExample.jar --kafka-topic topictest --brokers 10.0.0.10:9092
```

produce messages to kafka
```
./bin/kafka-console-producer.sh --broker-list 10.0.0.10:9092 --topic topictest
>
>test001
>test002
>test003
>
```

check the result from the jobmanager's web UI

```
2020-04-28 13:01:46,213 INFO  org.apache.kafka.common.utils.AppInfoParser                   - Kafka version : 0.10.2.2
2020-04-28 13:01:46,213 INFO  org.apache.kafka.common.utils.AppInfoParser                   - Kafka commitId : cd80bc412b9b9701
2020-04-28 13:01:46,258 INFO  org.apache.kafka.clients.consumer.internals.AbstractCoordinator  - Discovered coordinator 10.0.0.10:9094 (id: 2147473253 rack: null) for group .

test001
test002
test003
```
