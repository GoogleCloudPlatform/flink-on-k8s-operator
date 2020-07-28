# Helm Chart for Flink Job Cluster

This Chart will deploy the Flink Job Cluster.
It is confirmed to work with Flink Operator Images ```gcr.io/flink-operator/flink-operator:v1beta1-6``` and ```gcr.io/flink-operator/flink-operator:v1beta1-5```


## Prerequisites

This Chart requires you to have Flink Operator Running in your Cluster.
It is advised to use at least the following image version of flink-operator: ```gcr.io/flink-operator/flink-operator:v1beta1-6```
since it enables you to use annotations for the Flink-Job-Cluster components e.g. JobManager, TaskManager and Job.

## Installing the Chart

The instructions to install the Flink Job Cluster chart:

1. Prepare a Flink operator image. You should use at least ```gcr.io/flink-operator/flink-operator:v1beta1-6```.

2. Deploy the Flink Operator Helm Chart, follow these [instructions](https://github.com/GoogleCloudPlatform/flink-on-k8s-operator/tree/master/helm-chart/flink-operator)

3. Prepare a Flink-Job-Cluster image. You should use ```flink:latest``` or ```flink:1.9.3``` which has been tested and confirmed to work with flink-job-cluster helm chart.

3. Use the following command to install the flink-job-cluster chart:

  ```bash
  helm template --namespace [NAMESPACE] [RELEASE_NAME] [CHART] -f values.yaml |kubectl apply -f -
  ```

## Uninstalling the Chart

To uninstall your release:

  ```bash
  helm template --namespace [NAMESPACE] [RELEASE_NAME] [CHART] -f values.yaml |kubectl delete -f -
  ```

