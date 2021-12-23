# Deploy MatrixOne Cluster with Docker Compose

You can use the following command to start a 3-node MatrixOne Cluster directly which will get MatrixOne from Docker hub automatically:

## Matrixone Cluster start up

matrix cluster start with latest image

### connect mo-cluster

```shell
# connect mo cluster
export HOST=<YOUR MACHINE IP> PORT=<CLUSTER_PORT>
make mo-client
```

### start wtih custom image

```shell
# image and tag
export IMAGE=<IMAGE> TAG=<TAG>

# start cluster
make up

# stop cluster
make down

# clean data and log
make clean
```

### start with latest image

```shell

# start cluster
make prod-up

# stop cluster
make down

# clean
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
make dev-up

# stop cluster
make down

# clean data and log
make clean

# clean data, log and repo
make all-clean
```
