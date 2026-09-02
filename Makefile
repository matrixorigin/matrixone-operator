SHELL=/usr/bin/env bash -o pipefail

# Image URL to use all building/pushing image targets
REPO ?= "matrixorigin/matrixone-operator"
TAG ?= "latest"
GOPROXY ?= "https://proxy.golang.org,direct"
MO_VERSION ?= "1.2.3"
MO_IMAGE_REPO ?= "matrixorigin/matrixone"
BRANCH ?= main
ENVTEST_K8S_VERSION = 1.24.1

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

all: build

.PHONY: build
# Build operator image
build: generate manifests pkg
	docker build -f Dockerfile . -t ${REPO}:${TAG} --build-arg GOPROXY=$(GOPROXY)

PLATFORMS ?= linux/arm64,linux/amd64,linux/s390x,linux/ppc64le
.PHONY: docker-buildx
docker-buildx: ## Build and push docker image for the manager for cross-platform support
	docker buildx build --load --platform=$(PLATFORMS) --tag ${REPO}:${TAG} -f Dockerfile .

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

.PHONY: docs
docs:
	cd api && make docs

.PHONY: mockgen
generate-mockgen: mockgen ## General gomock(https://github.com/golang/mock) files
	$(MOCKGEN) -source=./runtime/pkg/reconciler/event.go -package fake > ./runtime/pkg/fake/event.go

# helm package
helm-pkg: manifests generate verify-chart
	./hack/package-chart.sh charts/

.PHONY: verify-chart
verify-chart:
	./hack/verify-chart.sh


# Generated artifacts that must be committed whenever their sources or generators change.
GENERATED_ARTIFACTS := \
	api/core/v1alpha1/zz_generated.deepcopy.go \
	charts/matrixone-operator/templates/crds \
	deploy/crds \
	deploy/webhook \
	docs/reference/api-reference.md

.PHONY: verify-generated
verify-generated:
	$(MAKE) generate
	$(MAKE) manifests
	$(MAKE) docs
	@status="$$(git status --porcelain -- $(GENERATED_ARTIFACTS))"; \
	if [[ -n "$$status" ]]; then \
		echo "generated artifacts are out of date:"; \
		printf '%s\n' "$$status"; \
		echo "run 'make generate manifests docs' and commit the results"; \
		exit 1; \
	fi

# Make sure the generated files are up to date before open PR. Keep the steps in
# the recipe so `make -j reviewable` cannot run generators and their checks at
# the same time.
reviewable: ci-reviewable
	$(MAKE) verify-generated
	$(MAKE) verify-chart
	$(MAKE) go-lint
	$(MAKE) check-license

ci-reviewable: generate manifests docs test
	go mod tidy

# Check whether the pull request is reviewable in CI. go-lint is deliberately
# excluded since the workflow runs golangci-lint as a separate action.
verify: ci-reviewable
	$(MAKE) verify-generated
	$(MAKE) verify-chart
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

LOCALBIN ?= $(shell pwd)/api/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

ENVTEST ?= $(LOCALBIN)/setup-envtest
SETUP_ENVTEST_MODULE = sigs.k8s.io/controller-runtime/tools/setup-envtest
SETUP_ENVTEST_VERSION = v0.0.0-20250517180713-32e5e9e948a5

.PHONY: envtest
envtest: $(LOCALBIN) ## Install the pinned setup-envtest version if necessary.
	@actual_version="$$(go version -m "$(ENVTEST)" 2>/dev/null | awk -v module="$(SETUP_ENVTEST_MODULE)" '$$1 == "mod" && $$2 == module { print $$3 }')"; \
	if [ "$$actual_version" != "$(SETUP_ENVTEST_VERSION)" ]; then \
		echo "Installing $(SETUP_ENVTEST_MODULE)@$(SETUP_ENVTEST_VERSION) (found: $${actual_version:-none})"; \
		GOBIN=$(LOCALBIN) go install $(SETUP_ENVTEST_MODULE)@$(SETUP_ENVTEST_VERSION); \
	fi

# TODO: include E2E
test: api-test unit

