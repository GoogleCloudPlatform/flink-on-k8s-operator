# Helm Chart for Flink Operator

This is the Helm chart for the Flink operator.

## Installing the chart

The instructions to install the Flink operator chart:

1. Flink operator image needs to be built and pushed. [Here](https://github.com/GoogleCloudPlatform/flink-on-k8s-operator/blob/master/docs/developer_guide.md#build-and-push-docker-image) is how to do it.

2. Run the bash script `update_template.sh` to update the manifest files in templates from the Flink operator source repo (This step is only required if you want to install from the local chart repo).

3. Finally operator chart can be installed by running:

	```bash
	helm repo add flink-operator-repo https://googlecloudplatform.github.io/flink-on-k8s-operator/
	helm install --name [RELEASE_NAME] flink-operator-repo/flink-operator --set operatorImage.name=[IMAGE_NAME]
	```
    or to install it using local repo with command:

    ```bash
    helm install --name [RELEASE_NAME] . --set operatorImage.name=[IMAGE_NAME]
    ```
