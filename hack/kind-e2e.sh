#!/usr/bin/env bash

set -euo pipefail

CLUSTER=${CLUSTER:-"mo"}

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
    kind load docker-image --name "${CLUSTER}" matrixorigin/matrixone-operator:latest

    echo "> Ensure k8s cluster is ready"
    kubectl cluster-info
    kubectl wait --for=condition=Ready pods --all --all-namespaces --timeout=30s
}

if [[ -z ${MO_VERSION+undefined-guard} ]]; then
  echo "MO_VERSION must be set" && exit 1
fi

CLUSTER=${CLUSTER:-mo}
MO_IMAGE_REPO=${MO_IMAGE_REPO:-"matrixorigin/matrixone"}
echo "> Run operator E2E with MO ${MO_IMAGE_REPO}:${MO_VERSION}"


function e2e::kind-e2e() {
  echo "> Start kind e2e test"

  trap "e2e::kind-cleanup ${CLUSTER}" EXIT
  e2e::ensure-kind
  echo "> Run e2e test"
  bash ./hack/e2e.sh
}

e2e::kind-e2e
exec "$@"

