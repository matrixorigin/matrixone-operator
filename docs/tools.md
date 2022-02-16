# Matrixone Tools

Tools for Matrixone Cluster

## Mysql Client Connector

Mysql Client for connecting Matrixone Cluster

### Build mysql client docker image

```shell
docker build tools/mysql-client/ -t matrixorigin/client:0.0.1
```

### How to use

kubernetes

```shell
kubectl run mysql-client --image=matrixorigin/client:0.0.1 -it --rm --restart=Never -- -h ${HOST} -P ${PORT} -u${USER} -p${PWD}
```

docker

```shell
docker run -it --rm --name mo-client matrixorigin/client:0.0.1 -h ${HOST} -P ${PORT} -u${USER} -p${PWD}
```

## BVT Test

Test Cluster by [mysql-tester](https://github.com/matrixorigin/mysql-tester)

### Build BVT Test docker image

```shell
docker build tools/bvt-test/ -t matrixorigin/mysql-tester:0.0.1
```

### How to use

kubernetes

```shell

kubectl run bvt-tester --image=matrixorigin/mysql-tester:0.0.1 -it -rm --restart=Never -- -host ${HOST}  -port  ${PORT} -user ${USER} -passwd ${PWD} 

```

docker

```shell
docker run -it --rm --name bvt-tester matrixorigin/mysql-tester:0.0.1 -host ${HOST}  -port  ${PORT} -user ${USER} -passwd ${PWD} 
```
