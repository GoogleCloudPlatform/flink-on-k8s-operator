# Create Flink job clusters with Helm Chart

## Overview

This Helm Chart is an addition to the [existing](https://github.com/GoogleCloudPlatform/flink-on-k8s-operator/tree/master/config/samples) way of deploying Flink job clusters.

## Prerequisites


- Fink Operator Image Version:  ```gcr.io/flink-operator/flink-operator:v1beta1-6``` follow these [instructions](https://github.com/GoogleCloudPlatform/flink-on-k8s-operator/tree/master/helm-chart/flink-operator) to deploy the Operator
- Flink Image Version: ```flink:1.9.3``` or ```flink:latest```
- Helm Version 2.x or 3.x


## Installing the Chart

The instructions to install the Flink Job Cluster chart:

1. Clone the repository to your local machine, which has access to your running kubernetes cluster.
  ```bash
  git clone https://github.com/GoogleCloudPlatform/flink-on-k8s-operator.git
  ```
2. Navigate to the following folder: ```/flink-on-k8s-operator/helm-chart```

3. Use the following command to install the Flink job cluster chart:
  ```bash
  helm template --namespace example-namespace flink-job-cluster ./flink-job-cluster -f ./flink-job-cluster/values.yaml |kubectl apply -f -
  ```

***Note*** the ```values.file``` in ```/flink-on-k8s-operator/helm-chart/flink-job-cluster/``` is just an example configuration. You can use your own values.yaml if you wish and edit the parts that you want to change. The current values.yaml has the minimum configuration requirements enabled for the Flink job cluster to start successfully. 

## Uninstalling the Chart

To uninstall your release:

1. Navigate to the following folder: ```/flink-on-k8s-operator/helm-chart```

2. Use the following command to uninstall the Flink job cluster chart:
  ```bash
  helm template --namespace example-namespace flink-job-cluster ./flink-job-cluster -f ./flink-job-cluster/values.yaml |kubectl delete -f -
  ```

## Check the Flink job cluster

After using the helm template command, the following resources will be deployed

- Flink job (1x)
- Flink job manager (1x)
- Flink task manager (2x)

You can check the status of your deployment by using
```bash
kubectl get deployments --namespace=<example-namespace>
```
To check your running Flink job, use
```bash
kubectl get jobs --namespace=<example-namespace>
```

## Flink Operator Releases

You can check which images of the Operator are available at [GoogleCloudPlatform](https://console.cloud.google.com/gcr/images/flink-operator/GLOBAL/flink-operator?gcrImageListsize=30)

## Monitoring

The Flink job cluster comes with a PodMonitor resource, which is the counter part to a ServiceMonitor.
The PodMonitor will use pod labels and configure prometheus to scrape the Flink job cluster metrics. Reason for using the podMonitor is simple, the Flink job cluster does not deploy services.

You can use the following [dashboard](https://grafana.com/grafana/dashboards/10369) in your grafana to monitor the flink-job-cluster.

## Run Jobs with InitContainer

You have the option to download blob containers, which would be your JAR files to execute as jobs, directly into the Flink job cluster pods.
For that, we have used a python library according to the following [documentation](https://docs.microsoft.com/de-de/azure/storage/blobs/storage-quickstart-blobs-python)

Furthermore, there is already an [example](https://github.com/GoogleCloudPlatform/flink-on-k8s-operator/blob/master/config/samples/flinkoperator_v1beta1_remotejobjar.yaml.) on how to run the Flink job cluster with a remote job jar.

## Run Jobs without InitContainer

If you do not want to use a remote job jar, you can simply use the Flink image e.g. ```flink:1.9.3``` and copy your built jar file into that image to create your custom Flink image. This way you can directly start the job without using an InitContainer. Just use your custom Flink image as ```image``` in the values.yaml, and make sure to set the correct path for the job to look for the JAR file.

```yaml
image:
  repository: <your_repository>/<your_custom_flink_image>
  tag: 1.9.3

job:
  # job will look for a JAR file at /JARFiles/<JAR_FILE> and execute it
  # className has to be valid and used in the provided JAR File
  jarFile: /JARfiles/JARfile.jar
```

## Check Running Jobs

You can check running jobs by using the following command:

```kubectl get jobs -n [NAMESPACE]```

## Updating Job

1. Build your new/updated JAR file which will be executed by the Flink job cluster
2. Prepare a new custom Flink Image which has your JAR file included, for example at: /jobs/<your_jar_file>
3. Upload your custom Flink Image to your registry
4. Specify your custom Flink Image in the helm-chart ```values.yaml``` under the ```image``` section
5. Navigate to ```/flink-on-k8s-operator/helm-chart``` and use the helm template command to update your running Flink job cluster.
  ```bash
  helm template --namespace example-namespace flink-job-cluster ./flink-job-cluster -f ./flink-job-cluster/values.yaml |kubectl apply -f -
  ```

## Roadmap

We are planning to extend the chart by adding the possibility to use [strimzi.io](https://strimzi.io/) KafkaUsers. This way, we can automatically spin up new KafkaUsers when deploying a Flink job cluster.
