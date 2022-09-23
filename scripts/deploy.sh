# Copyright 2022 Matrix Origin
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

#!/usr/bin/env bash

set -o errexit
set -o pipefail
# set -o nounset
# set -o xtrace

# set default variables
__dir="$(cd "$(dirname "BASH_SOURCE[0]")" && pwd)"
__file="__dir/$(basename "BASH_SOURCE[0]")"
__base="$(basename __file .sh)"

cluster_config="https://raw.githubusercontent.com/matrixorigin/matrixone-operator/main/examples/mo-cluster.yaml"
helm_repo="https://wanglei4687.github.io/charts/"
helm_charts="mo/matrixone-operator"
helm_version="0.1.0"
mo_namespace="matrixone"
mo_operator_ns="matrixone-operator"
op_name="mo"
secrete_name="aws"


function helm_install() {
    helm repo add $op_name $helm_repo
    helm repo udpate
}


function ns_create() {
    kubectl create ns $mo_namespace
    kubectl create ns $mo_operator_ns
}

function op_install() {
    helm install $op_name $helm_charts --version $helm_version -n $mo_operator_ns
}

function secret_install() {
    # aws s3
    access_key=`echo -n "${AWS_ACCESS_KEY_ID}" | base64`
    secret_key=`echo -n "${AWS_SECRET_ACCESS_KEY}" | base64`
    kubectl create secret generic $secrete_name --from-literal=AWS_ACCESS_KEY_ID=$access_key --from-literal=AWS_SECRET_ACCESS_KEY=$secret_key -n $mo_namespace

    # TODO: Support minio, nfs
}

function mo_install() {
    kubectl apply -f $cluster_config -n $mo_namespace
}

function mo_uninstall() {
    kubectl delete -f $cluster_config -n $mo_namespace
    kubectl delete secret $secret_name -n $mo_namespace
    kubectl delete ns $mo_namespace
}


function helm_uninstall() {
    helm uninstall $op_name -n $mo_operator_ns
    helm remove repo $helm_repo
    kubectl delete ns $mo_operator_ns
}

function clean() {
    mo_uninstall
    helm_uninstall
}

function install() {
    ns_create
    secret_install
    helm_install
    op_install
    mo_install
}

while [ True ]; do
    if [ "$1" = "install" -o "$1" = "i" ]; then
        install
        shift 1
    elif [ "$1" = "remove" -o "$1" = "rm" ]; then
        mo_uninstall
        shift 1
    elif [ "$1" = "clean" -o "$1" = "c" ]; then
        clean
        shift 1
    elif [ "$1" = "ns" ]; then
        ns_create
        shift 1
    elif [ "$1" = "secret" ]; then
        secrete_install
        shift 1
    elif [ "$1" = "param" ]; then
        echo "helm_repo ===>" $helm_repo
        echo "helm_charts ===>" $helm_charts
        echo "op_name ===>" $op_name
        echo "cluster_config ===>" $cluster_config
        echo "secrete_name ===>" $secrete_name
        echo "mo_operator_ns ===>" $mo_operator_ns
        echo "mo_namespace ===>" $mo_namespace
        shift 1
    else
        break
    fi
done
