#!/usr/bin/env bash

kubectl create ns kafka
kubectl create serviceaccount --namespace kube-system tiller
kubectl create clusterrolebinding tiller-cluster-rule --clusterrole=cluster-admin --serviceaccount=kube-system:tiller
helm init --service-account tiller --upgrade
helm repo add incubator http://storage.googleapis.com/kubernetes-charts-incubator
helm install --name my-kafka --namespace kafka incubator/kafka
