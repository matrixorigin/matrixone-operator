#!/usr/bin/env bash

set -euo pipefail

ROOT=$(cd $(dirname ${BASH_SOURCE[0]})/.. && pwd)
cd ${ROOT}

function e2e::prepare_image() {
    if [ ! $(docker image ls ${2} --format="true") ] ;
    then
        docker pull ${2}
    fi
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

function e2e::cleanup() {
    echo "> Tearing down"
    kind delete cluster --name ${1}
}

if [[ -z ${MO_VERSION+undefined-guard} ]]; then
  echo "MO_VERSION must be set" && exit 1
fi

CLUSTER=${CLUSTER:-mo}
MO_IMAGE_REPO=${MO_IMAGE_REPO:-"matrixorigin/matrixone"}
echo "> Run operator E2E with MO ${MO_IMAGE_REPO}:${MO_VERSION}"

trap "e2e::cleanup ${CLUSTER}" EXIT

echo "> Create kind cluster"
export KUBECONFIG=$(mktemp)
echo $KUBECONFIG
kind create cluster --name ${CLUSTER}
kubectl apply -f test/kind-rbac.yml
make build
kind load docker-image --name ${CLUSTER} matrixorigin/matrixone-operator:latest

echo "> Prepare e2e images"
e2e::prepare_image ${CLUSTER} ${MO_IMAGE_REPO}:${MO_VERSION}
e2e::prepare_image ${CLUSTER} openkruise/kruise-manager:v1.2.0

echo "> Install mo operator"
helm install mo ./charts/matrixone-operator --dependency-update

echo "> Ensure k8s cluster is ready"
kubectl cluster-info
kubectl wait --for=condition=Ready pods --all --all-namespaces --timeout=30s

echo "> Wait webhook certificate injected"
sleep 30

#if [[ ! -z ${AWS_ACCESS_KEY_ID+undefined-guard} ]] ; then
#  echo "> Ensure S3 credentials"
#  kubectl create secret generic aws --from-literal=AWS_ACCESS_KEY_ID=${AWS_ACCESS_KEY_ID} --from-literal=AWS_SECRET_ACCESS_KEY=${AWS_SECRET_ACCESS_KEY}
#fi

echo "> Run e2e test"
$GINKGO -stream -slowSpecThreshold=3000 ./test/e2e/... -- -mo-version=${MO_VERSION} -mo-image-repo=${MO_IMAGE_REPO}
