# Matrixone Docker Compose

## Matrixone Cluster start up

matrix cluster start with latest image

```shell
# start cluster
make pro-start

# stop cluster
make pro-down

# clean data and log
make pro-clean
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
make dev-start

# stop cluster
make dev-down

# clean data and log
make dev-clean
```
