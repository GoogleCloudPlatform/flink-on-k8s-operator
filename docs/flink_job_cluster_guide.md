# Running Flink-Job-Cluster in Kubernetes

## Overview

to run a Flink-Job-Cluster, you first have to deploy the Flink-Operator. For that, follow the [user_guide](https://github.com/GoogleCloudPlatform/flink-on-k8s-operator/blob/master/docs/user_guide.md)

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

## Monitoring

The Flink-Job-Cluster comes with a PodMonitor ressource, which is the counter part to a ServiceMonitor.
The PodMonitor will use pod labels and configure prometheus to scrape the flink-job-cluster metrics. Reason for using the podMonitor is simple, the flink-job-cluster does not deploy services.

You can use the following [dashboard](https://grafana.com/grafana/dashboards/10369) in your grafana to monitor the flink-job-cluster.

## Run Jobs with InitContainer

You have the option to download blob containers, which would be your JAR files to execute as jobs, directly into the flink-job-cluster pods.
For that, we have used a python library according to the following [documentation](https://docs.microsoft.com/de-de/azure/storage/blobs/storage-quickstart-blobs-python)

This way, you can directly download your jar files from a blob storage at Microsoft Azure.
A basic script that is confirmed to work could look like this:

```bash
import os, uuid
from azure.storage.blob import BlobServiceClient, BlobClient, ContainerClient

try:
    connect_str = os.getenv('AZURE_STORAGE_CONNECTION_STRING')
    container_name = os.getenv('CONTAINER_NAME')
    blob_name = os.getenv('BLOB_NAME')
    #example for a valid blob name: "com/test/test-java-shared-library/0.1.0-SNAPSHOT/test-java-shared-library.jar"

    blob = BlobClient.from_connection_string(connect_str, container_name, blob_name)

    with open("./JARfile.jar", "wb") as my_blob:
        blob_data = blob.download_blob()
        blob_data.readinto(my_blob)

except Exception as ex:
    print('Exception:')
    print(ex)
```
Either export the above envVars on your local machine, or use the flink-job-clusters envVars section to configure them.

The init container can be anything that is able to run ```python3``` or ```python3.5``` commands.
The above script would be executed by the InitContainer, and the downloaded JAR File should then be copied to the path in which flink-job-cluster will look for JAR files to execute the job.

## Run Jobs without InitContainer

You could also use the flink image e.g. ```flink:1.9.3``` and copy your built jar file into that image to create your custom flink-image. This way you can directly start the job without using an InitContainer. Just make sure to set the correct path for the job to look for the JAR file (see values.yaml of the helm-chart).

## Check Running Jobs

You can check running jobs by using the following command:

```kubectl get jobs -n [NAMESPACE]```

## Updating Job

Since the flink-job-cluster is only deployed while the job is running, you have to re-deploy the helm chart with the new JAR file ready to be downloaded and started.
If you are using the Jobs without an InitContainer, then you can simply update your JAR in the custom flink-image and re-deploy the helm-chart.

## Roadmap

We are planning to extend the chart by adding the possibility to use [strimzi.io](https://strimzi.io/) KafkaUsers. This way, we can automatically spin up new KafkaUsers when deploying a flink job cluster.

