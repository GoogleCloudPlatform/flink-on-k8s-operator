# Create Flink job clusters with Helm Chart

## Overview

This Helm Chart is an addition to the [existing](https://github.com/GoogleCloudPlatform/flink-on-k8s-operator/tree/master/config/samples) way of deploying Flink job clusters.

### Why use the Helm Chart?
A typical Helm chart will usually include all of the manifests which you would manually apply with kubectl as templates, along with a ```values.yaml``` file for quick management of user preferences, so it becomes a one-step process to manage all of these resources as a single resource. Since Flink job clusters are currently deployed with just one YAML file, it might seem like the helm chart is unnecessary. However, the more components are added in the future, such as a ```PodMonitor``` or ```Services```, the easier it will be to manage those manifests from a central ```values.yaml```. Helm also supports various deployment checks before and after deployment so it integrates well with CI/CD pipelines. Some of these benefits are listed below:

- Easy configuration as you just have to configure or enable features in the ```values.yaml``` without much knowledge of the entire chart
- You can use helm operations such as ```helm --dry run``` to check for any errors before deployment
- Automated rollback to a previous functioning release with the ```--atomic``` flag
- Manual rollbacks to previous revisions possible with ```helm rollback```
- Helm includes release versioning which can be checked by using the ```helm list <namespace>``` command

## Prerequisites


- Fink Operator Image Version:  ```gcr.io/flink-operator/flink-operator:v1beta1-6``` follow these [instructions](https://github.com/GoogleCloudPlatform/flink-on-k8s-operator/tree/master/helm-chart/flink-operator) to deploy the Operator
- Flink Image Version: ```flink:1.9.3``` or ```flink:latest```
- [Helm](https://helm.sh/docs/helm/helm_install/) version 2.x or 3.x

Optional:
- [Prometheus-Operator](https://github.com/GoogleCloudPlatform/flink-on-k8s-operator/blob/master/docs/user_guide.md#monitoring-with-prometheus) to use the custom resource ```PodMonitor``` in order to scrape flink-job-cluster metrics


## Installing the Chart

The instructions to install the Flink Job Cluster chart:

1. Clone the repository to your local machine, which has access to your running kubernetes cluster.
  ```bash
  git clone https://github.com/GoogleCloudPlatform/flink-on-k8s-operator.git
  ```
2. Navigate to the following folder: ```/flink-on-k8s-operator/helm-chart```

3. Use the following command to dry-run the Flink job cluster chart:
  ```bash
  helm install --dry-run --namespace=<namespace> flink-job-cluster ./flink-job-cluster -f ./flink-job-cluster/values.yaml
  ```
  The ```dry-run``` flag will render the templated yaml files. It is used to debug your chart. You'll be notified if there is any error in the chart configuration.

4. Use the following command to install the Flink job cluster chart:
  ```bash
  helm install --namespace=<namespace> flink-job-cluster ./flink-job-cluster -f ./flink-job-cluster/values.yaml
  ```

  Afterwards, you should see the following output in your console:
  ```bash
  NAME: flink-job-cluster
  LAST DEPLOYED: Tue Aug  4 10:39:10 2020
  NAMESPACE: <namespace>
  STATUS: deployed
  REVISION: 1
  TEST SUITE: None
  ```
***Note*** the ```values.file``` in ```/flink-on-k8s-operator/helm-chart/flink-job-cluster/``` is just an example configuration. You can use your own values.yaml if you wish and edit the parts that you want to change. The current values.yaml has the minimum configuration requirements enabled for the Flink job cluster to start successfully.

## Uninstalling the Chart

To uninstall your release:

1. Use the following command to list the Flink job cluster release:
  ```bash
  helm list --namespace=<namespace>
  ```
2. Find your release name and delete it:
  ```bash
  helm delete <release_name> --namespace=<namespace>
  ```

## Check the Flink job cluster

After using the helm command, the following resources will be deployed

- Flink job (1x)
- Flink job manager (1x)
- Flink task manager (2x)

You can check the status of your deployment by using
```bash
kubectl get deployments --namespace=<namespace>
```

## Flink Operator Releases

You can check which images of the Operator are available at [GoogleCloudPlatform](https://console.cloud.google.com/gcr/images/flink-operator/GLOBAL/flink-operator?gcrImageListsize=30)

## Monitoring

The Flink job cluster comes with a PodMonitor resource, which is the counter part to a ServiceMonitor.
The PodMonitor will use pod labels and configure prometheus to scrape the Flink job cluster metrics. Reason for using the PodMonitor is simple, the Flink job cluster does not deploy services.

You can use the following [dashboard](https://grafana.com/grafana/dashboards/10369) in your grafana to monitor the flink-job-cluster.

## Run Jobs with InitContainer

You have the option to download job jars to be executed as jobs, directly into the Flink job cluster pods.
There is already an [example](https://github.com/GoogleCloudPlatform/flink-on-k8s-operator/blob/master/config/samples/flinkoperator_v1beta1_remotejobjar.yaml). on how to run the Flink job cluster with a remote job jar.

## Run Jobs without InitContainer

If you do not want to use a remote job jar, you can simply use the Flink image e.g. ```flink:1.9.3``` and copy your built jar file into that image to create your custom Flink image. This way you can directly start the job without using an InitContainer. Just use your custom Flink image as ```image``` in the values.yaml, and make sure to set the correct path for the job to look for the JAR file.

```yaml
image:
  repository: <your_repository>/<your_custom_flink_image>
  tag: 1.9.3

job:
  # job will look for a JAR file at ./examples/streaming/WordCount.jar and execute it
  # className has to be valid and used in the provided JAR File
  jarFile: ./examples/streaming/WordCount.jar
```

## Check Running Jobs

You can check running jobs by using the following command:

```kubectl get jobs -n <namespace>```

## Updating Job

1. Build your new/updated JAR file which will be executed by the Flink job cluster
2. Prepare a new custom Flink Image which has your JAR file included, for example at: /JARFiles/<JAR_FILE>
3. Adjust the path for the jar file in ```values.yaml``` at job.jarFile from "./examples/streaming/WordCount.jar" to "/JARFiles/<JAR_FILE>"
3. Upload your custom Flink Image to your registry
4. Specify your custom Flink Image in the helm-chart ```values.yaml``` under the ```image``` section
5. Navigate to ```/flink-on-k8s-operator/helm-chart``` and use the helm command to update your running Flink job cluster.
  ```bash
  helm upgrade --namespace=<namespace> flink-job-cluster ./flink-job-cluster -f ./flink-job-cluster/values.yaml
  ```

## Roadmap

We are planning to extend the chart by adding the possibility to use [strimzi.io](https://strimzi.io/) KafkaUsers. This way, we can automatically spin up new KafkaUsers when deploying a Flink job cluster.