# Run unit tests
unit: generate fmt vet manifests envtest
	@assets="$$( $(ENVTEST) use $(ENVTEST_K8S_VERSION) -p path)" && \
		KUBEBUILDER_ASSETS="$$assets" CGO_ENABLED=0 go test ./pkg/... -coverprofile cover.out

api-test:
	cd api && make test

# Run kind e2e tests
e2e-kind: ginkgo
	REPO=${REPO} TAG=${TAG}	MO_IMAGE_REPO=$(MO_IMAGE_REPO) MO_VERSION=$(MO_VERSION) GINKGO=$(GINKGO) ./hack/kind-e2e.sh

# Launch a kind cluster and install mo operator
up:
	REPO=${REPO} TAG=${TAG}	MO_IMAGE_REPO=$(MO_IMAGE_REPO) MO_VERSION=$(MO_VERSION) ./hack/kind.sh up

# Run e2e tests
# KUBECONFIG is your kubernetes config path, OP_IMAGE_TAG is operator image tag
# export KUBECONFIG=<KUBECONFIG PATH> OP_IMAGE_TAG="sha-c1a16ce"
e2e: ginkgo
	REPO=${REPO} TAG=${TAG} MO_IMAGE_REPO=$(MO_IMAGE_REPO) MO_VERSION=$(MO_VERSION) GINKGO=$(GINKGO)  ./hack/e2e.sh

# Run against the configured Kubernetes cluster in ~/.kube/config
run: generate fmt vet manifests install
	CGO_ENABLED=0 go run cmd/operator/main.go

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

# Build playground image
build-pgd:
	 docker build -f playground/Dockerfile . -t matrixorigin/operator-playground:latest

# Run playground container
run-pgd: build-pgd
	docker run --privileged --name playground -p 6001:6001 --rm -it matrixorigin/operator-playground:latest

GINKGO = $(shell pwd)/bin/ginkgo
GINKGO_MODULE = github.com/onsi/ginkgo/v2
GINKGO_VERSION = v2.9.5
ginkgo: $(LOCALBIN)
	@actual_version="$$(go version -m "$(GINKGO)" 2>/dev/null | awk -v module="$(GINKGO_MODULE)" '$$1 == "mod" && $$2 == module { print $$3 }')"; \
	if [ "$$actual_version" != "$(GINKGO_VERSION)" ]; then \
		echo "Installing $(GINKGO_MODULE)/ginkgo@$(GINKGO_VERSION) (found: $${actual_version:-none})"; \
		GOBIN=$(PROJECT_DIR)/bin go install $(GINKGO_MODULE)/ginkgo@$(GINKGO_VERSION); \
	fi

MOCKGEN = $(shell pwd)/bin/mockgen
mockgen: ## Download mockgen locally if necessary
	$(call go-get-tool,$(MOCKGEN),github.com/golang/mock/mockgen@v1.6.0)

LICENSE_EYE = $(shell pwd)/bin/license-eye
license-eye: ## Download license-eye locally if necessary
	$(call go-get-tool,$(LICENSE_EYE),github.com/apache/skywalking-eyes/cmd/license-eye@v0.4.0)

GOLANGCI_LINT = $(shell pwd)/bin/golangci-lint
GOLANGCI_LINT_MODULE = github.com/golangci/golangci-lint/v2
GOLANGCI_LINT_VERSION = v2.1.6
golangci-lint: $(LOCALBIN)
	@actual_version="$$(go version -m "$(GOLANGCI_LINT)" 2>/dev/null | awk -v module="$(GOLANGCI_LINT_MODULE)" '$$1 == "mod" && $$2 == module { print $$3 }')"; \
	if [ "$$actual_version" != "$(GOLANGCI_LINT_VERSION)" ]; then \
		echo "Installing $(GOLANGCI_LINT_MODULE)/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION) (found: $${actual_version:-none})"; \
		GOBIN=$(PROJECT_DIR)/bin go install $(GOLANGCI_LINT_MODULE)/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION); \
	fi

# go-get-tool will 'go get' any package $2 and install it to $1.
PROJECT_DIR := $(shell dirname $(abspath $(lastword $(MAKEFILE_LIST))))
define go-get-tool
@[ -f $(1) ] || { \
set -e ;\
echo "Downloading $(2)" ;\
GOBIN=$(PROJECT_DIR)/bin go install $(2); \
}
endef
