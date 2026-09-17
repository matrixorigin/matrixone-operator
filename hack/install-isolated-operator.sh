#!/usr/bin/env bash

# Copyright 2026 Matrix Origin
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -euo pipefail

usage() {
    cat <<'EOF'
Usage:
  install-isolated-operator.sh \
    --namespace TEST_NAMESPACE \
    --release HELM_RELEASE \
    --repository IMAGE_REPOSITORY \
    --tag IMAGE_TAG \
    [--chart OPERATOR_CHART]

Install a MatrixOne Operator for shared-cluster E2E validation without
modifying the shared cluster-level Operator. The namespace must be dedicated
to E2E and is labelled so the Operator webhooks only match that namespace.
EOF
}

die() {
    echo "error: $*" >&2
    exit 1
}

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)

namespace=""
release=""
repository=""
tag=""
chart=""

while [[ $# -gt 0 ]]; do
    case "$1" in
        --namespace)
            namespace=${2:-}
            shift 2
            ;;
        --release)
            release=${2:-}
            shift 2
            ;;
        --repository)
            repository=${2:-}
            shift 2
            ;;
        --tag)
            tag=${2:-}
            shift 2
            ;;
        --chart)
            chart=${2:-}
            shift 2
            ;;
        -h|--help)
            usage
            exit 0
            ;;
        *)
            usage >&2
            die "unknown argument: $1"
            ;;
    esac
done

[[ -n "${namespace}" ]] || die "--namespace is required"
[[ -n "${release}" ]] || die "--release is required"
[[ -n "${repository}" ]] || die "--repository is required"
[[ -n "${tag}" ]] || die "--tag is required"

dns1123='^[a-z0-9]([-a-z0-9]*[a-z0-9])?$'
[[ "${namespace}" =~ ${dns1123} ]] || die "invalid namespace: ${namespace}"
[[ "${release}" =~ ${dns1123} ]] || die "invalid release name: ${release}"

case "${namespace}" in
    default|kube-system|kube-public|kube-node-lease|kruise-system|matrixone|mo-system)
        die "refusing to install an E2E Operator in shared namespace ${namespace}"
        ;;
esac

if [[ ! "${namespace}" =~ (^|-)e2e(-|$) ]]; then
    die "E2E namespace must contain an 'e2e' name segment: ${namespace}"
fi

package_root=""
cleanup() {
    if [[ -n "${package_root}" ]]; then
        rm -rf -- "${package_root}"
    fi
}
trap cleanup EXIT

if [[ -z "${chart}" ]]; then
    package_root=$(mktemp -d)
    package_chart=${PACKAGE_CHART:-"${SCRIPT_DIR}/package-chart.sh"}
    chart=$("${package_chart}" "${package_root}")
fi

isolation_label='matrixorigin.io/operator-e2e'
if existing_label=$(kubectl get namespace "${namespace}" \
    -o "jsonpath={.metadata.labels.matrixorigin\\.io/operator-e2e}" 2>/dev/null); then
    if [[ "${existing_label}" != "true" ]]; then
        die "existing namespace ${namespace} is not labelled ${isolation_label}=true"
    fi
else
    kubectl create namespace "${namespace}"
    kubectl label namespace "${namespace}" \
        "${isolation_label}=true" \
        'app.kubernetes.io/managed-by=matrixone-operator-e2e' \
        --overwrite
fi

helm upgrade --install "${release}" "${chart}" \
    --namespace "${namespace}" \
    --set-string "image.repository=${repository}" \
    --set-string "image.tag=${tag}" \
    --set onlyWatchReleasedNS=true \
    --set kruise.enabled=false \
    --set-string 'webhook.namespaceSelector.matchLabels.matrixorigin\.io/operator-e2e=true' \
    --atomic \
    --wait \
    --timeout 5m

echo "isolated MatrixOne Operator ${release} is ready in namespace ${namespace}"
