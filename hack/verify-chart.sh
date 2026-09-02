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

# Pod admission and Kruise custom resources remain fail-closed because their
# mutation/validation contracts cannot be repaired retroactively. Other built-in
# validators remain fail-open. The rendered chart currently emits failurePolicy
# before name inside each webhook block, which this check intentionally parses.
awk '
    /^[[:space:]]+failurePolicy:/ { policy = $2 }
    /^[[:space:]]+name: [a-z].*\.kb\.io$/ {
        name = $2
        expected = "Fail"
        if (name ~ /^vbuiltin/ || name == "vcustomresourcedefinition.kb.io" ||
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

# Parse RBAC rule boundaries so verbs from an adjacent rule cannot produce a
# false positive when the upstream template changes ordering.
if ! awk '
    function finish_rule() {
        if (has_storage_group && has_storageclasses) {
            found = 1
            if (!(has_get && has_list && has_watch)) failed = 1
        }
        has_storage_group = has_storageclasses = has_get = has_list = has_watch = 0
    }
    /^- apiGroups:$/ { finish_rule(); section = "apiGroups"; next }
    /^  resources:$/ { section = "resources"; next }
    /^  verbs:$/ { section = "verbs"; next }
    /^    - / {
        value = substr($0, 7)
        if (section == "apiGroups" && value == "storage.k8s.io") has_storage_group = 1
        if (section == "resources" && value == "storageclasses") has_storageclasses = 1
        if (section == "verbs" && value == "get") has_get = 1
        if (section == "verbs" && value == "list") has_list = 1
        if (section == "verbs" && value == "watch") has_watch = 1
    }
    /^---$/ { finish_rule(); section = "" }
    END { finish_rule(); exit failed || !found }
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

# Chart.lock is intentionally ignored by this repository. A developer-local
# lock file must not leak into the deterministic package.
if tar -tzf "${operator_package}" | awk '/\/Chart.lock$/ { found = 1 } END { exit !found }'; then
    echo "ignored Chart.lock leaked into the operator package" >&2
    exit 1
fi
