FROM golang:1.18-alpine3.14 as builder

ARG BRANCH=main
ARG PROXY="https://proxy.golang.org,direct"

RUN apk add --no-cache \
    wget \
    make \
    git \
    gcc \
    binutils-gold \
    musl-dev

RUN wget -O /usr/local/bin/dumb-init https://github.com/Yelp/dumb-init/releases/download/v1.2.2/dumb-init_1.2.2_amd64 \
 && chmod +x /usr/local/bin/dumb-init

RUN mkdir -p /go/src/github.com/matrixorigin/matrixone

RUN git clone -b ${BRANCH}  https://github.com/matrixorigin/matrixone.git /go/src/github.com/matrixorigin/matrixone

WORKDIR /go/src/github.com/matrixorigin/matrixone

RUN go env -w GOPROXY=${PROXY}

RUN make config && make build


FROM alpine

COPY --from=builder /go/src/github.com/matrixorigin/matrixone/mo-server /mo-server
COPY --from=builder /usr/local/bin/dumb-init /usr/local/bin/dumb-init
COPY system_vars_config.toml /system_vars_config.toml
COPY --chmod=755 entrypoint.sh /entrypoint.sh

WORKDIR /

EXPOSE 6001

ENTRYPOINT ["/entrypoint.sh"]
