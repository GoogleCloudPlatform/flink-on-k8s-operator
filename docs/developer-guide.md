# Developer Guide

## Build with Docker

The [Dockerfile](../docker/Dockerfile) specifies the dependencies and steps to build the operator from the source code. Run the following command from the root of the repo to start building a Docker image for the operator:

```bash
$ docker build -t <image-tag> -f docker/Dockerfile .
```

The operator image is based on [alpine image](https://hub.docker.com/_/alpine) by default, but you can use your own base image (e.g., an image with some custom dependencies) by specifying the argument `BASE_IMAGE`:

```bash
$ docker build --build-arg BASE_IMAGE=<base-image> \
    -t <image-tag> -f docker/Dockerfile .
```
