#!/usr/bin/env bash

set -euo pipefail

function e2e::kubectl_wait_appear() {
    local WAIT_N=0
    local MAX_WAIT=5
    while true; do
        kubectl get $@ 2>/dev/null | grep NAME && break
        if [[ ${WAIT_N} -lt ${MAX_WAIT} ]]; then
            WAIT_N=$((WAIT_N+1))
            echo "Waiting for $@ to be created, sleeping for ${WAIT_N} seconds"
            sleep ${WAIT_N}
        else
            echo "Timeout waiting for $@"
            exit 1
        fi
    done
}

function e2e::diagnosis() {
    exit_code=$?
    if [[ ${exit_code} -ne 0 ]]; then
        echo "> E2E test failed, cluster resources:"
        kubectl get all -A
    fi
}

MYSQL_TEST_IMAGE=${MYSQL_TEST_IMAGE:-matrixorigin/mysql-tester:latest}
NAMESPACE=${NAMESPACE:-default}

trap e2e::diagnosis EXIT 

echo "> Wait for kind cluster bootstrapping"
kubectl cluster-info
kubectl wait --for=condition=Ready pods --all --all-namespaces --timeout=30s

echo "> Check operator deployment"
e2e::kubectl_wait_appear -n ${NAMESPACE} pod -l name=matrixone-operator
kubectl wait --for=condition=Ready -n ${NAMESPACE} pod -l name=matrixone-operator --timeout=30s

echo "> Deploy cluster"
kubectl apply -f examples/tiny-cluster.yaml

echo "> Wait for cluster bootstrapping"
e2e::kubectl_wait_appear -n ${NAMESPACE} svc mo
e2e::kubectl_wait_appear -n ${NAMESPACE} svc mo-headless
e2e::kubectl_wait_appear -n ${NAMESPACE} pod -l matrixone_cr=mo
kubectl wait --for=condition=Ready -n ${NAMESPACE} pod -l matrixone_cr=mo --timeout=30s

echo "> Run bvt test"
kubectl run bvt-tester --image=${MYSQL_TEST_IMAGE} -i --rm --restart=Never --image-pull-policy=IfNotPresent -- \
    -host mo.default.svc.cluster.local -port 6001 -user dump -passwd 111

echo "> Tearing down..."
kubectl delete -f examples/tiny-cluster.yaml
# TODO(aylei): check tearing down