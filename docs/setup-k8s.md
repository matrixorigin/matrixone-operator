# Create a Kubernetes cluster

## Local

```shell
make up
```

## AWS EKS

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

# destroy eks cluster
eksctl delete cluster --name mo
```

## Tencent cloud

see more on [Tencent Cloud guide](./tencentcloud/tencent.md)
