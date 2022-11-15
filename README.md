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
