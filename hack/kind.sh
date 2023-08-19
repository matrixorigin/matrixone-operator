#!/usr/bin/env bash

set -euo pipefail

ROOT=$(cd $(dirname ${BASH_SOURCE[0]})/.. && pwd)
cd ${ROOT}
source ${ROOT}/hack/lib.sh

hack::ensure_kubectl
hack::ensure_helm
hack::ensure_kind

CLUSTER=${CLUSTER:-"local-mo"}

function up() {
    echo "> Start operator on kind"

    trap "kind::cleanup ${CLUSTER}" EXIT
    kind::ensure-kind
    kind::load-image
    kind::install-minio

    trap "e2e::cleanup" EXIT
    e2e::install

    echo "> Hold kind cluster, press Ctrl+C to exit"
    tail -f /dev/null
}

case $1 in
  up)
    up
    ;;
  *)
    echo "Usage: $0 up"
    exit 1
esac
