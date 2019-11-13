# Integeration test with Apache Kafka

This directory includes scripts and Kubernetes manifests for testing the operator with Apache Kafaka, see more details
in this [doc](../../docs/streaming_test_env_setup.md).

TL;DR

```bash
bash deploy_kafka.sh
kubectl apply -f kafka_click_generator.yaml
kubectl apply -f flinkcluster_clickcount.yaml
```
