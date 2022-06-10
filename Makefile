SHELL=/usr/bin/env bash -o pipefail

# Image URL to use all building/pushing image targets
IMG ?= "matrixorigin/matrixone-operator:latest"
# Produce CRDs that work back to Kubernetes 1.11 (no version conversion)
CRD_OPTIONS ?= "crd:maxDescLen=0,trivialVersions=true,generateEmbeddedObjectMeta=true"
MIMG ?= "matrixorigin/matrixone:kc"
BIMG ?= "matrixorigin/mysql-tester:latest"
PROXY ?= https://goproxy.cn,direct
BRANCH ?= main
TOOLS_BIN_DIR ?= $(shell pwd)/tmp/bin
CONTROLLER_GEN_BINARY := $(TOOLS_BIN_DIR)/controller-gen
GOLANGCILINTER_BINARY=$(TOOLS_BIN_DIR)/golangci-lint
TOOLING=$(CONTROLLER_GEN_BINARY)  $(GOLANGCILINTER_BINARY)

export PATH := $(TOOLS_BIN_DIR):$(PATH)


# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif


all: manager

# Build matrixone docker image
mo-build:
	cd third_part/mo-docker && docker build . -t $(MIMG) --build-arg PROXY=$(PROXY) --build-arg BRANCH=$(BRANCH)

# push matrixone docker image
mo-push:
	docker push $(MIMG)

# Run tests
test: generate fmt vet manifests
	go test ./... -coverprofile cover.out

# build mysql-tester image
# repo: https://github.com/matrixorigin/mysql-tester
bvt-build:
	docker build -f tools/bvt-test/Dockerfile . -t $(BIMG) --build-arg PROXY=$(PROXY)

# Build manager binary
manager: generate fmt vet
	CGO_ENABLED=0 go build -o manager cmd/operator/main.go

# Run against the configured Kubernetes cluster in ~/.kube/config
run: generate fmt vet manifests
	go run cmd/operator/main.go

# Install CRDs into a cluster
install: manifests
	kubectl apply -f deploy/crds/matrixone.matrixorigin.cn_matrixoneclusters.yaml

# Uninstall CRDs from a cluster
uninstall: manifests
	kubectl delete -f deploy/crds/matrixone.matrixorigin.cn_matrixoneclusters.yaml

# Deploy controller in the configured Kubernetes cluster in ~/.kube/config
deploy: manifests
	kubectl apply -f deploy/service_account.yaml
	kubectl apply -f deploy/role.yaml
	kubectl apply -f deploy/role_binding.yaml
	kubectl apply -f deploy/crds/matrixone.matrixorigin.cn_matrixoneclusters.yaml
	kustomize build deploy/ | kubectl apply -f -

# Destroyo Controller the configured Kubernetes cluster in ~/.kube/config
undeploy: manifests
	kustomize build deploy/ | kubectl delete -f -
	kubectl delete -f deploy/crds/matrixone.matrixorigin.cn_matrixoneclusters.yaml
	kubectl delete -f deploy/service_account.yaml
	kubectl delete -f deploy/role.yaml
	kubectl delete -f deploy/role_binding.yaml

# Generate manifests e.g. CRD, RBAC etc.
manifests: $(CONTROLLER_GEN_BINARY)
	$(CONTROLLER_GEN_BINARY) crd webhook paths="./..." output:crd:artifacts:config=deploy/crds/
	$(CONTROLLER_GEN_BINARY) crd webhook paths="./..." output:crd:artifacts:config=charts/matrixone-operator/templates/crds/

# Run go fmt against code
fmt:
	go fmt ./...

# Run go vet against code
vet:
	go vet ./...

# Generate code
generate: $(CONTROLLER_GEN_BINARY)
	$(CONTROLLER_GEN_BINARY) object:headerFile="hack/boilerplate.go.txt" paths="./..."

# helm lint
lint:
	helm lint charts/matrixone-operator

# golangci-lint
go-lint: $(GOLANGCILINTER_BINARY)
	$(GOLANGCILINTER_BINARY) run

# Build the docker image
op-build: generate manifests
	docker build -f images/operator/Dockerfile . -t ${IMG} --build-arg PROXY=$(PROXY)

# Push the docker image
op-push:
	docker push ${IMG}


# start a kind clsuter
kind:
	kind create cluster --config third_part/kind-config/config.yaml
	kubectl apply -f test/kind-rbac.yml

# kind load images
load:
	kind load docker-image matrixorigin/matrixone-operator:latest
	kind load docker-image matrixorigin/mysql-tester:latest

# helm package
helm-pkg:
	helm package charts/matrixone-operator
	mv matrixone-operator-0.1.0.tgz packages

# install tools
$(TOOLS_BIN_DIR):
	mkdir -p $(TOOLS_BIN_DIR)

$(TOOLING): $(TOOLS_BIN_DIR)
	@echo Installing tools from tools/tools.go
	@cat tools/tools.go | grep _ | awk -F'"' '{print $$2}' | GOBIN=$(TOOLS_BIN_DIR) xargs -tI % go install -mod=readonly -modfile=tools/go.mod %
