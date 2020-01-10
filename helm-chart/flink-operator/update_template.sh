#!/bin/bash

IMG="${IMG:-flink-operator:latest}"

sed -e 's#image: .*#image: '"${IMG}"'#' ../../config/default/manager_image_patch.template >../../config/default/manager_image_patch.yaml
sed -i '/- \.\.\/crd/d' ../../config/default/kustomization.yaml
kustomize build ../../config/default | tee templates/flink-operator.yaml
sed -i '1s/^/{{- if .Values.rbac.create }}\n/' templates/flink-operator.yaml
sed -i -e "\$a{{- end }}\n" templates/flink-operator.yaml
sed -i 's/flink-operator-system/{{ .Values.flinkOperatorNamespace }}/g' templates/flink-operator.yaml
sed -i 's/replicas: 1/replicas: {{ .Values.replicas }}/g' templates/flink-operator.yaml
sed -i "s/$IMG/{{ .Values.operatorImage.name }}/g" templates/flink-operator.yaml
cp ../../config/crd/bases/flinkoperator.k8s.io_flinkclusters.yaml templates/flink-cluster-crd.yaml
sed -i '1s/^/{{ if .Values.rbac.create }}\n/' templates/flink-cluster-crd.yaml
sed -i -e "\$a{{ end }}\n" templates/flink-cluster-crd.yaml
sed -i 's/{{$clusterName}}.example.com/clusterName.example.com/g' templates/flink-cluster-crd.yaml

git checkout ../../config/default/kustomization.yaml
