FROM golang:1.17 as builder

RUN mkdir -p /go/src/github.com/matrixorigin/mysql-tester

RUN git clone https://github.com/matrixorigin/mysql-tester.git /go/src/github.com/matrixorigin/mysql-tester

WORKDIR /go/src/github.com/matrixorigin/mysql-tester

RUN go env -w GOPROXY=https://goproxy.cn,direct && go env -w GO111MODULE=on

RUN make build

FROM ubuntu

COPY --from=builder /go/src/github.com/matrixorigin/mysql-tester/mysql-tester /mysql-tester
COPY --from=builder /usr/local/bin/dumb-init /usr/local/bin/dumb-init
COPY --from=builder /go/src/github.com/matrixorigin/mysql-tester/r /r
COPY --from=builder /go/src/github.com/matrixorigin/mysql-tester/t /t

WORKDIR /

ENTRYPOINT ["/mysql-tester"]
