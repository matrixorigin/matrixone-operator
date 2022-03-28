# Architecture

## Overview

```mermaid
graph LR
    l1(matrixone)
    l3(matrix monitor)
    l2(matrixcube)

    l1 -- metrics/log/tracing --> l3
    l2 -- metrics/log/tracing --> l3
    l1 -- schedule --> l2

```

## Matrixone

- matrixone as a compute layer
- default tree node for sechduler matrixcube

```mermaid
flowchart LR
    mo-0 -- raft --> mo-1  
    mo-1 -- raft --> mo-2 
    mo-2 --raft --> mo-0
```

## Matrixcube

matrixone scheduler matrixcube

```mermaid
flowchart LR
l1(matrixone)

cube-0  -- grpc --> l1
cube-1 -- grpc --> l1
cube-2 -- grpc --> l1
```

matrixcube consensus algorithm by raft

```mermaid
flowchart LR
cube-0 -- raft -->  cube-1
cube-1 -- raft --> cube-2
cube-2 -- raft --> cube-0
```

## Mmatrix monitor

```mermaid
flowchart LR
l1(matrixone)
l2(matrixcube)
l3(loki)
l4(prometheus)
l5(grafana)
l6(S3)
l7(thanso )
l8(mo-promtail)
l9(cube-promtail)
d1(Tempo)
d2(OpenTelemetry-mo)
d3(OPenTelemetry-cube)
    subgraph matrix monitor
    l3 -- storage --> l6
    l7 -- storage --> l6
    d1 -- stotage --> l6
    l3 -- observation --> l5
    d1 -- observation --> l5
    l4 --> l7 -- observation --> l5
    end

    subgraph matrixone
    l1  --> l8 -- log --> l3
    l1  --> d2--  metrics --> l4
    d2 -- tracing --> d1
    end

    subgraph matrixcube
    l2 --> d3 -- tracing --> d1
    d3 -- metrics --> l4
    l2 --> l9  -- log --> l3
    end
```

thanos architecture

![th-arch](img/thanos_arch.png)

thanos as  prometheus sidecar

![th-prom](img/thanos-prom.png)

## why loki

- horizontally scalable
- highly available
- multi-tenant

## Metadata mangement

TODO

## State management

TDDO

## Stateful service management

TODO

## Backup

TODO

## References

- [Improving HA and long-term storage for Prometheus using Thanos on EKS with S3](https://aws.amazon.com/cn/blogs/opensource/improving-ha-and-long-term-storage-for-prometheus-using-thanos-on-eks-with-s3/)
- [Loki tutorial: How to send logs from EKS with Promtail to get full visibility in Grafana](https://grafana.com/blog/2020/07/21/loki-tutorial-how-to-send-logs-from-eks-with-promtail-to-get-full-visibility-in-grafana/)
- [From Distributed Tracing to APM: Taking OpenTelemetry & Jaeger Up a Level](https://logz.io/blog/monitoring-microservices-opentelemetry-jaeger/)
- [Intro to distributed tracing with Tempo, OpenTelemetry, and Grafana Cloud](https://grafana.com/blog/2021/09/23/intro-to-distributed-tracing-with-tempo-opentelemetry-and-grafana-cloud/)
- [Jaeger vs Tempo - key features, differences, and alternatives](https://signoz.io/blog/jaeger-vs-tempo#:~:text=Both%20Grafana%20Tempo%20and%20Jaeger%20are%20tools%20aimed,as%20a%20project%20from%20Cloud%20Native%20Computing%20Foundation.)
- [Metrics, tracing, and logging](https://peter.bourgon.org/blog/2017/02/21/metrics-tracing-and-logging.html)
- [利用Opentelemetry+Loki+Temp+Granafa构建端到端的可观测平台](https://juejin.cn/post/7050134410229710884)