# Matrixone Operator

Based on [kubebuilder](https://book.kubebuilder.io/)

## Operator develop

Install the CRDs into the cluster

```shell
make install
```

Run your controller (this will run in the foreground, so switch to a new terminal if you want to leave it running):

```shell
make run
```

push operator image to hub

```shell
make docker-build docker-push IMG=<some-registry>/<project-name>:tag
```

deploy operator

```shell
make deploy IMG=<some-registry>/<project-name>:tag
```
