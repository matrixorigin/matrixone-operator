#!/usr/bin/env bash

# Prerequisites: go version >= 1.18, git, awk, kubectl, k8s cluster config file, k8s default storage class
# Use it:
# export KUBECONFIG=<YOUR KUBECONFIG PATH> && bash <(curl -s -L https://raw.githubusercontent.com/matrixorigin/matrixone-operator/main/hack/idc-e2e.sh)

# set default variables
__dir="$(cd "$(dirname "BASH_SOURCE[0]")" && pwd)"

MO_VERSION=${MO_VERSION:-"nightly-20eeb7c9"}
GIT_REPO=${GIT_REPO:-"https://github.com/wanglei4687/matrixone-operator.git"}
IMAGE_REPO=${IMAGE_REPO:-"matrixorigin/matrixone"}
DEST_DIR=${DEST_DIR:-"${__dir}/matrixone-operator"}
NAMESPACE=${NAMESPACE:-"matrixone-operator"}
KUBECONFIG=${KUBECONFIG:-"${__dir}/matrixone-operator/config.yaml"}

function clone_repo() {
    git clone "${1}" "${DEST_DIR}"
}

function clean() {
    echo "> Clean"
    # Delete clone repo dir
    echo "Remove repo..."
    rm -rf "${__dir}"/matrixone-operator
    # Uninstall helm charts
    echo "Uninstall helm charts..."
    helm uninstall mo -n "${NAMESPACE}"
    # Delete operator namespaces
    echo "Delete operator namespace..."
    kubectl delete ns "${NAMESPACE}"
    # Delete test ns
    echo "Delete test namespaces..."
    kubectl get ns --no-headers=true | awk '/e2e-/{print $1}' | xargs  kubectl delete ns
}

trap "clean" EXIT

echo "> Clone repo"
clone_repo "${GIT_REPO}"

echo "> Create namespaces"
kubectl create ns "${NAMESPACE}"

echo "> Install mo operator"
helm install mo "${__dir}"/matrixone-operator/charts/matrixone-operator --dependency-update -n "${NAMESPACE}"

echo "> Ensure operator is ready"
kubectl cluster-info
kubectl wait --for=condition=Ready pods --all -n "${NAMESPACE}" --timeout=30s

echo "> Wait webhook certificate injected"
sleep 30

echo "> Run e2e test"
cd "${__dir}"/matrixone-operator && make ginkgo
"${__dir}"/matrixone-operator/bin/ginkgo -stream -slowSpecThreshold=3000 "${__dir}"/matrixone-operator/test/e2e/... -- \
            -mo-version="${MO_VERSION}" \
            -mo-image-repo="${IMAGE_REPO}" \
            -kube-config="${KUBECONFIG}"
