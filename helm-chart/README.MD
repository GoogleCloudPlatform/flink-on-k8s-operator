### Helm Chart for Flink Operator

This is the Helm chart for the flink operator.

#### Installing the chart

The chart can be installed by running:

```bash
$ helm install ./helm-chart --namespace flink-operator --set image.repository=[IMAGE_REPOSITORY] --set image.tag=[IMAGE_TAG]
```

Note that you need to use the `--namespace` flag during `helm install` to specify in which namespace you want to install the operator. The namespace can be existing or not. When it's not available, Helm would take care of creating the namespace.

