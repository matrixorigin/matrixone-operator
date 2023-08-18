#!/usr/bin/env bash
#set -euo pipefail

ROOT=$(cd $(dirname ${BASH_SOURCE[0]})/.. && pwd)
cd ${ROOT}
source ${ROOT}/hack/lib.sh

function start() {
  echo "> Start e2e workflow"
  e2e::workflow
}

start
