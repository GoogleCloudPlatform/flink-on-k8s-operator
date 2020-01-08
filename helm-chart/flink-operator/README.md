# Helm Chart for Flink Operator

This is the Helm chart for the Flink operator.

## Install the dependency chart - cert-manager

```bash
# Install the CustomResourceDefinition resources separately
kubectl apply --validate=false -f https://raw.githubusercontent.com/jetstack/cert-manager/release-0.10/deploy/manifests/00-crds.yaml

# Add the Jetstack Helm repository
helm repo add jetstack https://charts.jetstack.io

# Update your local Helm chart repository cache
helm repo update

# Install the cert-manager Helm chart
helm install \
  --name cert-manager \
  --namespace cert-manager \
  --version v0.10.0 \
  jetstack/cert-manager
```

## Installing the chart

The instructions to install the Flink operator chart:

1. Flink operator image needs to be built and pushed. [Here](https://github.com/GoogleCloudPlatform/flink-on-k8s-operator/blob/master/docs/developer_guide.md#build-and-push-docker-image) is how to do it.

2. Run the bash script `update_template.sh` to update the manifest files in templates from the Flink operator source repo (This step is only required if you want to install from the local chart repo).

3. Finally operator chart can be installed by running:

	```bash
	helm repo add flink-operator-repo https://googlecloudplatform.github.io/flink-on-k8s-operator/
	helm install --name [RELEASE_NAME] flink-operator-repo/flink-operator --set operatorImage.name=[IMAGE_NAME]
	```

If you run into webhook related issue when installing operator chart, then disable the cert-manager webhook component and install operator chart again. The command to disable the webhook:

```bash
helm upgrade cert-manager \
   --reuse-values \
   --set webhook.enabled=false
```
