SHELL=/usr/bin/env bash -o pipefail

# Image URL to use all building/pushing image targets
IMG ?= "matrixorigin/matrixone-operator:latest"
PROXY ?= "https://proxy.golang.org,direct"
BRANCH ?= main

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

all: manager

.PHONY: build
# Build operator image
build: generate manifests pkg
	docker build -f Dockerfile . -t ${IMG} --build-arg PROXY=$(PROXY)

# Push operator image
push:
	docker push ${IMG}

# Build manager binary
manager: generate fmt vet
	CGO_ENABLED=0 go build -o manager cmd/operator/main.go

## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
.PHONY: manifests
manifests:
	cd api && make manifests

## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
.PHONY: generate
generate:
	cd api && make generate

.PHONY: mockgen
generate-mockgen: mockgen ## General gomock(https://github.com/golang/mock) files
	$(MOCKGEN) -source=./runtime/pkg/reconciler/event.go -package fake > ./runtime/pkg/fake/event.go

# helm package
helm-pkg: manifests generate helm-lint
	helm dependency build charts/matrixone-operator
	helm package -u charts/matrixone-operator -d charts/


# Make sure the generated files are up to date before open PR
reviewable: ci-reviewable go-lint check-license test

ci-reviewable: generate manifests test
	go mod tidy

# Check whether the pull request is reviewable in CI, go-lint is delibrately excluded since we already have golangci-lint action
verify: ci-reviewable
	echo "checking that branch is clean"
	test -z "$$(git status --porcelain)" || (echo "unclean working tree, did you forget to run make reviewable?" && exit 1)
	echo "branch is clean"

# Run go fmt against code
fmt:
	go fmt ./...

# Run go vet against code
vet:
	go vet ./...

# helm lint
helm-lint:
	helm lint charts/matrixone-operator

# golangci-lint
go-lint: golangci-lint
	$(GOLANGCI_LINT) run

# license check
check-license: license-eye
	$(LICENSE_EYE) -v info -c .licenserc.yml header check

# TODO: include E2E
test: api-test unit

# Run unit tests
unit: generate fmt vet manifests
	go test ./pkg/... -coverprofile cover.out

api-test:
	cd api && make test

# Run e2e tests
e2e: ginkgo
	GINKGO=$(GINKGO) ./hack/e2e.sh

# Run against the configured Kubernetes cluster in ~/.kube/config
run: generate fmt vet manifests install
	go run cmd/operator/main.go

# Install CRDs into a cluster
install: manifests
	kubectl apply -f deploy/crds

# Uninstall CRDs from a cluster
uninstall: manifests
	kubectl delete -f deploy/crds

# Deploy controller in the configured Kubernetes cluster in ~/.kube/config
deploy: install manifests
	kubectl apply -f deploy/service_account.yaml

# Destroy Controller the configured Kubernetes cluster in ~/.kube/config
undeploy: uninstall manifests
	kustomize build deploy/ | kubectl delete -f -

GINKGO = $(shell pwd)/bin/ginkgo
ginkgo:
	$(call go-get-tool,$(GINKGO),github.com/onsi/ginkgo/ginkgo@v1.6.0)

MOCKGEN = $(shell pwd)/bin/mockgen
mockgen: ## Download mockgen locally if necessary
	$(call go-get-tool,$(MOCKGEN),github.com/golang/mock/mockgen@v1.6.0)

LICENSE_EYE = $(shell pwd)/bin/license-eye
license-eye: ## Download license-eye locally if necessary
	$(call go-get-tool,$(LICENSE_EYE),github.com/apache/skywalking-eyes/cmd/license-eye@v0.4.0)

GOLANGCI_LINT = $(shell pwd)/bin/golangci-lint
golangci-lint:
	$(call go-get-tool,$(GOLANGCI_LINT),github.com/golangci/golangci-lint/cmd/golangci-lint@v1.46.1)

# go-get-tool will 'go get' any package $2 and install it to $1.
PROJECT_DIR := $(shell dirname $(abspath $(lastword $(MAKEFILE_LIST))))
define go-get-tool
@[ -f $(1) ] || { \
set -e ;\
echo "Downloading $(2)" ;\
GOBIN=$(PROJECT_DIR)/bin go install $(2); \
}
endef
