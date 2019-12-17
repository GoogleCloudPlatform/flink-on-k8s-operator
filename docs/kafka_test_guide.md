# Testing the Flink Operator with Apache Kafka

Often times developers or users want to be able to quickly try out the Flink Operator with a long-running streaming
application and test features like taking savepoints. The WordCount example including in the Flink release cannot do the
job, because it exits after processing the input file. In this case, you might need to have a streaming data source
(e.g., a Apache Kafka cluster), a streaming data generator and a Flink streaming application for testing purposes. This
document introduces how to setup such a test environment.

## Prerequisites

* a running Kubernetes cluster with enough capacity
* [Helm 2+](https://helm.sh/) initialized in the cluster
* a running Flink Operator in the cluster

## Steps

### 1. Install Kafka

Create namespace `kafka` and install Kafka including Zookeeper in it:

```bash
kubectl create ns kafka
helm repo add incubator http://storage.googleapis.com/kubernetes-charts-incubator
helm install --name my-kafka --namespace kafka incubator/kafka
```

If it failed with error like `User "system:serviceaccount:kube-system:default" cannot get resource "namespaces" in API
group "" in the namespace "kafka"`, you need to grant Helm Tiller the required permissions, for example:

```bash
kubectl create serviceaccount --namespace kube-system tiller
kubectl create clusterrolebinding tiller-cluster-rule --clusterrole=cluster-admin --serviceaccount=kube-system:tiller
helm init --service-account tiller --upgrade
```

After that Kafka broker service will be available at `my-kafka.kafka.svc.cluster.local:9092`, run the following command
to view more details:

```bash
helm status my-kafka
```

### 2. Ingest streaming data into Kafka

Deploy the [ClickGenerator](https://github.com/apache/flink-playgrounds/blob/master/docker/ops-playground-image/java/flink-playground-clickcountjob/src/main/java/org/apache/flink/playgrounds/ops/clickcount/ClickEventGenerator.java) application from the
[Flink Operations Playground](https://ci.apache.org/projects/flink/flink-docs-stable/getting-started/docker-playgrounds/flink-operations-playground.html) to write data to the Kafka cluster.

You can create a Docker image from the [Dockerfile](https://github.com/functicons/flink-playgrounds/blob/master/docker/ops-playground-image/Dockerfile) or use the existing image `functicons/flink-ops-playground:2-FLINK-1.9-scala_2.11` to create a deployment manifest.

`kafka_click_generator.yaml`:

```yaml
apiVersion: apps/v1beta1
kind: Deployment
metadata:
  name: kafka-click-generator
spec:
  replicas: 1
  template:
    metadata:
      labels:
        app: kafka-click-generator
    spec:
      containers:
        - name: kafka-click-generator
          image: functicons/flink-ops-playground:2-FLINK-1.9-scala_2.11
          command: ["java"]
          args:
            - "-classpath"
            - "/opt/ClickCountJob.jar:/opt/flink/lib/*"
            - "org.apache.flink.playgrounds.ops.clickcount.ClickEventGenerator"
            - "--bootstrap.servers"
            - "my-kafka.kafka.svc.cluster.local:9092"
            - "--topic"
            - "input"
```

then run

```bash
kubectl apply -f kafka_click_generator.yaml
```

### 3. Run streaming application with the operator

Now you can create a Flink job cluster CR with the [ClickEventCount](https://github.com/functicons/flink-playgrounds/blob/master/docker/ops-playground-image/java/flink-playground-clickcountjob/src/main/java/org/apache/flink/playgrounds/ops/clickcount/ClickEventCount.java)
application.

`flinkcluster_clickcount.yaml`:

```yaml
apiVersion: flinkoperator.k8s.io/v1alpha1
kind: FlinkCluster
metadata:
  name: flinkcluster-clickcount
spec:
  image:
    name: functicons/flink-ops-playground:2-FLINK-1.9-scala_2.11
  jobManager:
    ports:
      ui: 8081
    resources:
      limits:
        memory: "2Gi"
        cpu: "200m"
  taskManager:
    replicas: 2
    resources:
      limits:
        memory: "2Gi"
        cpu: "200m"
  job:
    jarFile: /opt/ClickCountJob.jar
    className: org.apache.flink.playgrounds.ops.clickcount.ClickEventCount
    args:
      [
        "--bootstrap.servers",
        "my-kafka.kafka.svc.cluster.local:9092",
        "--checkpointing",
        "--event-time",
      ]
    parallelism: 2
```

then run the following command to launch the streaming application:

```bash
kubectl apply -f flinkcluster_clickcount.yaml
```

After that you can check the Flink cluster and job status with:

```bash
kubectl describe flinkclusters flinkcluster-clickcount
```

### 4. Tear down

Delete the FlinkCluster custom resource:

```bash
kubectl delete flinkclusters flinkcluster-clickcount
```

Delete ClickGenerator:

```bash
kubectl delete deployments kafka-click-generator
```

Delete Kafka:

```bash
helm delete --purge my-kafka
```
