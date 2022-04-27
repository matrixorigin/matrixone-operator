ARG PROXY="https://proxy.golang.org,driect"

FROM golang:1.18 as builder

WORKDIR /workspace

RUN go env -w GOPROXY=${PROXY}

COPY go.mod go.mod
COPY go.sum go.sum

# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

COPY . .

# Build
RUN CGO_ENABLED=0 go build -a -o operator cmd/operator/main.go

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=builder /workspace/operator .
USER 65532:65532

ENTRYPOINT ["/operator"]
