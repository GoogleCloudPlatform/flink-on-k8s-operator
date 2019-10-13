# Docker image used for building and running unit tests for the Flink Operator.
#
# It installs required build dependencies (e.g., Go 12+), copies the project
# source code into the container, build and run tests.
#
# Usage: 
#
# docker build -t flink-operator-builder -f Dockerfile.builder .
# docker run flink-operator-builder


FROM ubuntu:18.04

RUN apt update && apt install -yqq curl git make gcc

# Install Go
RUN curl -s https://dl.google.com/go/go1.12.10.linux-amd64.tar.gz | tar -xz -C /usr/local/
ENV GOROOT=/usr/local/go
ENV PATH=${PATH}:${GOROOT}/bin

# Install Kubebuilder
RUN curl -sL https://go.kubebuilder.io/dl/2.0.1/linux/amd64 | tar -xz -C /usr/local/ \
    && mv /usr/local/kubebuilder_2.0.1_linux_amd64 /usr/local/kubebuilder
ENV PATH=${PATH}:/usr/local/kubebuilder/bin

WORKDIR /workspace/

# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum

# Cache deps before building so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer.
RUN go mod download

# Copy the project source code
COPY . /workspace/

# Build the flink-operator binary
RUN make build
