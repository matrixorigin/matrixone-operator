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

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
REPO_ROOT=$(cd "${SCRIPT_DIR}/.." && pwd)
TEST_ROOT=$(mktemp -d)
trap 'rm -rf -- "${TEST_ROOT}"' EXIT

bash -n "${REPO_ROOT}/hack/package-chart.sh" \
    "${REPO_ROOT}/hack/test-kruise-webhook-outage.sh" \
    "${REPO_ROOT}/hack/lib.sh"

helm template test "${REPO_ROOT}/charts/kruise" >"${TEST_ROOT}/kruise.yaml"

# Built-in API operations must remain available during a webhook outage. Kruise
# custom resources remain fail-closed because their webhooks own their contract.
awk '
    /^[[:space:]]+failurePolicy:/ { policy = $2 }
    /^[[:space:]]+name: [a-z].*\.kb\.io$/ {
        name = $2
        expected = "Fail"
        if (name == "mpod.kb.io" || name == "vpod.kb.io" || name == "vpodeviction.kb.io" ||
            name ~ /^vbuiltin/ || name == "vcustomresourcedefinition.kb.io" ||
            name == "vnamespace.kb.io" || name == "vingress.kb.io" || name == "vservice.kb.io") {
            expected = "Ignore"
        }
        if (policy != expected) {
            printf "webhook %s has failurePolicy %s, want %s\n", name, policy, expected > "/dev/stderr"
            failed = 1
        }
    }
    END { exit failed }
' "${TEST_ROOT}/kruise.yaml"

if grep -q 'StatefulSetAutoResizePVCGate=true' "${TEST_ROOT}/kruise.yaml"; then
    echo "StatefulSetAutoResizePVCGate must remain disabled by default" >&2
    exit 1
fi

if ! awk '
    /- storageclasses$/ { in_rule = 1; next }
    in_rule && /- get$/ { get = 1 }
    in_rule && /- list$/ { list = 1 }
    in_rule && /- watch$/ { watch = 1 }
    in_rule && /^---$/ { exit !(get && list && watch) }
    END { if (in_rule) exit !(get && list && watch) }
' "${TEST_ROOT}/kruise.yaml"; then
    echo "Kruise must have read-only get/list/watch access to StorageClasses" >&2
    exit 1
fi

# Reproduce a dirty developer workspace in a temporary source tree. The stale
# archive must not be copied into the operator package.
mkdir -p "${TEST_ROOT}/source/charts" "${TEST_ROOT}/packages"
cp -R "${REPO_ROOT}/charts/matrixone-operator" "${TEST_ROOT}/source/charts/"
cp -R "${REPO_ROOT}/charts/kruise" "${TEST_ROOT}/source/charts/"
mkdir -p "${TEST_ROOT}/source/charts/matrixone-operator/charts"
printf 'stale archive\n' >"${TEST_ROOT}/source/charts/matrixone-operator/charts/stale-9.9.9.tgz"

SOURCE_ROOT="${TEST_ROOT}/source" "${REPO_ROOT}/hack/package-chart.sh" "${TEST_ROOT}/packages" >/dev/null
operator_package=$(find "${TEST_ROOT}/packages" -maxdepth 1 -name 'matrixone-operator-*.tgz' -print -quit)
if [[ -z "${operator_package}" ]]; then
    echo "operator package was not created" >&2
    exit 1
fi

dependencies=$(tar -tzf "${operator_package}" | awk -F/ '$1 == "matrixone-operator" && $2 == "charts" && $3 != "" { print $3 }' | sort -u)
if [[ "${dependencies}" != "kruise" ]]; then
    echo "unexpected packaged dependencies:" >&2
    printf '%s\n' "${dependencies}" >&2
    exit 1
fi
