#!/bin/bash

IMG="${IMG:-flink-operator:latest}"

sed -e 's#image: .*#image: '"${IMG}"'#' ../../config/default/manager_image_patch.template >../../config/default/manager_image_patch.yaml
sed -i '/- \.\.\/crd/d' ../../config/default/kustomization.yaml
kubectl kustomize ../../config/default | tee templates/flink-operator.yaml
sed -i '1s/^/{{- if .Values.rbac.create }}\n/' templates/flink-operator.yaml
sed -i -e "\$a{{- end }}\n" templates/flink-operator.yaml
sed -i 's/flink-operator-system/{{ .Values.flinkOperatorNamespace }}/g' templates/flink-operator.yaml
sed -i 's/replicas: 1/replicas: {{ .Values.replicas }}/g' templates/flink-operator.yaml
sed -i "s/$IMG/{{ .Values.operatorImage.name }}/g" templates/flink-operator.yaml
sed -i 's/--watch-namespace=/--watch-namespace={{ .Values.watchNamespace }}/' templates/flink-operator.yaml
cp ../../config/crd/bases/flinkoperator.k8s.io_flinkclusters.yaml ../../config/crd/bases

git checkout ../../config/default/kustomization.yaml
