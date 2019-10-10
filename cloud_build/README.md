# Continuous Integration via Cloud Build

[Cloud Build](https://cloud.google.com/cloud-build/) is configured to trigger on
`/gcbrun` comment from maintainer in PR to the `master` branch. It includes a
sequence of build steps configured in [cloudbuild.yaml](./cloudbuild.yaml):

1.  Create a [builder Docker image](../Dockerfile.builder) which contains the
    project source code and the build dependencies.
1.  Run tests in the builder container.
