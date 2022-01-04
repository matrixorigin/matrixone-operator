# Matrixone Cluster with docker compose

## Build

1. Make sure `docker` and `docker compose` is installed.
2. Use `make` to start/delete a 3-node MatrixOne cluster

## Usage

### Start a 3-node cluster with default configuration

You can use the following command to start a MatrixOne cluster using the official MatrixOne image (which will be automatically downloaded from the docker hub).

```shell
# Start cluster using the latest image.
make up

# Or start cluster using a specific image by specifying `tag`.
make up TAG=<TAG>
```

You can also build a MatrixOne Image from the source directly.

```shell
# Clone MatrixOne into the current directory, default branch is `main`.
make dev-pre

# You can also specify a branch name when executing the following command.
make dev-pre BRANCH=<BRANCH>
```

### run mo-client to connect the cluster

```shell
# connect mo cluster
make mo-client HOST=<YOUR MACHINE IP> PORT=<CLUSTER_PORT>
```

### Close cluster

```shell
make down
```

### Clear residual data

```shell
make clean
```
