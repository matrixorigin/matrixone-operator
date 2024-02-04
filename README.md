# MatrixOne Operator

## 安装

### Helm 安装

添加helm仓库

```shell
helm repo add matrixone-operator https://matrixorigin.github.io/matrixone-operator
```

更新仓库

```shell
helm repo update
```

查看MatrixOne Operator版本

```shell
helm search repo matrixone-operator/matrixone-operator --versions --devel
```

安装MatrixOne Operator

```shell
helm install <RELEASE_NAME> matrixone-operator/matrixone-operator --version <VERSION>
```

### Helm 部署集群

查看 MatrixOne Chart 版本
```
> helm search repo matrixone-operator/matrixone --devel
NAME                                 	CHART VERSION	APP VERSION	DESCRIPTION
matrixone-operator/matrixone         	0.1.0        	1.16.0     	A Helm chart to deploy MatrixOne on K8S
matrixone-operator/matrixone-operator	1.1.0-alpha1 	0.1.0      	Matrixone Kubernetes Operator
```

安装 MatrixOne Chart（这会部署一个 MatrixOneCluster 对象）

```
安装MatrixOne Operator

```shell
helm install -n <NS> <RELEASE_NAME> matrixone-operator/matrixone --version <VERSION> -f values.yaml
```

values.yaml 的格式和 mo-cluster.yaml 相同, 可以参考默认的 values.yaml: https://github.com/matrixorigin/matrixone-operator/blob/main/charts/matrixone/values.yaml

开启 reusePVC:
```yaml
...
  cnGroups:
    - reusePVC: true
```
