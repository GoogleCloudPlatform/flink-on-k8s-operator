
# Image URL to use all building/pushing image targets
IMG ?= flink-operator:latest
# Produce CRDs that work back to Kubernetes 1.11 (no version conversion)
CRD_OPTIONS ?= "crd:trivialVersions=true"

all: flink-operator

# Check build dependencies.
deps:
	bash scripts/check_build_deps.sh

# Run tests
test: generate fmt vet manifests
	go test ./api/... ./controllers/... -coverprofile cover.out
	go mod tidy

# Build flink-operator binary
flink-operator: generate fmt vet
	go build -o bin/flink-operator main.go
	go mod tidy

# Run against the configured Kubernetes cluster in ~/.kube/config
run: generate fmt vet
	go run ./main.go
	go mod tidy

# Install CRDs into a cluster
install: manifests
	kubectl apply -f config/crd/bases

# Deploy cert-manager which is required by webhooks of the operator.
cert-manager:
	bash scripts/deploy_cert_manager.sh

# Deploy the operator in the configured Kubernetes cluster in ~/.kube/config
deploy: manifests cert-manager
	kubectl apply -f config/crd/bases
	kustomize build config/default | kubectl apply -f -

undeploy:
	kustomize build config/default | kubectl delete -f - || true
	kubectl delete -f config/crd/bases || true

# Deploy the sample Flink clusters in the Kubernetes cluster
samples:
	kubectl apply -f config/samples/

# Generate manifests e.g. CRD, RBAC etc.
manifests: controller-gen
	$(CONTROLLER_GEN) $(CRD_OPTIONS) rbac:roleName=manager-role webhook paths="./..." output:crd:artifacts:config=config/crd/bases
	go mod tidy

# Run go fmt against code
fmt:
	go fmt ./...

# Run go vet against code
vet:
	go vet ./...

# Generate code
generate: deps controller-gen
	$(CONTROLLER_GEN) object:headerFile=./hack/boilerplate.go.txt paths=./api/...

# Build the docker image
docker-build: test
	docker build . -t ${IMG}
	@echo "updating kustomize image patch file for Flink Operator resource"
	sed -e 's#image: .*#image: '"${IMG}"'#' ./config/default/manager_image_patch.template >./config/default/manager_image_patch.yaml

# Push the docker image
docker-push:
	docker push ${IMG}

# find or download controller-gen
# download controller-gen if necessary
controller-gen:
ifeq (, $(shell which controller-gen))
	go get sigs.k8s.io/controller-tools/cmd/controller-gen@v0.2.0-beta.2
CONTROLLER_GEN=$(shell go env GOPATH)/bin/controller-gen
else
CONTROLLER_GEN=$(shell which controller-gen)
endif
