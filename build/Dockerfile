# Copyright 2019 Google LLC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     https://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

ARG BASE_IMAGE="alpine:3.10.0"

# Build the operator in a temporary builder image.
FROM golang:1.12.5-alpine as builder-image
RUN apk add --no-cache bash git
WORKDIR ${GOPATH}/src/github.com/GoogleCloudPlatform/flink-on-k8s-operator
COPY . ./
RUN go generate && CGO_ENABLED=0 GOOS=linux go build -o /usr/bin/flink-operator

# Create the operator image by copying the operator atop the base image.
FROM ${BASE_IMAGE}
COPY --from=builder-image /usr/bin/flink-operator /usr/bin/
RUN apk add --no-cache bash openssl curl tini
COPY build/entrypoint.sh /usr/bin/
ENTRYPOINT ["bash", "/usr/bin/entrypoint.sh"]
