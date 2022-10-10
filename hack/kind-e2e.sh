#!/usr/bin/env bash

set -euo pipefail

CLUSTER=${CLUSTER:-"mo"}
ROOT=$(cd $(dirname ${BASH_SOURCE[0]})/.. && pwd)
cd ${ROOT}
source ${ROOT}/hack/lib.sh

function e2e::prepare_image() {
    if [ ! $(docker image ls ${2} --format="true") ] ;
    then
        docker pull ${2}
    fi
    kind load docker-image --name ${1} ${2}
}

function e2e::kind-cleanup() {
    echo "> Tearing down"
    kind delete cluster --name "${1}"
}

function e2e::ensure-kind() {
    echo "> Create kind cluster"
    export KUBECONFIG=$(mktemp)
    echo "$KUBECONFIG"
    kind create cluster --name "${CLUSTER}"
    kubectl apply -f test/kind-rbac.yml
    make build
    kind load docker-image --name "${CLUSTER}" ${REPO}:${TAG}

    echo "> Ensure k8s cluster is ready"
    kubectl cluster-info
    kubectl wait --for=condition=Ready pods --all --all-namespaces --timeout=30s
}

function e2e::load_image() {
  e2e::prepare_image ${CLUSTER} ${MO_IMAGE_REPO}:${MO_VERSION}
  e2e::prepare_image ${CLUSTER} openkruise/kruise-manager:v1.2.0
}

if [[ -z ${MO_VERSION+undefined-guard} ]]; then
  echo "MO_VERSION must be set" && exit 1
fi

hack::ensure_kubectl
hack::ensure_helm
hack::ensure_kind

CLUSTER=${CLUSTER:-mo}


function e2e::kind-e2e() {
  echo "> Start kind e2e test"

  trap "e2e::kind-cleanup ${CLUSTER}" EXIT
  e2e::ensure-kind
  e2e::load_image
  echo "> Run e2e test"
  bash ./hack/e2e.sh
}


e2e::kind-e2e
exec "$@"

