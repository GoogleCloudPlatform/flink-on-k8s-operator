#!/usr/bin/env bash

# Verifies the dependencies required for building the operator are installed
# correctly.

echo "Checking build dependencies..."

# Go
go_url="https://golang.org/doc/install"
go_major_version=$(go version | grep -P -o '\d\.\d+' | cut -d '.' -f 1)
go_minor_version=$(go version | grep -P -o '\d\.\d+' | cut -d '.' -f 2)
if [[ "${go_major_version}" -lt 1 && "${go_minor_version}" -lt 12 ]]; then
  echo "Error: Go 1.12+ is required for the build"
  echo "Please install it by following the instructions at ${go_url}"
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
kustomize_major_version=$(kustomize version \
    | grep -P -o 'KustomizeVersion:\d+\.' \
    | grep -P -o '\d+')
if [[ -z "${kustomize_major_version}" ]]; then
  kustomize_major_version=$(kustomize version \
    | grep -P -o 'kustomize/v\d+\.' \
    | grep -P -o '\d+')
fi
if [[ "${kustomize_major_version}" -lt 3 ]]; then
  echo "Error: Kustomize v3+ is required for the build."
  echo "Please install it by following the instructions at ${kustomize_url}"
  exit 1
fi

# KubeBuilder
kubebuilder_url="https://book.kubebuilder.io/quick-start.html#installation"
kubebuilder_major_version=$(kubebuilder version \
    | grep -P -o 'KubeBuilderVersion:"\d+\.' \
    | grep -P -o '\d+')
if [[ "${kubebuilder_major_version}" -lt 2 ]]; then
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
