# Integeration test with Apache Kafka

This directory includes scripts and Kubernetes manifests for testing the operator with Apache Kafaka, see more details
in this [doc](../../docs/kafka_test_guide.md).

TL;DR

```bash
bash deploy_kafka.sh
kubectl apply -f kafka_click_generator.yaml
kubectl apply -f flinkcluster_clickcount.yaml
```
