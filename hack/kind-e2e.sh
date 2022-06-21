#!/usr/bin/env bash

set -euo pipefail

ROOT=$(cd $(dirname ${BASH_SOURCE[0]})/.. && pwd)
cd ${ROOT}

function e2e::prepare_image() {
    docker pull ${2}
    kind load docker-image --name ${1} ${2}
}

function e2e::cleanup() {
    echo "> Tearing down"
    kind delete cluster --name ${1}
}

CLUSTER=${CLUSTER:-mo}
MO_VERSION=${MO_VERSION:-0.4.0}

trap "e2e::cleanup ${CLUSTER}" EXIT

echo "> Create kind cluster"
kind create cluster --name ${CLUSTER} --config test/kind-config.yml
kubectl apply -f test/kind-rbac.yml
make op-build
make load

echo "> Prepare e2e images"
e2e::prepare_image ${CLUSTER} matrixorigin/matrixone:${MO_VERSION}
e2e::prepare_image ${CLUSTER} matrixorigin/mysql-tester:${MO_VERSION}

echo "> Deploy operator"
make deploy

echo "> Run e2e test"
MYSQL_TEST_IMAGE=matrixorigin/mysql-tester:${MO_VERSION} ./hack/e2e.sh
