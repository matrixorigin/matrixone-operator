# This Makefile is an aid for ../Makefile and does not supposed to be used independently
# Produce CRDs that work back to Kubernetes 1.11 (no version conversion)
CRD_OPTIONS ?= crd:crdVersions=v1 

.PHONY: manifests
## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
manifests: controller-gen
	$(CONTROLLER_GEN) $(CRD_OPTIONS) rbac:roleName=matrixone output:rbac:artifacts:config=deploy webhook paths="./..." output:crd:artifacts:config=deploy/crds output:webhook:artifacts:config=deploy/webhook
	$(CONTROLLER_GEN) $(CRD_OPTIONS)  webhook paths="./..." output:crd:artifacts:config=charts/matrixone-operator/templates/crds/ output:webhook:artifacts:config=charts/matrixone-operator/templates/crds/webhook


.PHONY: generate
generate: controller-gen core/v1alpha1/zz_generated.deepcopy.go

core/v1alpha1/zz_generated.deepcopy.go: api/core/v1alpha1/*_types.go
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

CONTROLLER_GEN = $(shell pwd)/api/bin/controller-gen
controller-gen: ## Download controller-gen locally if necessary.
	$(call go-get-tool,$(CONTROLLER_GEN),sigs.k8s.io/controller-tools/cmd/controller-gen@v0.9.2)

# go-get-tool will 'go get' any package $2 and install it to $1.
PROJECT_DIR := $(shell dirname $(abspath $(lastword $(MAKEFILE_LIST))))
define go-get-tool
@[ -f $(1) ] || { \
set -e ;\
echo "Downloading $(2)" ;\
GOBIN=$(PROJECT_DIR)/bin go install $(2); \
}
endef
