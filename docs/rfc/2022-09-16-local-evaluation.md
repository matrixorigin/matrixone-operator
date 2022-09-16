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
- modev

## Start a matrixone cluster

First, We should pull image from docker hub

```shell
docker pull matrixorigin/matrixone-operator:playground
```

Second, Start playground docker container

```shell
docker run -d --name playground --privileged -it -v ~/.aws:/root/.aws  matrixorigin/matrixone-operator:playground
docker exec -it playground /bin/sh
```

Then, Start a matrixone cluster 

```shell
modev start
```

Example of start function:

```shell
function start() {
	kind create cluster
	kubectl create ns matrixone
	kubectl create ns matrixone-operator
	helm install matrixone-operator matrixone-operator/matrixone-operator --version 0.1.0 -n matrixone-operator
	kubectl apply -f https://raw.githubusercontent.com/wanglei4687/matrixone-operator/main/examples/mo-cluster.yaml -n matrixone
	
}
```

## Stop a matrixone cluster

```shell
modev stop
```

Example of stop function:

```shell
function stop() {
	kind delete cluster
}
```

## Future

- Support multiple cluster
