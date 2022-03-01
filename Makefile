
SHELL=/usr/bin/env bash -o pipefail

# Image URL to use all building/pushing image targets
IMG ?= "matrixone-operator:latest"
# Produce CRDs that work back to Kubernetes 1.11 (no version conversion)
CRD_OPTIONS ?= "crd:maxDescLen=0,trivialVersions=true,generateEmbeddedObjectMeta=true"
MIMG ?= "matrixone:latest"
GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)
TAG?=$(shell git rev-parse --short HEAD)
VERSION?=$(shell cat VERSION | tr -d " \t\n\r")
TOOLS_BIN_DIR ?= $(shell pwd)/tmp/bin
GOLANGCILINTER_BINARY=$(TOOLS_BIN_DIR)/golangci-lint

ifeq ($(GOPATH), arm)
	ARCH = armv7
else
	ARCH = $(GOARCH)
endif

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif


BUILD_DATE=$(shell date +"%Y%m%d-%T")
# source: https://docs.github.com/en/free-pro-team@latest/actions/reference/environment-variables#default-environment-variables
ifndef GITHUB_ACTIONS
	BUILD_USER?=$(USER)
	BUILD_BRANCH?=$(shell git branch --show-current)
	BUILD_REVISION?=$(shell git rev-parse --short HEAD)
else
	BUILD_USER=Action-Run-ID-$(GITHUB_RUN_ID)
	BUILD_BRANCH=$(GITHUB_REF:refs/heads/%=%)
	BUILD_REVISION=$(GITHUB_SHA)
endif

MATRIXONE_OPERATOR_PKG=github.com/matrixorigin/matrix-operator

# The ldflags for the go build process to set the version related data.
GO_BUILD_LDFLAGS=\
	-s \
	-X $(MATRIXONE_OPERATOR_PKG)/version.Revision=$(BUILD_REVISION)  \
	-X $(MATRIXONE_OPERATOR_PKG)/version.BuildUser=$(BUILD_USER) \
	-X $(MATRIXONE_OPERATOR_PKG)/version.BuildDate=$(BUILD_DATE) \
	-X $(MATRIXONE_OPERATOR_PKG)/version.Branch=$(BUILD_BRANCH) \
	-X $(MATRIXONE_OPERATOR_PKG)/version.Version=$(VERSION)

GO_BUILD_RECIPE=\
	GOOS=$(GOOS) \
	GOARCH=$(GOARCH) \
	CGO_ENABLED=0 \
	go build -ldflags="$(GO_BUILD_LDFLAGS)"

.PHONY: all 
all: operator

.PHONY: clean
clean:
	git clean  -Xf ,

# Build matrixone docker image
.PHONY: mo-build
mo-build:
	cd third_part/mo-docker && docker build . -t $(MIMG)
	
# push matrixone docker image
.PHONY: mo-push
mo-push:
	docker push $(MIMG)

# Run tests
.PHONY: test
test: generate fmt vet manifests
	go test ./... -coverprofile cover.out

# Build manager binary
..PHONY: operator
operator:
	$(GO_BUILD_RECIPE) -o $@ cmd/operator/main.go

# Run against the configured Kubernetes cluster in ~/.kube/config
.PHONY: run
run: generate fmt vet manifests
	go run cmd/operator/main.go

# Install CRDs into a cluster
.PHONY: install
install: manifests
	kubectl apply -f deploy/crds/matrixone.matrixorigin.cn_matrixoneclusters.yaml

# Uninstall CRDs from a cluster
.PHONY: uninstall
uninstall: manifests
	kubectl delete -f deploy/crds/matrixone.matrixorigin.cn_matrixoneclusters.yaml

# Deploy controller in the configured Kubernetes cluster in ~/.kube/config
.PHONY: deploy
deploy: manifests
	kubectl apply -f deploy/service_account.yaml
	kubectl apply -f deploy/role.yaml
	kubectl apply -f deploy/role_binding.yaml
	kubectl apply -f deploy/crds/matrixone.matrixorigin.cn_matrixoneclusters.yaml
	kustomize build deploy/ | kubectl apply -f -

# Destroy controller in the configured Kubernetes cluster in ~/.kube/config
.PHONY: undeploy
undeploy: manifests
	kustomize build deploy/ | kubectl delete -f -
	kubectl delete -f deploy/crds/matrixone.matrixorigin.cn_matrixoneclusters.yaml
	kubectl delete -f deploy/service_account.yaml
	kubectl delete -f deploy/role.yaml
	kubectl delete -f deploy/role_binding.yaml

# Generate manifests e.g. CRD, RBAC etc.
.PHONY: manifests
manifests: controller-gen
	$(CONTROLLER_GEN) $(CRD_OPTIONS) paths="./..." output:crd:artifacts:config=deploy/crds/
	$(CONTROLLER_GEN) $(CRD_OPTIONS) paths="./..." output:crd:artifacts:config=charts/matrixone-operator/templates/crds/

.PHONY: format
format:  go-fmt

# Run go fmt against code
.PHONY: go-fmt
go-fmt:
	gofmt -s -w .

.PHONY: jsonnet-fmt
jsonnet-fmt: $(JSONNETFMT_BINARY)
	# *.*sonnet will match *.jsonnet and *.libsonnet files but nothing else in this repository
	find . -name *.jsonnet -not -path "*/vendor/*" -print0 | xargs -0 $(JSONNETFMT_BINARY) -i

.PHONY: shellcheck
shellcheck: $(SHELLCHECK_BINARY)
	$(SHELLCHECK_BINARY) $(shell find . -type f -name "*.sh" -not -path "*/vendor/*")


.PHONY: check-golang
check-golang: $(GOLANGCILINTER_BINARY)
	$(GOLANGCILINTER_BINARY) run

# Run go vet against code
.PHONY: vet
vet:
	go vet ./...

# Generate code
.PHONY: generate
generate: controller-gen
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

# helm lint
.PHONY: lint
lint: 
	helm lint charts/matrixone-operator

# Build the docker image
.PHONY: op-build
op-build: generate manifests
	docker build -f images/operator/Dockerfile . -t ${IMG}

# Push the docker image
.PHONY: op-push
op-push:
	docker push ${IMG}

# find or download controller-gen
# download controller-gen if necessary
.PHONY: controller-gen
controller-gen:
ifeq (, $(shell which controller-gen))
	@{ \
	set -e ;\
	CONTROLLER_GEN_TMP_DIR=$$(mktemp -d) ;\
	cd $$CONTROLLER_GEN_TMP_DIR ;\
	go mod init tmp ;\
	go get sigs.k8s.io/controller-tools/cmd/controller-gen@v0.6.1 ;\
	rm -rf $$CONTROLLER_GEN_TMP_DIR ;\
	}
CONTROLLER_GEN=$(GOBIN)/controller-gen
else
CONTROLLER_GEN=$(shell which controller-gen)
endif