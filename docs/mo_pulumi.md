# Matrixone-operator pulumi tutorial

- [Install pulumi](https://www.pulumi.com/docs/get-started/install/)
- [AWS config](https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-quickstart.html)

## [Configuration](https://www.pulumi.com/docs/intro/concepts/config/)

- Config install platform

Support eks now

```shell
pulumi config set installPlatform eks
```

- Config instance types

```shell
pulumi config set --path 'instance.types[0]' 't3.medium'
```

- Config disk size

```shell
pulumi config set --path 'instance.diskSize' 50
```

- Install mo cluster

```shell
pulumi config set installMOCluster true
```

- Config zone number (default is all available zones)

```shell
pulumi config set zoneNumber 2
```

- Config region CN for aws-cn policy

```shell
pulumi config set regionCN true
```

- Config [eks public access cidrs](https://docs.aws.amazon.com/eks/latest/userguide/cluster-endpoint.html)

For test using  `0.0.0.0/0`. For security, it should config to a special value.

```shell
pulumi config set --secret publicAccessCidrs "0.0.0.0/0"
```

## Example config on dev pulumi stack 

`Pulumi.dev.yml`

```yaml
encryptionsalt: v1:oqwT/1bFF2I=:v1:KbAlSIvjyEVeeJFo:gnUgAaK31yPskzEhfXzsV+YfIxQjEg==
config:
  operator:installMOCluster: "true"
  operator:installPlatform: eks
  operator:instance:
    diskSize: 50
    types:
    - t3.medium
  operator:publicAccessCidrs:
    secure: v1:l5awFrxTtM/cKyeW:ctSzGYprHLB3KNQY3LtusquWCX2m7nDZWg==
  operator:regionCN: "true"
  operator:zoneNumber: "2"
```

## bootstrap

- [Login to local server](https://www.pulumi.com/docs/intro/concepts/state/#logging-into-the-local-filesystem-backend)

```shell
pulumi login --local
```

- Preview the deploy plan

```shell
pulumi preview
```

- Bootstrap the cluster

```shell
pulumi up --yes
```

## Shutdown the cluster

```shell
pulumi destroy --yes
```

## Export kubeconfig

```shell
pulumi stack output kubeconfig > kubeconfig.yml
```

## Using kubeconfig to access k8s

```shell
export KUBECONFIG=./kubeconfig.yml

kubectl get po --all-namespaces -o wide
```
