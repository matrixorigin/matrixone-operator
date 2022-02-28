# Get Started with Matrixone Operator in Kubernetes

Prerequisites

- [Docker](https://docs.docker.com/get-docker/)
- [Helm](https://helm.sh/)

## Create a sample Kubernetes cluster

This section describes two ways to create a simple Kubernetes cluster. After creating a Kubernetes cluster, you can use it to test Matrixone cluster managed by Matrixone Operator. Choose whichever best matches your environment.

- Use [kind](https://kind.sigs.k8s.io/) to deploy a Kubernetes cluster in Docker. It is a common and recommended way.
- Use [minikube](https://minikube.sigs.k8s.io/)  to deploy a Kubernetes cluster running locally in a VM.

How to create Kubernetes cluster can see [document](./cluster.md)

MacOS **Kind Example**

```shell
# install kind
brew install kind

# start a kind cluster
kind create cluster --name mo
```

## Deploy Matrixone Operator

[Matrixone Operator helm repository](https://artifacthub.io/packages/helm/matrixone-operator/matrixone-operator)

### Using helm charts

- Install cluster scope operator into the `matrixone-operator` namespace

```shell
# Create namespace
kubectl create ns matrixone-operator

# Add helm repository
helm repo add matrixone https://matrixorigin.github.io/matrixone-operator

# Update repo
helm repo update

# Show helm values about Matrixone Operator
helm show values matrixone/matrixone-operator

# Deploy matrixone operator into matrixone-operator namespace
helm install mo-operator matrixone/matrixone-operator -n matrixone-operator
```

- Check Operator Status

```shell
kubectl get po -n matrixone-operator
```

Matrixone Operator is ready:

```txt
NAME                                              READY   STATUS    RESTARTS   AGE
mo-operator-matrixone-operator-5dd548755f-b7p64   1/1     Running   0          55sx
```

## Deploy a sample Matrixone cluster

- An example spec to deploy a tiny matrixone cluster is included. Install cluster into `matrixone` namespace

```shell
# Create namespace
kubectl create ns matrixone

# Deploy a sample cluster
kubectl apply -f examples/tiny-cluster.yaml -n matrixone
```

- Check Matrixone cluster status

```shell
kubectl get po -n matrixone
```

Matrixone cluster is ready:

```txt
NAME   READY   STATUS    RESTARTS   AGE
mo-0   1/1     Running   0          26s
mo-1   1/1     Running   0          26s
mo-2   1/1     Running   0          26s
```

## Connect to a Matrixone cluster

- Connect cluster by `port-forward`

```shell
# Port-forward 6001 -> 6001
kubectl port-forward service/mo 6001:6001 -n matrixone

# connect to cluster
mysql -h 127.0.0.1 -P 6001 -udump -p111
```

- Connect cluster by [tools](./tools.md)

## Uninstall Matrixone resouces

- Uninstall Matrixone cluster

```shell
kubectl delete -f examples/tiny-cluster.yaml -n matrixone
```

The Matrixone cluster should display the state when the cluster is deleted:

```txt
kubectl get po -n matrixone
> No resources found in matrixone namespace.
```

Then delete namespace:

```shell
# Delete the namespace after all matrixone pods are deleted
kubectl delete ns matrixone
```

- Uninstall Matrixone Operator

```shell
helm uninstall mo-operator -n matrixone-operator
```

The Matrixone Operator should display the state when the cluster is deleted:

```txt
kubectl get po -n matrixone-operator
> No resources found in matrixone-operator namespace.
```

Then delete namespace:

```shell
# Delete the namespace after matrixone operator pod are deleted
kubectl delete ns matrixone-operator
```
