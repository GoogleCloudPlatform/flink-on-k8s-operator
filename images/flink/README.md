# Using GCS connector with custom Flink images

To use [Google Cloud Storage](https://cloud.google.com/storage/) as remote storage for checkpoints, savepoints or job
jar, you can create a custom Docker image based on the official Flink image and add
[GCS connector](https://github.com/GoogleCloudPlatform/bigdata-interop/tree/master/gcs) in it.

## Create custom Docker image with GCS connector

Edit the Flink, Scala, GCS connections versions in [properties](./properties) file as you need. Then run the following
command to build and push the image:

```bash
make build push IMAGE_TAG=gcr.io/[MY_PROJECT]/flink:[FLINK_VERSION]-scala_[SCALA_VERSION]-gcs
```

## Before creating a Flink cluster

Before you create a Flink cluster with the custom image, you need to prepare several things:

### GCP service account

1. [Create a service account](https://cloud.google.com/iam/docs/creating-managing-service-accounts) with required
  [GCS IAM roles](https://cloud.google.com/storage/docs/access-control/iam-roles) on the bucket or use an existing
  service account, download the key file (JSON) to your local machine.

2. Create Kubernetes Secret object with your service account key file.

  ```bash
  kubectl create secret generic gcp-service-account-secret --from-file gcp_service_account_key.json=[/PATH/TO/KEY]
  ```

### GCS connector config

GCS connector uses Hadoop [core-site.xml](./docker/hadoop/core-site.xml) as its config file, we need to create a
ConfigMap and make the file available to the Flink TaskManager containers:

1. Create a configMap with the `core-site.xml`.

  ```bash
  kubectl create configmap hadoop-configmap --from-file [/PATH/TO/CORE-SITE.XML]
  ```

## Create a Flink cluster

Now you can create a Flink cluster with the Secret and ConfigMap created in the previous steps.

  ```yaml
  apiVersion: flinkoperator.k8s.io/v1alpha1
  kind: FlinkCluster
  metadata:
    name: my-flinkjobcluster
  spec:
    image:
      name: gcr.io/[MY_PROJECT]/flink:[FLINK_VERSION]-scala_[SCALA_VERSION]-gcs
    jobManager:
      ...
    taskManager:
      ...
    job:
      jarFile: gs://my-bucket/my-job.jar
      savepoint: gs://my-bucket/path-to-savepoints/savepoint-1234
      savepointsDir: gs://my-bucket/path-to-savepoints
      autoSavepointSeconds: 300
    hadoopConfig:
      configMapName: hadoop-configmap
      mountPath: /etc/hadoop/conf
    gcpConfig:
      serviceAccount:
        secretName: gcp-service-account-secret
        keyFile: gcp_service_account_key.json
        mountPath: /etc/gcp/keys
  ```

Then run:

```bash
kubectl apply -f my_flinkjobcluster.yaml
```

The operator will automatically mount the service account key file to the specified path and set the environment
variable `GOOGLE_APPLICATION_CREDENTIALS` to the key file in the JobManager, TaskManager and Job containers.
