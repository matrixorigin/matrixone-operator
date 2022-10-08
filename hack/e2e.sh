#!/usr/bin/env bash

#set -euo pipefail

OPNAMESPACE=${OPNAMESPACE:-"mo-system"}
MO_VERSION=${MO_VERSION:-"nightly-20eeb7c9"}
IMAGE_REPO=${IMAGE_REPO:-"matrixorigin/matrixone"}

function e2e::check() {
  CMD=pgrep
  PPROC=ginkgo
  crds=$(kubectl get crds --no-headers=true | awk '/matrixorigin/{print $1}')
  echo "> E2E check"
  if [ -n "`$CMD $PPROC`" ]; then
    echo "Already running e2e test, Wait for E2E Ready or Kill it"
    exit 1
  elif [[ $crds != "" ]]; then
    echo "Please delete old CRDS"
    exit 1
  else
    echo "Can run e2e test"
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
  helm install mo ./charts/matrixone-operator --dependency-update --set image.repository=${REPO} --set image.tag=${TAG} -n "${OPNAMESPACE}"

  echo "> Wait webhook certificate injected"
  sleep 30
}

function e2e::delete_resource() {

  echo "> Delete resources"
  ns=$(kubectl get pvc --all-namespaces --no-headers=true | awk '/^e2e/{print $1}')

  ls=$(kubectl get logset -n "$value" | awk '/logset/{print $1}')
  echo "CRD LogSet: $ls"
  if [[ $ls != "" ]]; then
    kubectl delete logset log -n "$ns"
  fi

  mc=$(kubectl get matrixoneclusters -n "$value" | awk '/matrixoneclusters/{print $1}')
  echo "CRD matrixoneclusters: $mc"
  if [[ $mc != "" ]]; then
    kubectl delete matrixoneclusters test -n "$ns"
  fi


}

function e2e::cleanup() {
    echo "> Clean"
    e2e::delete_resource
# TODO: e2e delete hanging pod
#    echo "Delete e2e test namespace"
#    kubectl get ns --all-namespaces --no-headers=true | awk '/^e2e/{print $1}' | xargs kubectl delete ns
    # Uninstall helm charts
    echo "Uninstall helm charts..."
    helm uninstall mo -n "${OPNAMESPACE}"
    echo "Wait for charts uninstall"
    sleep 10
    echo "Delete operator namespace"
    kubectl delete ns "$OPNAMESPACE"
}

function e2e::workflow() {
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
