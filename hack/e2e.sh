#!/usr/bin/env bash

set -euo pipefail

CLUSTER=${CLUSTER:-"e2e-mo-kind"}
MO_VERSION=${MO_VERSION:-"nightly-20eeb7c9"}
IMAGE_REPO=${IMAGE_REPO:-"matrixorigin/matrixone"}
OPNAMESPACE=${OPNAMESPACE:-"e2e-matrixone-operator"}
TESTNAMESPACE=${TESTNAMESPACE:-"e2e"}

function e2e::check() {
  echo "> E2E check"
  nse2e=$(kubectl get ns --no-headers=true | awk  '/^e2e/{print $1}')

  if [[ $nse2e != "" ]]; then
    echo "Find e2e namespace $nse2e"
    echo "Please delete e2e namespace before idc e2e, Or Waiting e2e finished"
    exit 1
  else
    echo "Env Check Finished, You can start e2e test"
  fi
}

function e2e::run() {
    echo "> Run e2e test"
    make ginkgo
    ./bin/ginkgo -stream -slowSpecThreshold=3000 ./test/e2e/... -- \
                -mo-version="${MO_VERSION}" \
                -mo-image-repo="${IMAGE_REPO}"
}

function e2e::install() {
  echo "> Create operator namespace"
  kubectl create ns "${OPNAMESPACE}"
  echo "> Install mo operator"
  helm install mo ./charts/matrixone-operator --dependency-update -n "${OPNAMESPACE}"

  echo "> Wait webhook certificate injected"
  sleep 30
}

function e2e::cleanup() {
    echo "> Clean"
    # Uninstall helm charts
    echo "Uninstall helm charts..."
    helm uninstall mo -n "${OPNAMESPACE}"
    echo "Delete operator namespace"
    kubectl delete ns "$OPNAMESPACE"
#    # Delete test ns
#    echo "Delete test namespaces..."
#    kubectl get ns --no-headers=true | awk '/^e2e-/{print $1}' | xargs  kubectl delete ns
}

function e2e::workflow() {
  echo "> Start workflow"
  trap "e2e::cleanup" EXIT

  e2e::check
  e2e::install
  e2e::run
}

function e2e::start() {
  echo "> Start e2e workflow"
  e2e::workflow
}

e2e::start
exec "$@"
