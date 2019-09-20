#!/usr/bin/env bash

# Verifies the dependencies required for building the operator are installed
# correctly.

echo "Checking build dependencies..."

# Go
go_version=$(go version | grep -P -o '\d\.\d+\.\d+')
if [[ ! "${go_version}" > "1.12." ]]; then
  echo "Error: Go 1.12+ is required for the build."
  echo "Please install it by following the instructions at ${kustomize_url}"
  exit 1
fi

# Docker
docker_url="https://docs.docker.com/install"
if ! which docker >/dev/null; then
  echo 'Error: Docker is required for the build.'
  echo "Please install it by following the instructions at ${docker_url}"
  exit 1
fi

# Kustomize
kustomize_url="https://github.com/kubernetes-sigs/kustomize/blob/master/docs/INSTALL.md"
kustomize_version=$(kustomize version | grep -P -o 'KustomizeVersion:\d+.\d+.\d+' | grep -P -o '\d+.\d+.\d+')
if [[ ! "${kustomize_version}" > "3.1." ]]; then
  echo "Error: Kustomize v3.1.0+ is required for the build."
  echo "Please install it by following the instructions at ${kustomize_url}"
  exit 1
fi

# KubeBuilder
kubebuilder_url="https://book.kubebuilder.io/quick-start.html#installation"
kubebuilder_version=$(kubebuilder version | grep -P -o 'KubeBuilderVersion:"\d+\.\d+\.\d+"' | grep -P -o '\d+\.\d+\.\d+')
if [[ ! "${kubebuilder_version}" > "2." ]]; then
  echo "Error: KubeBuilder v2+ is required for the build."
  echo "Please install it by following the instructions at ${kubebuilder_url}"
  exit 1
fi

expected_kubebuilder_dir=/usr/local/kubebuilder
if [[ ! -f "${expected_kubebuilder_dir}/bin/kubebuilder" ]]; then
  echo "Error: KubeBuilder must be installed at directory ${expected_kubebuilder_dir}/."
  echo "Consider creating a link from ${expected_kubebuilder_dir} to your current KubeBuilder directory."
  exit 1
fi

echo "Done with dependencies checking."
