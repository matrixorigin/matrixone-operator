# MO K8S Cluster

matrixone cluster deployment document.

## k8s cluster

### kind

```shell
# Install kind on MacOS
brew install kind

# Install kind on Linux 
wget https://github.com/kubernetes-sigs/kind/releases/download/0.2.1/kind-linux-amd64
mv kind-linux-amd64 kind
chmod +x kind
mv kind /usr/local/bin

# start a cluster which named mo
kind create cluster --name mo --config ./kind_config/config.yaml

# delete a cluster 
kind delete cluster --name mo
```

### minikube

```shell
# MacOS
brew install minikube

# Linux
wget https://github.com/kubernetes/minikube/releases/download/v1.24.0/minikube-1.24.0-0.x86_64.rpm
sudo rpm -ivh minikube-1.24.0-0.x86_64.rpm

# start cluster
minikube start

# stop cluster
minikube stop

# minikube dashboard
minikube dashboard
```

### AWS EKS

EKS command tools

```shell
# MacOS
brew install awscli
brew install eksctl

# Linux
# awscli
curl "https://s3.amazonaws.com/aws-cli/awscli-bundle.zip" -o "awscli-bundle.zip"
unzip awscli-bundle.zip
sudo ./awscli-bundle/install -i /usr/local/aws -b /usr/local/bin/aws

# eskctl
curl --silent --location "https://github.com/weaveworks/eksctl/releases/latest/download/eksctl_$(uname -s)_amd64.tar.gz" | tar xz -C /tmp
sudo mv /tmp/eksctl /usr/local/bin
```

aws configuration, path: `~/.aws`

config

```text
[default]
output = json
region = cn-northwest-1
```

credentials

```text
aws_access_key_id = <ACCESS_KEY_ID>
aws_secret_access_key = <SECRET_ACCESS_KEY>
```

start a eks cluster with tree nodes

```shell
# create eks cluster
eksctl create cluster --name mo --version 1.21 --region cn-northwest-1 --nodes 3 --node-type t3.medium --managed
```

## matrixone k8s cluster deploy

### single node start

```shell
# deploy
kubectl create ns matrixone
kubectl apply -f charts -n matrixone
```

### connect matrixone test

default user `dump`,  default password `111`

```shell
kubectl run mysql-client --image=mysql:5.7 -it --rm --restart=Never -- /bin/bash

# then
mysql -h mo.matrixone.svc.cluster.local -P 6001 -udump -p
```

## matrixone docker compose start

[matrixone docker compose tutorial](./docker-compose/README.md)

**reference:**

[docker document](https://docs.docker.com/)

[docker compose document](https://docs.docker.com/compose/)
