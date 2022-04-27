# Image URL to use all building/pushing image targets
IMG ?= "matrixorigin/matrixone-operator:latest"
# Produce CRDs that work back to Kubernetes 1.11 (no version conversion)
CRD_OPTIONS ?= "crd:maxDescLen=0,trivialVersions=true,generateEmbeddedObjectMeta=true"
MIMG ?= "matrixorigin/matrixone:latest"
PROXY ?= https://goproxy.cn,direct
BRANCH ?= main


# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

all: operator

# Build matrixone docker image
mo-build:
	cd third_part/mo-docker && docker build . -t $(MIMG) --build-arg PROXY=$(PROXY) --build-arg BRANCH=$(BRANCH)

# push matrixone docker image
mo-push:
	docker push $(MIMG)

# Run tests
test: generate fmt vet manifests
	go test ./... -coverprofile cover.out

# Build manager binary
operator: generate fmt vet
	CGO_ENABLED=0 go build -o operator cmd/operator/main.go

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

# Destroy controller in the configured Kubernetes cluster in ~/.kube/config
undeploy: manifests
	kustomize build deploy/ | kubectl delete -f -
	kubectl delete -f deploy/crds/matrixone.matrixorigin.cn_matrixoneclusters.yaml
	kubectl delete -f deploy/service_account.yaml
	kubectl delete -f deploy/role.yaml
	kubectl delete -f deploy/role_binding.yaml

# Generate manifests e.g. CRD, RBAC etc.
manifests: controller-gen
	$(CONTROLLER_GEN) $(CRD_OPTIONS) paths="./..." output:crd:artifacts:config=deploy/crds/
	$(CONTROLLER_GEN) $(CRD_OPTIONS) paths="./..." output:crd:artifacts:config=charts/matrixone-operator/templates/crds/

# Run go fmt against code
fmt:
	go fmt ./...

# Run go vet against code
vet:
	go vet ./...

# Generate code
generate: controller-gen
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

# helm lint
lint:
	helm lint charts/matrixone-operator

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


# helm package
helm-pkg:
	helm package charts/matrixone-operator
	mv matrixone-operator-0.1.0.tgz packages

# find or download controller-gen
# download controller-gen if necessary
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
