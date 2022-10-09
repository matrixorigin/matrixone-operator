#!/usr/bin/env bash

if [ -z "$ROOT" ]; then
    echo "error: ROOT should be initialized"
    exit 1
fi

OS=$(go env GOOS)
ARCH=$(go env GOARCH)
BIN=${ROOT}/bin
KUBECTL_VERSION=${KUBECTL_VERSION:-1.24.2}
KUBECTL_BIN=$BIN/kubectl
HELM_BIN=$BIN/helm
HELM_VERSION=${HELM_VERSION:-3.5.0}
KIND_BIN=$BIN/kind
KIND_VERSION=${KIND_VERSION:-0.14.0}
export PATH=$PATH:${BIN}

test -d "$BIN" || mkdir -p "$BIN"

function hack::ensure_kubectl() {
    if command -v kubectl &> /dev/null; then
        return 0
    fi
    echo "Installing kubectl v$KUBECTL_VERSION..."
    tmpfile=$(mktemp)
    trap "test -f $tmpfile && rm $tmpfile" RETURN
    curl --retry 10 -L -o $tmpfile https://storage.googleapis.com/kubernetes-release/release/v${KUBECTL_VERSION}/bin/${OS}/${ARCH}/kubectl
    mv $tmpfile $KUBECTL_BIN
    chmod +x $KUBECTL_BIN
}

function hack::ensure_helm() {
    if command -v helm &> /dev/null; then
        return 0
    fi
    echo "Installing helm ${HELM_VERSION}..."
    local HELM_URL=https://get.helm.sh/helm-v${HELM_VERSION}-${OS}-${ARCH}.tar.gz
    curl --retry 3 -L -s "$HELM_URL" | tar --strip-components 1 -C $BIN -zxf - ${OS}-${ARCH}/helm
}

function hack::ensure_kind() {
    if command -v kind &> /dev/null; then
        return 0
    fi
    echo "Installing kind v$KIND_VERSION..."
    tmpfile=$(mktemp)
    trap "test -f $tmpfile && rm $tmpfile" RETURN
    curl --retry 10 -L -o $tmpfile https://github.com/kubernetes-sigs/kind/releases/download/v${KIND_VERSION}/kind-$(uname)-amd64
    mv $tmpfile $KIND_BIN
    chmod +x $KIND_BIN
}
