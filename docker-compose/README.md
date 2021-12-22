# Deploy MatrixOne Cluster with Docker Compose

You can use the following command to start a 3-node MatrixOne Cluster directly which will get MatrixOne from Docker hub automatically:


```shell
# start cluster
make pro-start

# stop cluster
make pro-down

# clean data and log
make pro-clean
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
make dev-start

# stop cluster
make dev-down

# clean data and log
make dev-clean
```
