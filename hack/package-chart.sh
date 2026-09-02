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
SOURCE_ROOT=${SOURCE_ROOT:-${REPO_ROOT}}
OUTPUT_DIR=${1:-"${REPO_ROOT}/charts"}
STAGING_ROOT=$(mktemp -d)
trap 'rm -rf -- "${STAGING_ROOT}"' EXIT

OPERATOR_SOURCE="${SOURCE_ROOT}/charts/matrixone-operator"
KRUISE_SOURCE="${SOURCE_ROOT}/charts/kruise"
OPERATOR_STAGING="${STAGING_ROOT}/matrixone-operator"

mkdir -p "${OPERATOR_STAGING}/charts" "${OUTPUT_DIR}"
cp "${OPERATOR_SOURCE}/Chart.yaml" "${OPERATOR_SOURCE}/values.yaml" "${OPERATOR_STAGING}/"
cp -R "${OPERATOR_SOURCE}/templates" "${OPERATOR_STAGING}/"
if [[ -f "${OPERATOR_SOURCE}/.helmignore" ]]; then
    cp "${OPERATOR_SOURCE}/.helmignore" "${OPERATOR_STAGING}/"
fi

# Package only the dependency declared by this repository. In particular, do
# not copy or inspect OPERATOR_SOURCE/charts, which may contain ignored archives
# left by an earlier local build.
helm package "${KRUISE_SOURCE}" --destination "${OPERATOR_STAGING}/charts" >/dev/null
helm package "${OPERATOR_STAGING}" --destination "${OUTPUT_DIR}" | awk '{print $NF}'
