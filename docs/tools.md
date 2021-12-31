# Matrixone Tools

Tools for matrixone cluster

## Mysql Client Connector

Mysql Client for connecting Matrixone Cluster

```shell
docker build docker build tools/mysql-client/ -t matrixorigin/client:0.0.1
```

### How to use

kubernetes

```shell
kubectl run mysql-client --image=--image=matrixoneorigin/client:0.0.1 -it --rm --restart=Never -- /bin/bash
```

docker

```shell
docker run -it --rm --name mo-client matrixoneorigin/client:0.0.1 mysql -h ${HOST} -P ${PORT} -u${USER} -p${PWD}
```
