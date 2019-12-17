#!/usr/bin/env bash

# Copyright 2019 Google LLC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# Generate Kubernetes manifest file for the Flink Operator and deploy it to a
# running cluster.

set -euxo pipefail

THIS_DIR=$(cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd)
BASE_DIR=$(realpath ${THIS_DIR}/..)
TEMPLATE_FILE=${THIS_DIR}/flink-operator.yaml.template
MANIFEST_FILE=/tmp/flink-operator.yaml

echo -e "\nImporting environment variables...\n"
. ${BASE_DIR}/env.sh
PROJECT=$(gcloud config list --format "value(core.project)" | sed "s#:#/#")
IMAGE_TAG=${FLINK_OPERATOR_NAME}:${FLINK_OPERATOR_VERSION}
REMOTE_IMAGE_TAG=gcr.io/${PROJECT}/${IMAGE_TAG}

echo -e "\nGenerating Flink Operator manifest file...\n"
cat ${TEMPLATE_FILE} \
    | sed "s#{{operator_name}}#${FLINK_OPERATOR_NAME}#" \
    | sed "s#{{operator_namespace}}#${FLINK_OPERATOR_NAMESPACE}#" \
    | sed "s#{{operator_service_account}}#${FLINK_OPERATOR_SERVICE_ACCOUNT}#" \
    | sed "s#{{operator_version}}#${FLINK_OPERATOR_VERSION}#" \
    | sed "s#{{operator_image}}#${REMOTE_IMAGE_TAG}#" \
    >${MANIFEST_FILE}

echo -e "\nGetting Kubernetes cluster info...\n"
kubectl cluster-info

echo -e "\nDeploying Flink Operator to Kubernetes...\n"
kubectl apply -f ${MANIFEST_FILE}

echo -e "\nChecking Flink Operator status...\n"
sleep 30
kubectl get all -A -l 'app.kubernetes.io/name=flink-operator'

echo -e "\nPrinting Flink Operator log...\n"
kubectl logs -l app.kubernetes.io/name=flink-operator -n ${FLINK_OPERATOR_NAMESPACE}