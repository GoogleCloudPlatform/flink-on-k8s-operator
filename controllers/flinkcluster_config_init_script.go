/*
Copyright 2021 Google LLC.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

// This script is part of the cluster's ConfigMap and mounted into the container
// at `/opt/flink-operator/config-init.sh` for config initialization.
const configInitScript = `
#! /usr/bin/env bash

# This script copies config files which are mounted from ConfigMap 
# then merges mounted secrets into flink-conf.yaml.

set -euo pipefail

function init_config() {
  local config_dir="$1"
  local config_secret_dir="$2"
  local output_dir="$3"

  cp -R "${config_dir}/." "${output_dir}"

  for file in "${config_secret_dir}"/*; do
    if [[ ! -f "${file}" ]]; then
      continue
    fi

    local key
    local value
    key=$(basename "${file}")
    value=$(cat "${file}")
    case "${key}" in
      "jobmanager.rpc.address") continue;;
      "jobmanager.rpc.port") continue;;
      "blob.server.port") continue;;
      "query.server.port") continue;;
      "rest.port") continue;;
      *) printf "%s: %s\n" "${key}" "${value}" >> "${output_dir}/flink-conf.yaml";;
    esac
  done
}

function main() {
  echo -e "---------- Initializing config file ----------"
  if ! init_config "$@"; then
    exit 1
  fi
}

main "$@"
`
