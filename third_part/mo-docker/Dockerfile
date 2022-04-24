FROM golang:1.18 as builder

ARG BRANCH=main
ARG PROXY="https://proxy.golang.org,direct"

RUN mkdir -p /go/src/github.com/matrixorigin/matrixone

RUN git clone -b ${BRANCH}  https://github.com/matrixorigin/matrixone.git /go/src/github.com/matrixorigin/matrixone

WORKDIR /go/src/github.com/matrixorigin/matrixone

RUN go env -w GOPROXY=${PROXY}

RUN make config && make build


FROM ubuntu

COPY --from=builder /go/src/github.com/matrixorigin/matrixone/mo-server /mo-server
COPY system_vars_config.toml /system_vars_config.toml
COPY --chmod=755 entrypoint.sh /entrypoint.sh

WORKDIR /

EXPOSE 6001

ENTRYPOINT ["/entrypoint.sh"]
