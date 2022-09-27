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

# set default variables
__dir="$(cd "$(dirname "BASH_SOURCE[0]")" && pwd)"

function clone_repo() {
    git clone ${1} ${__dir}/matrixone-operator
}

function prepare_image() {
    docker pull ${1}
}

function clean() {
#    rm -rf ${__dir}/matrixone-operator
    helm uninstall mo
}

MO_VERSION=${MO_VERSION:-"nightly-20eeb7c9"}
GIT_REPO=${GIT_REPO:-"https://github.com/matrixorigin/matrixone-operator.git"}
IMAGE_REPO=${IMAGE_REPO:-"matrixorigin/matrixone"}
KUBECONFIG=${KUBECONFIG:-"/Users/lei/.kube/config"}

trap "clean" EXIT

echo "> Clone repo"
clone_repo ${GIT_REPO}

echo "> Prepare e2e images"
prepare_image "${IMAGE_REPO}:${MO_VERSION}"
prepare_image openkruise/kruise-manager:v1.2.0

echo "> Install mo operator"
helm install mo ${__dir}/matrixone-operator/charts/matrixone-operator --dependency-update

echo "> Ensure k8s cluster is ready"
kubectl cluster-info
kubectl wait --for=condition=Ready pods --all --all-namespaces --timeout=30s

echo "> Wait webhook certificate injected"
sleep 30

echo "> Run e2e test"
cd ${__dir}/matrixone-operator && make ginkgo && \
    ${__dir}/matrixone-operator/bin/ginkgo -stream -slowSpecThreshold=3000 ${__dir}/matrixone-operator/test/e2e/... -- \
            -mo-version=${MO_VERSION} \
            -mo-image-repo=${IMAGE_REPO} \
            -kube-config=${KUBECONFIG}
