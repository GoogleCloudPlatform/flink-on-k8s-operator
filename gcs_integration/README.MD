# Save savepoints to GCS

Flink Operator supports taking savepoints and storeing it into Google Cloud Storage when deploying a job with a custom flink image with GCS connector installed. 

## Create custom docker image with GCS connector

Note that the job jar needs to be in the same image as GCS connector in order to save savepoints to GCS. This example supports downloading the job jar from GCS and putting it under `${FLINK_HOME}/job/` of the image, the job jar URI is set in ${FLINK_JOB_JAR_URI}. Or you can customize the entrypoint.sh or Dockerfile to upload your job jar into the custom image.

After the ${FLINK_JOB_JAR_URI} is set correctly if you also upload job jar in the same way as in example, you can simply run the command below to build the image and push it to GCR.

    ```bash
    make build push IMAGE_TAG=gcr.io/google.com/<my-project>
    ```

The default flink version or gcs connector version can be modified in properties file as you need.

## Create a sample job cluster with the custom flink image

Assume the flink operator has been up and running, now you can create a job cluster with a flink job. The operator will automatically take savepoints and save them into GCS.

To create a sample flink job cluster, simply fork the [sample Flink job cluster](https://github.com/GoogleCloudPlatform/flink-on-k8s-operator/blob/master/config/samples/flinkoperator_v1alpha1_flinkjobcluster.yaml) and point the image to the IMAG_TAG specified above in spec to instead of default flink version. Then simply run:

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

1) If you want to grant access to entire cluster

  * Update the default GKE service account associated at the GKE cluster creation time.
    1. [Create a service account](https://cloud.google.com/iam/docs/creating-managing-service-accounts) with proper IAM roles assigneds. E.g. Storage Admin
    2. Set [access scope](https://cloud.google.com/iam/docs/service-accounts#access_scopes) of cluster nodes to be full access: -scopes=storage-full, then run the command:

        ```bash
	$ gcloud container clusters create example-cluster \
            --zone us-central1-a \
	    --scopes=storage-full --service-account=[YOUR_SERVICE_ACCOUNT]
	```

2) If you want to grant access per application:

  * Give the applications running on GKE access to Google Cloud Platform service, e.g. Google Cloud Storage, by [creating service account key file](https://cloud.google.com/iam/docs/creating-managing-service-account-keys) and importing as a k8s Secrete resource.
  * Configure the core-site.xml with service account key file path when installing GCS connector for building customized docker image. Follow the example [here](https://github.com/GoogleCloudPlatform/bigdata-interop/blob/master/gcs/INSTALL.md) to configure the core-site.xml for hadoop.

