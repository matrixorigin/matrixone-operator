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

if [[ $# -ne 3 ]]; then
    echo "usage: $0 OPERATOR_CHART RELEASE RELEASE_NAMESPACE" >&2
    exit 2
fi

operator_chart=$1
release=$2
release_namespace=$3
kruise_namespace=kruise-system
test_namespace=kruise-webhook-outage-test
webhook_service=kruise-webhook-service
manager_deployment=kruise-controller-manager
needs_recovery=true

recover() {
    local status=$?
    kubectl delete namespace "${test_namespace}" --ignore-not-found --wait=false >/dev/null 2>&1 || true
    if [[ "${needs_recovery}" == true ]]; then
        echo "> Restore Kruise webhook after outage test"
        helm upgrade "${release}" "${operator_chart}" -n "${release_namespace}" --reuse-values \
            --wait --timeout=5m >/dev/null 2>&1 || true
    fi
    exit "${status}"
}
trap recover EXIT

echo "> Simulate Kruise webhook service outage"
# Change a chart-managed selector to remove all endpoints without stopping the
# controller. Helm upgrade must restore the declared selector value.
kubectl -n "${kruise_namespace}" patch service "${webhook_service}" --type=merge \
    -p '{"spec":{"selector":{"control-plane":"reliability-outage"}}}' >/dev/null

for _ in $(seq 1 30); do
    endpoints=$(kubectl -n "${kruise_namespace}" get endpoints "${webhook_service}" \
        -o jsonpath='{.subsets}' 2>/dev/null || true)
    [[ -z "${endpoints}" ]] && break
    sleep 1
done
if [[ -n "${endpoints:-}" ]]; then
    echo "Kruise webhook endpoints did not become empty" >&2
    exit 1
fi

kubectl create namespace "${test_namespace}" >/dev/null
kubectl -n "${test_namespace}" apply -f - >/dev/null <<'EOF'
apiVersion: v1
kind: Pod
metadata:
  name: admitted-during-outage
spec:
  restartPolicy: Never
  containers:
    - name: main
      image: busybox:1.36
      command: ["sh", "-c", "exit 0"]
EOF

set +e
failure_output=$(kubectl -n "${test_namespace}" apply -f - 2>&1 <<'EOF'
apiVersion: apps.kruise.io/v1alpha1
kind: CloneSet
metadata:
  name: rejected-during-outage
spec:
  replicas: 0
  selector:
    matchLabels:
      app: rejected-during-outage
  template:
    metadata:
      labels:
        app: rejected-during-outage
    spec:
      containers:
        - name: main
          image: busybox:1.36
EOF
)
failure_status=$?
set -e
if [[ ${failure_status} -eq 0 ]]; then
    echo "Kruise custom resource unexpectedly passed admission during webhook outage" >&2
    exit 1
fi
if ! grep -Eq 'failed calling webhook|no endpoints available|context deadline exceeded' <<<"${failure_output}"; then
    echo "Kruise custom resource failed for an unexpected reason: ${failure_output}" >&2
    exit 1
fi

echo "> Upgrade release and recover Kruise webhook service"
helm upgrade "${release}" "${operator_chart}" -n "${release_namespace}" --reuse-values \
    --wait --timeout=5m >/dev/null
kubectl -n "${kruise_namespace}" rollout status deployment "${manager_deployment}" --timeout=5m >/dev/null

for _ in $(seq 1 60); do
    endpoints=$(kubectl -n "${kruise_namespace}" get endpoints "${webhook_service}" \
        -o jsonpath='{.subsets}' 2>/dev/null || true)
    [[ -n "${endpoints}" ]] && break
    sleep 1
done
if [[ -z "${endpoints:-}" ]]; then
    echo "Kruise webhook endpoints did not recover after upgrade" >&2
    exit 1
fi
needs_recovery=false

kubectl -n "${test_namespace}" apply -f - >/dev/null <<'EOF'
apiVersion: v1
kind: Pod
metadata:
  name: admitted-after-recovery
spec:
  restartPolicy: Never
  containers:
    - name: main
      image: busybox:1.36
      command: ["sh", "-c", "exit 0"]
EOF

echo "> Kruise webhook outage and recovery behavior verified"
