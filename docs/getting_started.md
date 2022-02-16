# Get Started with Matrixone Operator in Kubernetes

Prerequisites

- [Docker](https://docs.docker.com/get-docker/)
- [Helm](https://helm.sh/)
- [kustomize](https://kustomize.io/)

## Create a testing Kubernetes cluster

This section describes two ways to create a simple Kubernetes cluster. After creating a Kubernetes cluster, you can use it to test Matrixone clusters managed by Matrixone Operator. Choose whichever best matches your environment.

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

### Pure install

- Register the Matrixone custom resource definition (CRD). Deploy on default namespace

```shell
make deploy
```

- Destroy the Matrixone custom resource definition (CRD).

```shell
make undeploy
```

### Using helm charts

- Install cluster scope operator into the `matrixone-operator` namespace

```shell
# Create namespace
kubectl create ns matrixone-operator

# Install Matrixone Operator using Helm
helm install mo-op charts/matrixone-operator -n matrixone-operator
```

- Uninstall Operator

```shell
helm uninstall mo-op -n matrixone-operator
```

## Deploy a sample Matrixone Cluster

- An example spec to deploy a tiny matrixone cluster is included. Install cluster into `matrixone` namespace

```shell
# Create namespace
kubectl create ns matrixone

# Deploy a sample cluster
kubectl apply -f example/tiny-cluster -n matrixone
```

## Connect to a Matrixone Cluster

- Connect cluster by `port-forward`

```shell
# Port-forward 6001 -> 6001
kubectl port-forward service/mo 6001:6001 -n matrixone

# connect to cluster
mysql -h 127.0.0.1 -P 6001 -udump -p111
```

- Connect cluster by [tools](./tools.md)

## Destroy the Matrixone cluster and the Kubernets cluster

- Destroy Matrixone cluster by `helm` or using `make undeploy`
- [Destroy Kubernetes cluster](./cluster.md)
