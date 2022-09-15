# MatrixOne Operator Local Evaluation

Prerequisites

- Docker

## Components of image

image based on `docker:22.06-rc-dind`

- awscli
- kubectl
- kind
- helm
- helm repo (operator)
- modev

## Start a matrixone cluster

First, We should pull image from docker hub

```shell
docker pull matrixorigin/matrixone-operator:playground
```

Second, Start playground docker container

```shell
docker run -rm -it -v ~/.aws:/root/.aws  matrixorigin/matrixone-operator:playground /bin/bash
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
	helm repo add https://matrixorigin.github.io/matrixone-operator/charts
	helm install matrixone-operator matrixone-operator/matrixone-operator --version 0.1.0
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
