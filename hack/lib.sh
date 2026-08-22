#!/usr/bin/env bash

if [ -z "$ROOT" ]; then
    echo "error: ROOT should be initialized"
    exit 1
fi

OS=$(go env GOOS)
ARCH=$(go env GOARCH)
BIN=${ROOT}/bin
KUBECTL_VERSION=${KUBECTL_VERSION:-1.24.2}
KUBECTL_BIN=$BIN/kubectl
HELM_BIN=$BIN/helm
HELM_VERSION=${HELM_VERSION:-3.5.0}
KIND_BIN=$BIN/kind
KIND_VERSION=${KIND_VERSION:-0.14.0}
OPNAMESPACE=${OPNAMESPACE:-"mo-system"}
export PATH=$PATH:${BIN}

test -d "$BIN" || mkdir -p "$BIN"

function hack::ensure_kubectl() {
    if command -v kubectl &> /dev/null; then
        return 0
    fi
    echo "Installing kubectl v$KUBECTL_VERSION..."
    tmpfile=$(mktemp)
    trap "test -f $tmpfile && rm $tmpfile" RETURN
    curl --retry 10 -L -o $tmpfile https://storage.googleapis.com/kubernetes-release/release/v${KUBECTL_VERSION}/bin/${OS}/${ARCH}/kubectl
    mv $tmpfile $KUBECTL_BIN
    chmod +x $KUBECTL_BIN
}

function hack::ensure_helm() {
    if command -v helm &> /dev/null; then
        return 0
    fi
    echo "Installing helm ${HELM_VERSION}..."
    local HELM_URL=https://get.helm.sh/helm-v${HELM_VERSION}-${OS}-${ARCH}.tar.gz
    curl --retry 3 -L -s "$HELM_URL" | tar --strip-components 1 -C $BIN -zxf - ${OS}-${ARCH}/helm
}

function hack::ensure_kind() {
    if command -v kind &> /dev/null; then
        return 0
    fi
    echo "Installing kind v$KIND_VERSION..."
    tmpfile=$(mktemp)
    trap "test -f $tmpfile && rm $tmpfile" RETURN
    curl --retry 10 -L -o $tmpfile https://github.com/kubernetes-sigs/kind/releases/download/v${KIND_VERSION}/kind-$(uname)-amd64
    mv $tmpfile $KIND_BIN
    chmod +x $KIND_BIN
}

function kind::prepare_image() {
    if [ ! $(docker image ls ${2} --format="true") ] ;
    then
        docker pull ${2}
    fi
    kind load docker-image --name ${1} ${2}
}

function kind::cleanup() {
    echo "> Tearing down"
    kind delete cluster --name "${1}"
}

function kind::ensure-kind() {
    echo "> Create kind cluster"
    export KUBECONFIG=$(mktemp)
    echo "########## KUBECONFIG Path ##########"
    echo "$KUBECONFIG"
    echo "#####################################"
    kind create cluster --name "${CLUSTER}"
    kubectl apply -f test/kind-rbac.yml
    make build
    kind load docker-image --name "${CLUSTER}" ${REPO}:${TAG}

    echo "> Ensure k8s cluster is ready"
    kubectl cluster-info
    kubectl wait --for=condition=Ready pods --all --all-namespaces --timeout=300s
}

function kind::load-image() {
    local kruise_image
    kruise_image=$(helm template kruise "${ROOT}/charts/kruise" | awk '/^[[:space:]]+image:.*kruise-manager/ {print $2; exit}')
    if [[ -z "${kruise_image}" ]]; then
        echo "error: failed to resolve Kruise manager image from local chart"
        return 1
    fi

    kind::prepare_image ${CLUSTER} ${MO_IMAGE_REPO}:${MO_VERSION}
    kind::prepare_image ${CLUSTER} "${kruise_image}"
    kind::prepare_image ${CLUSTER} minio/minio:RELEASE.2023-11-01T01-57-10Z
}

function kind::install-minio() {
    kubectl -n default apply -f examples/minio.yaml
}

function e2e::check() {
  CMD=pgrep
  crds=$(kubectl get crds --no-headers=true | awk '/matrixorigin/{print $1}')
  echo "> E2E check"
  if [[ $crds != "" ]]; then
    echo "Please delete old CRDS"
    exit 1
  else
    echo "Can run e2e test"
  fi
}

function e2e::run() {
    echo "> Run e2e test"
    make ginkgo
    ./bin/ginkgo -nodes=4 -stream=true -slowSpecThreshold=3000 ./test/e2e/... -- \
                -mo-version="${MO_VERSION}" \
                -mo-image-repo="${MO_IMAGE_REPO}"

}

function e2e::install() {
  local chart_root
  chart_root=$(mktemp -d)

  mkdir -p "${chart_root}/matrixone-operator/charts"
  cp charts/matrixone-operator/Chart.yaml charts/matrixone-operator/values.yaml "${chart_root}/matrixone-operator/"
  cp -R charts/matrixone-operator/templates "${chart_root}/matrixone-operator/"
  if ! helm package charts/kruise --destination "${chart_root}/matrixone-operator/charts"; then
    rm -rf -- "${chart_root}"
    return 1
  fi

  echo "> Create operator namespace"
  kubectl create ns "${OPNAMESPACE}"
  echo "> Install mo operator"
  if ! helm install mo "${chart_root}/matrixone-operator" --set image.repository="${REPO}" --set image.tag="${TAG}" -n "${OPNAMESPACE}"; then
    rm -rf -- "${chart_root}"
    return 1
  fi
  rm -rf -- "${chart_root}"

  echo "> Wait webhook certificate injected"
  sleep 30
}

function e2e::cleanup() {
    echo "Delete e2e test namespace"
    kubectl get ns --all-namespaces --no-headers=true | awk '/^e2e/{print $1}' | xargs kubectl delete ns
    # Uninstall helm charts
    echo "Uninstall helm charts..."
    helm uninstall mo -n "${OPNAMESPACE}"
    echo "Wait for charts uninstall"
    sleep 10
    echo "Delete operator namespace"
    kubectl delete ns "$OPNAMESPACE"
}

function e2e::workflow() {
  e2e::check
  trap "e2e::cleanup" EXIT
  e2e::install || return 1
  e2e::run
}
