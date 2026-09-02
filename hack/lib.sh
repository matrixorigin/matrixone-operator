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
    local kruise_hook_image
    kruise_image=$(helm template kruise "${ROOT}/charts/kruise" | awk '/^[[:space:]]+image:.*kruise-manager/ {print $2; exit}')
    if [[ -z "${kruise_image}" ]]; then
        echo "error: failed to resolve Kruise manager image from local chart"
        return 1
    fi
    kruise_hook_image=$(helm template kruise "${ROOT}/charts/kruise" | awk '/^[[:space:]]+image:.*kruise-helm-hook/ {print $2; exit}')
    if [[ -z "${kruise_hook_image}" ]]; then
        echo "error: failed to resolve Kruise Helm hook image from local chart"
        return 1
    fi

    kind::prepare_image ${CLUSTER} ${MO_IMAGE_REPO}:${MO_VERSION}
    kind::prepare_image ${CLUSTER} "${kruise_image}"
    kind::prepare_image ${CLUSTER} "${kruise_hook_image}"
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
    local nodes="${E2E_NODES:-4}"
    echo "> Run e2e test"
    make ginkgo
    ./bin/ginkgo -nodes="${nodes}" -stream=true -slowSpecThreshold=3000 ./test/e2e/... -- \
                -mo-version="${MO_VERSION}" \
                -mo-image-repo="${MO_IMAGE_REPO}"

}

function e2e::install() {
  local chart_root
  local operator_chart
  chart_root=$(mktemp -d)

  if ! operator_chart=$(./hack/package-chart.sh "${chart_root}"); then
    rm -rf -- "${chart_root}"
    return 1
  fi

  echo "> Create operator namespace"
  kubectl create ns "${OPNAMESPACE}"
  echo "> Install mo operator"
  if ! helm install mo "${operator_chart}" --set image.repository="${REPO}" --set image.tag="${TAG}" -n "${OPNAMESPACE}"; then
    rm -rf -- "${chart_root}"
    return 1
  fi
  if ! e2e::wait-webhook-ready; then
    rm -rf -- "${chart_root}"
    return 1
  fi
  rm -rf -- "${chart_root}"
}

function e2e::test-kruise-webhook-outage() {
  local chart_root
  local operator_chart
  local status=0
  chart_root=$(mktemp -d)

  if ! operator_chart=$(./hack/package-chart.sh "${chart_root}"); then
    rm -rf -- "${chart_root}"
    return 1
  fi
  ./hack/test-kruise-webhook-outage.sh "${operator_chart}" mo "${OPNAMESPACE}" || status=$?
  rm -rf -- "${chart_root}"
  return "${status}"
}

function e2e::wait-webhook-ready() {
  local selector="app.kubernetes.io/name=matrixone-operator,app.kubernetes.io/instance=mo"
  local mutating="matrixone-operator-mutating-webhook-mo"
  local validating="matrixone-operator-validating-webhook-mo"
  local timeout_seconds=300
  local deadline

  echo "> Wait for operator deployment"
  if ! kubectl -n "${OPNAMESPACE}" wait deployment \
    -l "${selector}" \
    --for=condition=Available \
    --timeout="${timeout_seconds}s"; then
    kubectl -n "${OPNAMESPACE}" get pods -o wide || true
    return 1
  fi

  echo "> Wait for webhook CA injection"
  deadline=$((SECONDS + timeout_seconds))
  while ((SECONDS < deadline)); do
    local mutating_ca
    local validating_ca
    mutating_ca=$(kubectl get mutatingwebhookconfiguration "${mutating}" \
      -o jsonpath='{.webhooks[0].clientConfig.caBundle}' 2>/dev/null || true)
    validating_ca=$(kubectl get validatingwebhookconfiguration "${validating}" \
      -o jsonpath='{.webhooks[0].clientConfig.caBundle}' 2>/dev/null || true)
    if [[ -n "${mutating_ca}" && "${mutating_ca}" != "Cg==" && \
          -n "${validating_ca}" && "${validating_ca}" != "Cg==" ]]; then
      echo "> Webhook CA injection completed"
      return 0
    fi
    sleep 2
  done

  echo "error: webhook CA injection did not complete within ${timeout_seconds}s"
  kubectl get mutatingwebhookconfiguration "${mutating}" -o yaml || true
  kubectl get validatingwebhookconfiguration "${validating}" -o yaml || true
  kubectl -n "${OPNAMESPACE}" logs deployment/mo-matrixone-operator --tail=100 || true
  return 1
}

function e2e::cleanup() {
    echo "Delete e2e test namespace"
    if ! kubectl delete namespace -l managed-by=e2e-suite \
      --ignore-not-found --wait=true --timeout=600s; then
      kubectl get namespace -l managed-by=e2e-suite -o yaml || true
      return 1
    fi
    # Uninstall helm charts
    echo "Uninstall helm charts..."
    if ! helm uninstall mo -n "${OPNAMESPACE}"; then
      kubectl -n kruise-system logs job/mo-finalizer --all-containers=true || true
      return 1
    fi
    echo "Wait for charts uninstall"
    sleep 10
    echo "Delete operator namespace"
    kubectl delete ns "$OPNAMESPACE"
}

function e2e::workflow() {
  e2e::check
  trap "e2e::cleanup" EXIT
  e2e::install || return 1
  local run_status=0
  local outage_status=0
  e2e::run || run_status=$?
  # Run the disruptive outage/upgrade scenario after the established E2E suite
  # so it cannot change the suite's initial cluster state. Preserve an earlier
  # suite failure while still collecting the outage-test result.
  e2e::test-kruise-webhook-outage || outage_status=$?
  if [[ "${run_status}" -eq 0 && "${outage_status}" -ne 0 ]]; then
    run_status=${outage_status}
  fi
  trap - EXIT
  e2e::cleanup || return 1
  return "${run_status}"
}
