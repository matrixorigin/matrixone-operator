#!/usr/bin/env bash

set -euo pipefail

ROOT=$(cd $(dirname ${BASH_SOURCE[0]})/.. && pwd)
cd ${ROOT}

CLUSTER=${CLUSTER:-"e2e-mo-kind"}
MO_VERSION=${MO_VERSION:-"nightly-20eeb7c9"}
IMAGE_REPO=${IMAGE_REPO:-"matrixorigin/matrixone"}
OPNAMESPACE=${OPNAMESPACE:-"idc-e2e-matrixone-operator"}
TESTNAMESPACE=${TESTNAMESPACE:-"e2e"}

function e2e::prepare_image() {
    docker pull ${2}
    kind load docker-image --name ${1} ${2}
}

function e2e::kubectl_wait_appear() {
    local WAIT_N=0
    local MAX_WAIT=5
    while true; do
        kubectl get $@ 2>/dev/null | grep NAME && break
        if [ ${WAIT_N} -lt ${MAX_WAIT} ]; then
            WAIT_N=$((WAIT_N+1))
            echo "Waiting for $@ to be created, sleeping for ${WAIT_N} seconds"
            sleep ${WAIT_N}
        else
            echo "Timeout waiting for $@"
            exit 1
        fi
    done
}

function e2e::kind-cleanup() {
    echo "> Tearing down"
    kind delete cluster --name ${1}
}

function e2e::idc-cleanup() {
    echo "> Clean"
    # Uninstall helm charts
    echo "Uninstall helm charts..."
    helm uninstall mo -n "${OPNAMESPACE}"
    choe "Delete operator namespace"
    kubectl delete ns "$OPNAMESPACE"
    # Delete test ns
    echo "Delete test namespaces..."
    kubectl get ns --no-headers=true | awk -v m="${CLUSTER}" '/^{m}/{print $1}' | xargs  kubectl delete ns

}

function e2e::idc-install() {
  kubectl create ns idc-e2e-matrixone-operator
  helm install mo ./charts/matrixone-operator --dependency-update -n "${OPNAMESPACE}"
  kubectl wait --for=condition=Ready pods --all -n "${OPNAMESPACE}"
}

function e2e::idc-check() {
  echo "> E2E idc check"
  nse2e=$(kubectl get ns --no-headers=true  | awk -v m="$TESTNAMESPACE" '/^{m}/{print $1}')
  nsope2e=$(kubectl get ns --no-headers=true | awk -v m="$OPNAMESPACE" '/^{$m}/{print $1}')


  if [[ $nse2e != "" ]]; then
    echo "Find e2e namespace $nse2e"
    echo "Please delete e2e namespace before idc e2e, Or Waiting e2e finished"
    exit 1
  elif [[ $nsope2e != "" ]]; then
    echo "Find e2e operator namespace: $nsope2e"
    echo "Please delete idc operator namespace before idc e2e, Or waiting idc e2e finished"
    exit 1
  else
    echo "Env Check Finished, You can start e2e test"
  fi
}

function e2e::ensure-kind() {
    echo "> Create kind cluster"
    export KUBECONFIG=$(mktemp)
    echo $KUBECONFIG
    kind create cluster --name ${CLUSTER}
    kubectl apply -f test/kind-rbac.yml
    make build
    kind load docker-image --name "${CLUSTER}" matrixorigin/matrixone-operator:latest

    echo "> Prepare e2e images"
    e2e::prepare_image "${CLUSTER}" matrixorigin/matrixone:${MO_VERSION}
    e2e::prepare_image "${CLUSTER}" openkruise/kruise-manager:v1.2.0

    echo "> Ensure k8s cluster is ready"
    kubectl cluster-info
    kubectl wait --for=condition=Ready pods --all --all-namespaces --timeout=30s
}

function e2e::run() {
    echo "> Run e2e test"
    make ginkgo
    ./bin/ginkgo -stream -slowSpecThreshold=3000 ./test/e2e/... -- \
                -mo-version="${MO_VERSION}" \
                -mo-image-repo="${IMAGE_REPO}"
}

function e2e::install-op() {
  echo "> Create operator namespace"
  kubectl create ns "${OPNAMESPACE}"
  echo "> Install mo operator"
  helm install mo ./charts/matrixone-operator --dependency-update -n "${OPNAMESPACE}"

  echo "> Wait webhook certificate injected"
  sleep 30
}

function e2e::kind-workflow() {
  echo "> Start kind workflow"
  trap "e2e::kind-cleanup ${CLUSTER}" EXIT

  e2e::ensure-kind
  e2e::install-op

  e2e::run
}

function e2e::idc-workflow() {
  echo "> Start idc-workflow"
  trap "e2e::idc-cleanup" EXIT

  e2e::idc-check
  e2e::install-op
  e2e::run
}

function e2e::start() {
  echo "> Start e2e workflow"
  ctype=$(kind get clusters | awk -v m="$CLUSTER" '/^{m}/{print $1}')

  if [[ $ctype == "" ]]; then
    echo "Not find kind test cluster, Start kind workflow"
    e2e::kind-workflow
  else
    e2e::idc-workflow
  fi
}

e2e::start
exec "$@"
