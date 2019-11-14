# Use GCS as remote storage with custom Flink images

To use [Google Cloud Storage](https://cloud.google.com/storage/) as remote storage for checkpoints, savepoints or job
jar, you can create a custom Docker image based on the official Flink image and add GCS connector in it.

## Create custom Docker image with GCS connector

Edit the Flink, Scala, GCS connections versions in [properties](./properties) file as you need. Then run the following
command to build and push the image:

```bash
make build push IMAGE_TAG=gcr.io/[MY_PROJECT]/flink:[FLINK_VERSION]-scala_[SCALA_VERSION]-gcs
```

## Create a sample job cluster with the custom Flink image

To create a Flink job cluster, you can fork the [sample Flink job cluster](https://github.com/GoogleCloudPlatform/flink-on-k8s-operator/blob/master/config/samples/flinkoperator_v1alpha1_flinkjobcluster.yaml) and point the image to the IMAG_TAG specified above and set
related properties to use GCS. Note that there are two ways to provide Flink the job jar. You can directly embed it into
the custom image with the `COPY` command in Dockerfile. You can also upload your job jar onto GCS and then reference it
in your job YAML file with `jarFile: gs://my-bucket/path-to-jar`.

Example:

```yaml
apiVersion: flinkoperator.k8s.io/v1alpha1
kind: FlinkCluster
metadata:
  name: my-flinkjobcluster
spec:
  image:
    name: gcr.io/[MY_PROJECT]/flink:[FLINK_VERSION]-scala_[SCALA_VERSION]-gcs
  ...
  job:
    jarFile: gs://my-bucket/my-job.jar
    savepoint: gs://my-bucket/path-to-savepoints/savepoint-1234
    savepointsDir: gs://my-bucket/path-to-savepoints
    autoSavepointSeconds: 300
  ...
```

Then run:

```bash
kubectl apply -f my_flinkjobcluster.yaml
```

## Configure permissions for accessing GCS from your job

After the job cluster is up and running, you may find that the auto savepoints failed due to lack of permission to write
into the GCS bucket. The errors may look something like this:

```json
{
  "code" : 403,
  "errors" : [{
    "domain" : "global",
    "message" : "Insufficient Permission",
    "reason" : "insufficientPermissions"
    }],
  "message" : "Insufficient Permission"
}
```

To resolve this issue, several options are available:

### Grant permission to the cluster

Update the default GKE service account associated at the GKE cluster creation time.

1. [Create a service account](https://cloud.google.com/iam/docs/creating-managing-service-accounts) with proper IAM
   roles assigneds. E.g. Storage Admin

2. Set [access scope](https://cloud.google.com/iam/docs/service-accounts#access_scopes) of cluster nodes to be full
   access: `--scopes=storage-full`, then run the command:

   ```bash
   gcloud container clusters create example-cluster \
       --zone us-central1-a \
       --scopes=storage-full --service-account=[YOUR_SERVICE_ACCOUNT]
   ```

### Grant permission to the application

The steps to mount the service account key file in Kubernetes Secret to Flink cluster and the Hadoop
[core-site.xml](./docker/hadoop/core-site.xml) in ConfigMap in
order to grant write access to GCS are:

1. [Creating a service account](https://cloud.google.com/iam/docs/creating-managing-service-account-keys), grant required
  GCS role to it, download the key file (JSON).

2. Create Kubernetes Secret object with your service account key file stored in it, which can be later accessed by a
  running pod.

  ```bash
  kubectl create secret generic gcp-service-account-key --from-file=gcp_service_account_key.json=[/PATH/TO/KEY.json]
  ```

3. Create a configMap with the filename as key and the content of the file as value of the configMap.

  ```bash
  kubectl create configmap hadoop-config --from-file [/PATH/TO/CORE-SITE.XML]
  ```

4. Create Volumes with the Secret and ConfigMap and mount them onto `/etc/hadoop/keys` and `/etc/hadoop/conf`
  respectively, e.g.:

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
      mounts:
      - name: gcp-service-account-key-volume
        mountPath: /etc/hadoop/keys
      volumes:
      - name: gcp-service-account-key-volume
        secret:
          secretName: gcp-service-account-key
    taskManager:
      ...
      mounts:
      - name: gcp-service-account-key-volume
        mountPath: /etc/hadoop/keys
      - name: hadoop-conf-volume
        mountPath: /etc/hadoop/conf
      volumes:
      - name: gcp-service-account-key-volume
        secret:
          secretName: gcp-service-account-key
      - name: hadoop-conf-volume
        configMap:
          name: hadoop-config
    job:
      ...
    envVars:
    - name: GOOGLE_APPLICATION_CREDENTIALS
      value: /etc/hadoop/keys/gcp_service_account_key.json
    ...
  ```

5. Finally deploy the job to the cluster:

  ```bash
  kubectl apply -f my_flinkjobcluster.yaml
  ```

## Reference

[Flink checkpoints to Google Cloud Storage](https://stackoverflow.com/questions/51860988/flink-checkpoints-to-google-cloud-storage)