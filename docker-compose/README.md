# Deploy MatrixOne Cluster with Docker Compose

You can use the following command to start a 3-node MatrixOne Cluster directly which will get MatrixOne from Docker hub automatically:


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

You can also use following command to build a MatrixOne image directly:
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
