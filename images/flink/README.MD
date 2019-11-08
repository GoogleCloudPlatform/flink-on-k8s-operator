# Save savepoints to GCS

Flink Operator supports taking savepoints and storeing it into Google Cloud Storage when deploying a job with a custom Flink image with GCS connector installed. 

## Create custom docker image with GCS connector

Note that there are two ways to provide Flink the job jar. You can directly embed it into the image, e.g. COPY from a local file path in Dockerfile. You can also upload your job jar onto GCS and then reference it in your job YAML file with `jarFile: gs://my-bucket/path-to-jar`. This example takes the first approach and supports downloading the job jar from GCS and putting it under `${FLINK_HOME}/job/` of the image, the job jar URI is set in ${FLINK_JOB_JAR_URI}. You can customize the entrypoint.sh or Dockerfile to upload your job jar in your own way.

After the ${FLINK_JOB_JAR_URI} is set correctly if you also upload job jar in the same way as in example, you can simply run the command below to build the image and push it to GCR.

```bash
make build push IMAGE_TAG=gcr.io/google.com/[MY_PROJECT]/flink:[FLINK_VERSION]-scala_[SCALA_VERSION]-gcs
```

The default Flink version or GCS connector version can be modified in properties file as you need.

## Create a sample job cluster with the custom Flink image

Assume the Flink operator has been up and running, now you can create a job cluster with a Flink job. The operator will automatically take savepoints and save them into GCS.

To create a sample Flink job cluster, simply fork the [sample Flink job cluster](https://github.com/GoogleCloudPlatform/flink-on-k8s-operator/blob/master/config/samples/flinkoperator_v1alpha1_flinkjobcluster.yaml) and point the image to the IMAG_TAG specified above in spec instead of the default Flink version. Then simply run:

```bash
kubectl apply -f flinkoperator_v1alpha1_flinkjobcluster.yaml
```

## Troubleshooting

After the job cluster is up and running, you may find that the auto savepoints failed due to lack of permission to write into the GCS bucket. The errors may look something like this:

```json
{
    "code" : 403,
    "errors" : [ {
        "domain" : "global",
    	"message" : "Insufficient Permission",
    	"reason" : "insufficientPermissions"
    } ],
    "message" : "Insufficient Permission"
}
```

To resolve this issue, several options are available:

**If you want to grant access to entire cluster**

  * Update the default GKE service account associated at the GKE cluster creation time.
    1. [Create a service account](https://cloud.google.com/iam/docs/creating-managing-service-accounts) with proper IAM roles assigneds. E.g. Storage Admin
    2. Set [access scope](https://cloud.google.com/iam/docs/service-accounts#access_scopes) of cluster nodes to be full access: --scopes=storage-full, then run the command:

        ```bash
        gcloud container clusters create example-cluster \
    	      --zone us-central1-a \
    	      --scopes=storage-full --service-account=[YOUR_SERVICE_ACCOUNT]
        ```

**If you want to grant access per application:**

The steps to mount the service account key file in Kubernetes Secret to Flink cluster and also the [core-site.xml](https://hadoop.apache.org/docs/r1.2.1/api/org/apache/hadoop/conf/Configuration.html) in ConfigMap in order to grant write access to GCS are:
Create the Kubernetes Secret with your service account key file.

  * Grant the applications running on GKE access to Google Cloud Platform service, e.g. Google Cloud Storage, by [creating service account key file](https://cloud.google.com/iam/docs/creating-managing-service-account-keys) and importing as a Kubernetes Secrete resource.
  * Create Kubernetes Secret object with your service account key file stored in it, which can be later accessed by a running pod.

    ```bash
    kubectl create secret generic gcp-service-account-key --from-file=gcp_service_account_key.json=[/PATH/TO/KEY.json]
    ```

  * Create a configMap with the filename as key and the content of the file as value of the configMap.

    ```bash
    kubectl create configmap hadoop-config --from-file [/PATH/TO/CORE-SITE.XML]
    ```

  * Next you are able to add both Secret and ConfigMap data to a Volume and mount them onto the path wherever you want them to be in the container. You can udpate accordingly the sample job cluster YAML file to mount config file and key file to your Flink cluster:

    ```yaml
    apiVersion: flinkoperator.k8s.io/v1alpha1
    kind: FlinkCluster
    metadata:
      name: flinkjobcluster-sample
    spec:
      image:
        name: gcr.io/google.com/[MY_PROJECT]/flink:[FLINK_VERSION]-scala_[SCALA_VERSION]-gcs
      jobManager:
        ports:
          ui: 8081
        resources:
          limits:
            memory: "512Mi"
            cpu: "200m"
        mounts:
        - name: gcp-service-account-key-volume
          mountPath: /etc/hadoop/keys
        volumes:
        - name: gcp-service-account-key-volume
          secret:
            secretName: gcp-service-account-key
      taskManager:
        replicas: 2
        resources:
          limits:
            memory: "1024Mi"
            cpu: "200m"
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
        jarFile: ./examples/streaming/WordCount.jar
        className: org.apache.flink.streaming.examples.wordcount.WordCount
        args: ["--input", "./README.txt"]
        parallelism: 2
        savepointsDir: gs://my-bucket/path-to-savepoints
        autoSavepointSeconds: 300
        cleanupPolicy:
          afterJobSucceeds: DeleteCluster
          afterJobFails: KeepCluster
      envVars:
      - name: GOOGLE_APPLICATION_CREDENTIALS
        value: /etc/hadoop/keys/gcp_service_account_key.json
      flinkProperties:
        taskmanager.numberOfTaskSlots: "1"
    ```

  * Finally just simple run apply to deploy the job:

    ```bash
    kubectl apply -f flinkoperator_v1alpha1_flinkjobcluster.yaml
    ```
