# Matrixone Docker Compose

## Matrixone Cluster start up

matrix cluster start with latest image

```shell
# image and tag
export IMAGE=matrixorigin/matrixone TAG=latest

# start cluster
make up

# stop cluster
make down

# clean data and log
make clean
```

## Test Matrixone Cluster with docker compose

Build image and start matrixone cluster

```shell
# clone repo
export BRANCH=main
make dev-pre

# build testing image
make dev-build

# start cluster
make up

# stop cluster
make down

# clean data and log
make clean

# connect mo cluster
export HOST=<YOUR MACHINE IP>
export PORT=<CLUSTER_PORT>
make mo-client

# test other node
export PORT=<OTHER_PORT>
make mo-client
```
