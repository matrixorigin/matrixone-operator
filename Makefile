SHELL=/usr/bin/env bash -o pipefail

# Image URL to use all building/pushing image targets
IMG ?= "matrixorigin/matrixone-operator:latest"
MIMG ?= "matrixorigin/matrixone:kc"
BIMG ?= "matrixorigin/mysql-tester:latest"
PROXY ?= https://goproxy.cn,direct
BRANCH ?= main
TOOLS_BIN_DIR ?= $(shell pwd)/tmp/bin
GOLANGCILINTER_BINARY=$(TOOLS_BIN_DIR)/golangci-lint
LICENSE_EYE_BINARY=$(TOOLS_BIN_DIR)/license-eye
TOOLING=$(GOLANGCILINTER_BINARY) $(LICENSE_EYE_BINARY)


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

# Make sure the generated files are up to date before open PR
reviewable: ci-reviewable go-lint check-license

ci-reviewable: generate manifests test
	go mod tidy

# Check whether the pull request is reviewable in CI, go-lint is delibrately excluded since we already have golangci-lint action
verify: ci-reviewable
	echo "checking that branch is clean"
	test -z "$$(git status --porcelain)" || (echo "unclean working tree, did you forget to run make reviewable?" && exit 1)
	echo "branch is clean"

# license check
check-license: $(LICENSE_EYE_BINARY)
	$(LICENSE_EYE_BINARY) -v info -c .licenserc.yml header check

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
	kubectl apply -f deploy/crds

# Uninstall CRDs from a cluster
uninstall: manifests
	kubectl delete -f deploy/crds

kruise:
	helm repo add openkruise https://openkruise.github.io/charts/
	helm repo update
	helm install kruise openkruise/kruise --version 1.2.0

# Deploy controller in the configured Kubernetes cluster in ~/.kube/config
deploy: install manifests
	kubectl apply -f deploy/service_account.yaml
	kubectl apply -f deploy/role.yaml
	kubectl apply -f deploy/role_binding.yaml
	kustomize build deploy/ | kubectl apply -f -

# Destroyo Controller the configured Kubernetes cluster in ~/.kube/config
undeploy: uninstall manifests
	kustomize build deploy/ | kubectl delete -f -
	kubectl delete -f deploy/service_account.yaml
	kubectl delete -f deploy/role.yaml
	kubectl delete -f deploy/role_binding.yaml

## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
.PHONY: manifests
manifests:
	cd api && make manifests

## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
.PHONY: generate
generate:
	cd api && make generate

# Run go fmt against code
fmt:
	go fmt ./...

# Run go vet against code
vet:
	go vet ./...

# helm lint
lint:
	helm lint charts/matrixone-operator

# golangci-lint
go-lint: $(GOLANGCILINTER_BINARY)
	$(GOLANGCILINTER_BINARY) run

# Build the docker image
op-build: generate manifests pkg
	docker build -f images/operator/Dockerfile . -t ${IMG} --build-arg PROXY=$(PROXY)

# Push the docker image
op-push:
	docker push ${IMG}

# local e2e test on kind
e2e:
	./hack/kind-e2e.sh

# start a kind clsuter
kind:
	kind create cluster --config test/kind-config.yml
	kubectl apply -f test/kind-rbac.yml

# kind load images
load:
	kind load docker-image --name mo matrixorigin/matrixone-operator:latest

# helm package
helm-pkg: charts
	helm package charts/matrixone-operator
	mv matrixone-operator-0.1.0.tgz packages

# install tools
$(TOOLS_BIN_DIR):
	mkdir -p $(TOOLS_BIN_DIR)

$(TOOLING): $(TOOLS_BIN_DIR)
	@echo Installing tools from tools/tools.go
	@cat tools/tools.go | grep _ | awk -F'"' '{print $$2}' | GOBIN=$(TOOLS_BIN_DIR) xargs -tI % go install -mod=readonly -modfile=tools/go.mod %
