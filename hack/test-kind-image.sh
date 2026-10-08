#!/usr/bin/env bash
# Copyright 2026 Matrix Origin

set -euo pipefail
ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
source "${ROOT}/hack/lib.sh"
ARCH=amd64
test_dir=$(mktemp -d)
trap 'rm -rf -- "${test_dir}"' EXIT
calls="${test_dir}/calls"

docker() {
    printf 'docker %s\n' "$*" >> "${calls}"
    case "$1 $2" in
        "image inspect")
            [[ "${cached}" == true ]] || return 1
            printf 'linux/amd64\n'
            ;;
        "image save")
            if [[ "$3" == --help ]]; then
                [[ "${modern}" != true ]] || printf ' --platform string\n'
            else
                [[ "${save_error}" == 0 ]] || return "${save_error}"
                printf 'archive' > "$4"
            fi
            ;;
        "pull --platform") return "${pull_error}" ;;
        "build --platform") return "${build_error}" ;;
        *) echo "unexpected docker invocation: $*" >&2; return 99 ;;
    esac
}

kind() {
    printf 'kind %s\n' "$*" >> "${calls}"
    [[ "$1 $2 $3 $4" == "load image-archive --name fixture" ]]
    [[ -s "$5" ]]
    return "${load_error}"
}

reset_case() {
    : > "${calls}"
    cached=true modern=true pull_error=0 save_error=0 load_error=0 build_error=0
}

reset_case
kind::prepare_image fixture openkruise/kruise-helm-hook:v0.1.0
! grep -q 'docker pull' "${calls}"
grep -q -- '--platform linux/amd64 openkruise/kruise-helm-hook:v0.1.0' "${calls}"
grep -q 'kind load image-archive' "${calls}"

reset_case
cached=false
kind::prepare_image fixture image:tag
grep -q 'docker pull --platform linux/amd64 image:tag' "${calls}"

reset_case
modern=false
kind::prepare_image fixture image:tag
! grep -q -- '--platform' "${calls}"

for failure in pull save load; do
    reset_case
    case "${failure}" in
        pull) cached=false; pull_error=7 ;;
        save) save_error=8 ;;
        load) load_error=9 ;;
    esac
    if kind::prepare_image fixture image:tag; then
        echo "expected ${failure} failure" >&2
        exit 1
    else
        status=$?
    fi
    case "${failure}" in
        pull) [[ "${status}" == 7 ]]; ! grep -q 'docker image save' "${calls}" ;;
        save) [[ "${status}" == 8 ]]; ! grep -q 'kind load' "${calls}" ;;
        load) [[ "${status}" == 9 ]] ;;
    esac
done

reset_case
CLUSTER=fixture
kind::prepare_minio
grep -q -- "docker build --platform linux/amd64 -f ${ROOT}/hack/Dockerfile.minio -t ${E2E_MINIO_IMAGE}" "${calls}"
grep -q -- "--platform linux/amd64 ${E2E_MINIO_IMAGE}" "${calls}"
! grep -q 'docker pull' "${calls}"

reset_case
build_error=10
if kind::prepare_minio; then
    echo 'expected MinIO build failure' >&2
    exit 1
else
    [[ "$?" == 10 ]]
fi
! grep -q 'docker image inspect' "${calls}"
! grep -q 'kind load' "${calls}"

kubectl() {
    printf 'kubectl %s\n' "$*" >> "${calls}"
    [[ "$*" == '-n default apply -f -' ]]
    local manifest
    manifest=$(cat)
    [[ "${manifest}" == *"image: ${E2E_MINIO_IMAGE}"* ]]
    [[ "${manifest}" != *'image: quay.io/minio/minio:'* ]]
    # The real multi-document fixture must keep its Service and Secret as well.
    [[ "${manifest}" == *'kind: StatefulSet'* ]]
    [[ "${manifest}" == *'kind: Service'* ]]
    [[ "${manifest}" == *'kind: Secret'* ]]
}
reset_case
kind::install-minio
grep -q 'kubectl -n default apply -f -' "${calls}"

echo 'PASS: platform-scoped kind images, pinned MinIO build/install and failure propagation'
