#!/usr/bin/env bash

# This script is supposed to be used by project administrators to release
# a new Flink Operator image from a Git commit. It tags the commit with the
# version number (e.g., v1beta1-5), builds and pushes the image to
# gcr.io/flink-operator/flink-operator:<version>.
#
# Usage: run this script from the root of the repo with
#
# scripts/release-image.sh <version e.g., v1beta1-5> <git-commit>

set -euxo pipefail

function main() {
  if (( $# != 2 )); then
    echo "Usage: release-image.sh <version> <git-commit>"
    exit 1
  fi

  local -r version="$1"
  local -r commit="$2"
  local -r image="gcr.io/flink-operator/flink-operator:$version"

  local existing_commit="$(git rev-parse ${version} 2>/dev/null || true)"
  if [[ -n "${existing_commit}" && "${existing_commit}" != "${commit}" && "${existing_commit}" != "${version}" ]]; then
    echo "The version tag ${version} already exists for commit ${existing_commit}"
    exit 2
  fi

  local -r head="$(git rev-parse HEAD)"
  if [[ "${commit}" != "${head}" ]]; then
    echo "You must run this script from commit ${commit}, current HEAD is ${head}"
    exit 3
  fi

  # Build and push the image.
  make operator-image push-operator-image IMG=${image}

  # Update latest
  docker tag ${image} gcr.io/flink-operator/flink-operator:latest
  docker push gcr.io/flink-operator/flink-operator:latest

  # Tag commit with the version
  git tag -a "${version}" -m "" "${commit}"
  git push origin "${version}"
}

main "$@"
