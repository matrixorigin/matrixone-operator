# MO K8S Cluster

matrixone cluster deployment document.

## K8S cluster start method

Install tools

pulumi

```shell
# pulumi
brew install pulumi
```

kind

```shell
# kind
brew install kind
```

minikube

```shell
# minikube
brew install minikube
```

### Kind

kind command

```shell
# start cluster
kind create cluster --name mo --config ./kind_config/config.yaml

# stop cluster
kind delete cluster --name mo
```

### Minikube

minikube command

```shell
# start cluster
minikube start

# stop cluster
minikube stop

# minikube dashboard
minikube dashboard
```

### AWS

start aws cluster using pulumi

```shell
# get go package
go mod tidy

# Create a new stack, which is an isolated deployment target for this project
pulumi stack init

# Set the required configuration variables for this program
pulumi confi set aws:region cn-northwest-1

# Stand up the EKS cluster
pulumi up --yes

# After 10-15 minutes, your cluster will be ready, 
# and the kubeconfig JSON you’ll use to connect to the cluster will be available as an output. 
# You can save this kubeconfig to a file like so
pulumi stack output kubeconfig --show-secrets >kubeconfig.json

# Once you have this file in hand, you can interact with your new cluster as usual via kubectl
KUBECONFIG=./kubeconfig.json kubectl get nodes

# Once you’ve finished experimenting, tear down your stack’s resources by destroying and removing it
pulumi destroy --yes
pulumi stack rm --yes
```
