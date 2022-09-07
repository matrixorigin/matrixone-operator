#!/bin/bash
set -euo pipefail

ROOT=$(cd $(dirname ${BASH_SOURCE[0]})/.. && pwd)
cd ${ROOT}

for i in $(find . -name \*.go); do
  if ! grep -q Copyright $i
  then
    cat ./hack/boilerplate.go.txt $i >$i.new && mv $i.new $i
  fi
done
