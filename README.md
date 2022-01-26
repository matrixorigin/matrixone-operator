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
make op-build op-push IMG=<some-registry>/<project-name>:tag
```

## Helm deploy Operator

helm install see [website](https://helm.sh/docs/intro/install/)

```shell
kubectl create ns matrixone
helm install mo-op charts/matrixone-operator -n matrixone
```

## Deploy Matrixone Cluster

```shell
kubectl apply -f examples/tiny-cluster.yaml -n matrixone
```
