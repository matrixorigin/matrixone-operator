#!/usr/bin/env bash

set -o errexit
set -o pipefail
set -o nounset
# set -o xtrace

# set default variables
__dir="$(cd "$(dirname "BASH_SOURCE[0]")" && pwd)"
__file="__dir/$(basename "BASH_SOURCE[0]")"
__base="$(basename __file .sh)"

cluster_config="https://raw.githubusercontent.com/matrixorigin/matrixone-operator/main/examples/mo-cluster.yaml"
helm_repo=""
helm_charts=""
helm_version=""
secret_path=""
mo_namespace=""
mo_operator_ns=""


function helm() {
    helm repo add  $helm_repo
    helm repo udpate
    helm install mo $helm_charts --version $helm_version -n $mo_operator_ns
}


function mo_install() {
    kubectl create ns $namespace
    kubectl apply -f $secret_path
    kubectl apply -f $cluster_config
}

function mo_uninstall() {
    kubectl delete -f $cluster_config
    kubectl delete -f $secret_path
    kubectl delete ns $namespace
}

function clean() {
    mo_uninstall()
    helm  unisntall mo -n $mo_operator_ns
    helm remove repo $helm_repo
}
