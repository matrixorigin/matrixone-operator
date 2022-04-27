FROM golang:1.17 as builder

ARG PROXY="https://proxy.golang.org,driect"

RUN mkdir -p /go/src/github.com/matrixorigin/mysql-tester

RUN git clone https://github.com/matrixorigin/mysql-tester.git /go/src/github.com/matrixorigin/mysql-tester

WORKDIR /go/src/github.com/matrixorigin/mysql-tester

RUN  go env -w GOPROXY=${PROXY}
RUN make build

FROM ubuntu

COPY --from=builder /go/src/github.com/matrixorigin/mysql-tester/mysql-tester /mysql-tester
COPY --from=builder /go/src/github.com/matrixorigin/mysql-tester/r /r
COPY --from=builder /go/src/github.com/matrixorigin/mysql-tester/t /t

WORKDIR /

ENTRYPOINT ["/mysql-tester"]
