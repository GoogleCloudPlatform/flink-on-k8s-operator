#!/usr/bin/env bash

set -o errexit

readonly HELM_VERSION=2.13.1
readonly CHART_RELEASER_VERSION=0.1.4
readonly KUSTOMIZE_VERSION=3.8.9

echo "Installing Helm..."
curl -LO "https://kubernetes-helm.storage.googleapis.com/helm-v$HELM_VERSION-linux-amd64.tar.gz"
sudo mkdir -p "/usr/local/helm-v$HELM_VERSION"
sudo tar -xzf "helm-v$HELM_VERSION-linux-amd64.tar.gz" -C "/usr/local/helm-v$HELM_VERSION"
sudo ln -sf "/usr/local/helm-v$HELM_VERSION/linux-amd64/helm" /usr/local/bin/helm
rm -f "helm-v$HELM_VERSION-linux-amd64.tar.gz"
helm init --client-only --skip-refresh
helm repo rm stable
helm repo add stable https://charts.helm.sh/stable

echo "Installing chart-releaser..."
chart_releaser_url="https://github.com/helm/chart-releaser/releases/download/v${CHART_RELEASER_VERSION}/chart-releaser_${CHART_RELEASER_VERSION}_Linux_x86_64.tar.gz"
curl -LO "${chart_releaser_url}"
sudo mkdir -p "/usr/local/chart-releaser-v$CHART_RELEASER_VERSION"
sudo tar -xzf "chart-releaser_${CHART_RELEASER_VERSION}_Linux_x86_64.tar.gz" -C "/usr/local/chart-releaser-v$CHART_RELEASER_VERSION"
sudo ln -sf "/usr/local/chart-releaser-v$CHART_RELEASER_VERSION/chart-releaser" /usr/local/bin/chart-releaser
rm -f "chart-releaser_${CHART_RELEASER_VERSION}_Linux_x86_64.tar.gz"

echo "Install Kustomize..."
kustomize_url=$(curl -s "https://api.github.com/repos/kubernetes-sigs/kustomize/releases?per_page=100" | jq -r '.[].assets[] | select(.browser_download_url | test("kustomize(_|.)?(v)?'$KUSTOMIZE_VERSION'_linux_amd64")) | .browser_download_url')
curl -s -S -L "${kustomize_url}" -o kustomize_linux_amd64.tar.gz
sudo mkdir -p /usr/local/kustomize
sudo tar -xzf ./kustomize_linux_amd64.tar.gz -C /usr/local/kustomize
sudo ln -sf /usr/local/kustomize/kustomize /usr/local/bin/kustomize
rm -f ./kustomize_linux_amd64.tar.gz

