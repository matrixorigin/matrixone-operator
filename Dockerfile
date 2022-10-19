FROM golang:1.19 as builder

ARG GOPROXY="https://proxy.golang.org,direct"

WORKDIR /workspace

RUN go env -w GOPROXY=${GOPROXY}

COPY go.mod go.mod
COPY go.sum go.sum
COPY api/go.mod api/go.mod
COPY api/go.sum api/go.sum
COPY runtime/go.mod runtime/go.mod
COPY runtime/go.sum runtime/go.sum

# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

COPY . .

# Build
RUN CGO_ENABLED=0 go build -a -o manager cmd/operator/main.go

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=builder /workspace/manager .
USER 65532:65532

ENTRYPOINT ["/manager"]
