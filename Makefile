# Copyright 2019 Google LLC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

include ./env.sh

PROJECT=$(shell gcloud config list --format "value(core.project)" | sed "s%:%/%")
IMAGE_TAG=$(FLINK_OPERATOR_NAME):$(FLINK_OPERATOR_VERSION)
REMOTE_IMAGE_TAG=gcr.io/$(PROJECT)/$(IMAGE_TAG)

.PHONY: image
.PHONY: push
.PHONY: deploy
.PHONY: undeploy
.PHONY: test
.PHONY: clean

default: test

# Build Flink Operator image.
image:
	docker build -t $(IMAGE_TAG) -f build/Dockerfile .

# Push Flink Operator image to GCR.
push: image
	gcloud auth configure-docker
	docker tag $(IMAGE_TAG) $(REMOTE_IMAGE_TAG)
	docker push $(REMOTE_IMAGE_TAG)

# Deploy Flink Operator to Kubernetes cluster.
deploy: push
	bash deployments/deploy.sh

# Delete Flink Operator from Kubernetes cluster.
undeploy: /tmp/flink-operator.yaml
	kubectl delete -f /tmp/flink-operator.yaml
	sleep 10
	kubectl get all -A -l "app.kubernetes.io/name=$(FLINK_OPERATOR_NAME)"

test: image
	docker run $(IMAGE_TAG)

clean:
	docker image rm $(IMAGE_TAG) -f || true
	docker image rm $(REMOTE_IMAGE_TAG) -f || true
	gcloud container images delete $(REMOTE_IMAGE_TAG) --force-delete-tags -q || true
