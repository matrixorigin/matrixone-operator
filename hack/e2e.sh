#!/usr/bin/env bash

#set -euo pipefail

OPNAMESPACE=${OPNAMESPACE:-"mo-system"}

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
  echo "> Create operator namespace"
  kubectl create ns "${OPNAMESPACE}"
  echo "> Install mo operator"
  helm install mo ./charts/matrixone-operator --dependency-update --set image.repository=${REPO} --set image.tag=${TAG} -n "${OPNAMESPACE}"

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
  e2e::install
  e2e::run
}

function e2e::start() {
  echo "> Start e2e workflow"
  e2e::workflow
}

e2e::start
