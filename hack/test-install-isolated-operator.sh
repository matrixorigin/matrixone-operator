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
INSTALLER="${SCRIPT_DIR}/install-isolated-operator.sh"
TEST_ROOT=$(mktemp -d)
trap 'rm -rf -- "${TEST_ROOT}"' EXIT

BIN_DIR="${TEST_ROOT}/bin"
COMMAND_LOG="${TEST_ROOT}/commands.log"
mkdir -p "${BIN_DIR}"

cat >"${BIN_DIR}/kubectl" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
printf 'kubectl' >>"${COMMAND_LOG}"
printf ' %q' "$@" >>"${COMMAND_LOG}"
printf '\n' >>"${COMMAND_LOG}"

if [[ "${1:-}" == "get" && "${2:-}" == "namespace" ]]; then
    case "${TEST_NAMESPACE_STATE:-missing}" in
        missing)
            exit 1
            ;;
        managed)
            printf 'true'
            ;;
        unmanaged)
            printf 'false'
            ;;
    esac
fi
EOF

cat >"${BIN_DIR}/helm" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
printf 'helm' >>"${COMMAND_LOG}"
printf ' %q' "$@" >>"${COMMAND_LOG}"
printf '\n' >>"${COMMAND_LOG}"
EOF

cat >"${TEST_ROOT}/package-chart" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
printf 'package-chart' >>"${COMMAND_LOG}"
printf ' %q' "$@" >>"${COMMAND_LOG}"
printf '\n' >>"${COMMAND_LOG}"
printf '%s/operator.tgz\n' "${TEST_ROOT}"
EOF

chmod +x "${BIN_DIR}/kubectl" "${BIN_DIR}/helm" "${TEST_ROOT}/package-chart"
export COMMAND_LOG TEST_ROOT
export PATH="${BIN_DIR}:${PATH}"

fail() {
    echo "FAIL: $*" >&2
    exit 1
}

assert_log_contains() {
    local pattern=$1
    grep -F -- "${pattern}" "${COMMAND_LOG}" >/dev/null || {
        cat "${COMMAND_LOG}" >&2
        fail "command log does not contain: ${pattern}"
    }
}

assert_log_not_contains() {
    local pattern=$1
    if grep -F -- "${pattern}" "${COMMAND_LOG}" >/dev/null; then
        cat "${COMMAND_LOG}" >&2
        fail "command log unexpectedly contains: ${pattern}"
    fi
}

run_installer() {
    "${INSTALLER}" \
        --namespace "${1}" \
        --release "${2:-udf-e2e}" \
        --repository example.invalid/matrixone-operator \
        --tag test \
        --chart "${TEST_ROOT}/chart"
}

run_installer_with_packaged_chart() {
    PACKAGE_CHART="${TEST_ROOT}/package-chart" \
        "${INSTALLER}" \
        --namespace mo-python-udf-e2e-packaged \
        --release udf-e2e \
        --repository example.invalid/matrixone-operator \
        --tag test
}

: >"${COMMAND_LOG}"
if TEST_NAMESPACE_STATE=managed run_installer matrixone; then
    fail "shared matrixone namespace was accepted"
fi
assert_log_not_contains "helm"

: >"${COMMAND_LOG}"
if TEST_NAMESPACE_STATE=unmanaged run_installer mo-python-udf-e2e-existing; then
    fail "an existing namespace without the isolation label was accepted"
fi
assert_log_not_contains "helm"

: >"${COMMAND_LOG}"
TEST_NAMESPACE_STATE=missing run_installer mo-python-udf-e2e-new
assert_log_contains "kubectl create namespace mo-python-udf-e2e-new"
assert_log_contains "kubectl label namespace mo-python-udf-e2e-new matrixorigin.io/operator-e2e=true"
assert_log_contains "helm upgrade --install udf-e2e"
assert_log_contains "onlyWatchReleasedNS=true"
assert_log_contains "kruise.enabled=false"
assert_log_contains "webhook.namespaceSelector.matchLabels.matrixorigin\\\\.io/operator-e2e=true"
assert_log_not_contains "set image"

: >"${COMMAND_LOG}"
TEST_NAMESPACE_STATE=managed run_installer mo-python-udf-e2e-existing
assert_log_not_contains "create namespace"
assert_log_contains "helm upgrade --install udf-e2e"

: >"${COMMAND_LOG}"
TEST_NAMESPACE_STATE=missing run_installer_with_packaged_chart
assert_log_contains "package-chart"
assert_log_contains "helm upgrade --install udf-e2e ${TEST_ROOT}/operator.tgz"

echo "isolated operator installer tests passed"
