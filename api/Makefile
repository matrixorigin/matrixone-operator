# This Makefile is an aid for ../Makefile and does not supposed to be used independently
# Produce CRDs that work back to Kubernetes 1.11 (no version conversion)
CRD_OPTIONS ?= crd:crdVersions=v1
ENVTEST_K8S_VERSION = 1.24.1

.PHONY: manifests
## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
manifests: controller-gen
	$(CONTROLLER_GEN) $(CRD_OPTIONS) rbac:roleName=manager-role webhook paths="./..." output:crd:artifacts:config=../deploy/crds output:webhook:artifacts:config=../deploy/webhook
	$(CONTROLLER_GEN) $(CRD_OPTIONS) rbac:roleName=manager-role paths="./..." output:crd:artifacts:config=../charts/matrixone-operator/templates/crds/

.PHONY: generate
generate: controller-gen
	$(CONTROLLER_GEN) object:headerFile="../hack/boilerplate.go.txt" paths="./..."

.PHONY: test
test: manifests generate envtest ## Run tests.
	@assets="$$( $(ENVTEST) use $(ENVTEST_K8S_VERSION) -p path)" && \
		KUBEBUILDER_ASSETS="$$assets" go test ./... -coverprofile cover.out

.PHONY: docs
docs: crd-ref-docs
	$(CRD_REF_DOCS) --source-path=core/v1alpha1 --renderer=markdown --output-path ../docs/reference/api-reference.md

## Location to install dependencies to
PROJECT_DIR := $(shell dirname $(abspath $(lastword $(MAKEFILE_LIST))))
LOCALBIN ?= $(PROJECT_DIR)/bin
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

CONTROLLER_GEN = $(LOCALBIN)/controller-gen
CONTROLLER_GEN_MODULE = sigs.k8s.io/controller-tools
CONTROLLER_GEN_PACKAGE = sigs.k8s.io/controller-tools/cmd/controller-gen
CONTROLLER_GEN_VERSION = v0.16.0
# After changing CONTROLLER_GEN_VERSION, run `make verify-generated` from the repository root
# and commit every regenerated artifact.
.PHONY: controller-gen
controller-gen: $(LOCALBIN) ## Download the pinned controller-gen version locally if necessary.
	$(call go-install-pinned,$(CONTROLLER_GEN),$(CONTROLLER_GEN_MODULE),$(CONTROLLER_GEN_PACKAGE),$(CONTROLLER_GEN_VERSION))

CRD_REF_DOCS = $(LOCALBIN)/crd-ref-docs
CRD_REF_DOCS_MODULE = github.com/elastic/crd-ref-docs
CRD_REF_DOCS_PACKAGE = github.com/elastic/crd-ref-docs
CRD_REF_DOCS_VERSION = v0.0.12
.PHONY: crd-ref-docs
crd-ref-docs: $(LOCALBIN)
	$(call go-install-pinned,$(CRD_REF_DOCS),$(CRD_REF_DOCS_MODULE),$(CRD_REF_DOCS_PACKAGE),$(CRD_REF_DOCS_VERSION))

# go-install-pinned ensures binary $1 was built from module $2 at version $4.
# The install package $3 can be a command below the module root.
define go-install-pinned
	@set -e; \
	actual_version="$$(go version -m "$(1)" 2>/dev/null | awk -v module="$(2)" '$$1 == "mod" && $$2 == module { print $$3 }')"; \
	if [ "$$actual_version" != "$(4)" ]; then \
		echo "Installing $(3)@$(4) (found: $${actual_version:-none})"; \
		GOBIN=$(LOCALBIN) go install $(3)@$(4); \
	fi
endef
