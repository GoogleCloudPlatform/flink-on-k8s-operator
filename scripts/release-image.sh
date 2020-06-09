#!/usr/bin/env bash

# This script is supposed to be used by project administrators to release
# a new Flink Operator image from a Git commit. It tags the commit with the
# version number (e.g., v1beta1-5), builds and pushes the image to
# gcr.io/flink-operator/flink-operator:<version>.
#
# Usage: run this script from the root of the repo with
#
# scripts/release-image.sh <version-tag e.g., v1beta1-5> <git-commit>

set -euxo pipefail

function main() {
  if (( $# != 2 )); then
    echo "Usage: release-image.sh <version-tag> <git-commit>"
    exit 1
  fi

  local -r version="$1"
  local -r commit="$2"
  local -r image="gcr.io/flink-operator/flink-operator:$version"

  local head
  head="$(git rev-parse HEAD)"
  if [[ "${commit}" != "${head}" ]]; then
    echo "You must run this script from commit ${commit}, current HEAD is ${head}"
    exit 2
  fi

  # Build and push the image.
  make operator-image push-operator-image IMG=${image}

  # Tag commit with the version
  git tag -a "${version}" -m "" "${commit}"
  git push origin "${version}"
}

main "$@"
