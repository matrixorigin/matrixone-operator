#!/usr/bin/env bash

set -euo pipefail

ROOT=$(cd $(dirname ${BASH_SOURCE[0]})/.. && pwd)
cd ${ROOT}
source ${ROOT}/hack/lib.sh

CLUSTER=${CLUSTER:-"mo"}

hack::ensure_kubectl
hack::ensure_helm
hack::ensure_kind

function e2e::kind-e2e() {
    echo "> Start kind e2e test"

    trap "kind::cleanup ${CLUSTER}" EXIT
    kind::ensure-kind
    kind::load-image
    kind::install-minio
    echo "> Run e2e test"
    bash ./hack/e2e.sh
}

e2e::kind-e2e
exec "$@"
