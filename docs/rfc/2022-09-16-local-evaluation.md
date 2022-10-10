# MatrixOne Operator Local Evaluation

Prerequisites

- Docker

## Components of image

image based on `docker:22.06-rc-dind`

- awscli
- kubectl
- kind
- helm
- helm repo (operator, minio)
- modev (command line tools)

## Start a matrixone cluster

First, We should pull image from docker hub

```shell
docker pull matrixorigin/matrixone-operator:playground
```

Second, Start playground docker container

```shell
docker run -d --name playground --privileged -it  -p 6001:6001 -p 6002:80 matrixorigin/matrixone-operator:playground
```

port 6001: mysql server
port 6002: webui service

Connect mysql server by mysql client

```shell
mysql -h 127.0.0.1 -P 6001 -udump -p111
```

See webui: `127.0.0.1:60021`


### How to play operator and mo cluster

```shell
docker exec -it playground /bin/sh
```

Then you can do some operation on container terminate

You can see cluster status:

```shell
# show all pods status
kubectl get po --all-namespaces

```

Example of start function:

## Future

- Support multiple cluster
