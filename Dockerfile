FROM golang:1.26.6 as builder

ARG GOPROXY="https://proxy.golang.org,direct"
ENV GOFLAGS=-p=4

WORKDIR /workspace

RUN go env -w GOPROXY=${GOPROXY}

# The pinned MatrixOne module uses CGO for its allocator and lock-service
# dependencies. Build native libraries from that exact module version rather
# than linking against libraries from a different MatrixOne checkout.
RUN apt-get update && apt-get install -y --no-install-recommends cmake g++ make bzip2 git && rm -rf /var/lib/apt/lists/*

COPY go.mod go.mod
COPY go.sum go.sum
COPY api/go.mod api/go.mod
COPY api/go.sum api/go.sum

# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN --mount=type=cache,id=gomodcache,target=/go/pkg/mod go mod download

RUN --mount=type=cache,id=gomodcache,target=/go/pkg/mod \
    mo_dir="$(go list -m -f '{{.Dir}}' github.com/matrixorigin/matrixone)" && \
    test -n "${mo_dir}" && \
    chmod u+w "${mo_dir}" && \
    chmod -R u+w "${mo_dir}/thirdparties" "${mo_dir}/cgo" && \
    chmod u+x "${mo_dir}/cgo/mo-native-provenance" "${mo_dir}/cgo/mo-stage-native-libs" "${mo_dir}/thirdparties/download" && \
    make -C "${mo_dir}" -j4 cgo

COPY . .

# Build
RUN --mount=type=cache,id=gomodcache,target=/go/pkg/mod \
    --mount=type=cache,id=gobuildcache,target=/root/.cache/go-build \
    mo_dir="$(go list -m -f '{{.Dir}}' github.com/matrixorigin/matrixone)" && \
    CGO_ENABLED=1 CGO_CFLAGS="-I${mo_dir}/thirdparties/install/include" \
    CGO_LDFLAGS="-L${mo_dir}/thirdparties/install/lib" \
    go build -o manager cmd/operator/main.go && \
    cp "${mo_dir}/thirdparties/install/lib/libusearch_c.so" /workspace/libusearch_c.so

# The manager and libusearch_c.so now require glibc and the C++/OpenMP runtime.
FROM debian:trixie-slim
RUN apt-get update && apt-get install -y --no-install-recommends libgomp1 libstdc++6 && rm -rf /var/lib/apt/lists/*
WORKDIR /
COPY --from=builder /workspace/manager .
COPY --from=builder /workspace/libusearch_c.so /usr/local/lib/
ENV LD_LIBRARY_PATH=/usr/local/lib
USER 65532:65532

ENTRYPOINT ["/manager"]
