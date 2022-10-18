# Get Started with matrixone-operator

`matrixone-operator` helps you manage MatrixOne(MO) clusters in Kubernetes(k8s). This document will guide you install `matirxone-operator` on your k8s and manage MO clusters using the operator.

Prerequisites:

- Kubernetes 1.18 or later;
   - or [Docker](https://docs.docker.com/engine/install/) and [Kind](https://kind.sigs.k8s.io/docs/user/quick-start/#installation) if you do not have an existing k8s cluster available;
- [kubectl 1.18 or later](https://kubernetes.io/docs/tasks/tools/);
- [helm 3](https://helm.sh/docs/intro/install/).

## Prepare Your Kubernetes Cluster

Verify the cluster meets the minimal version requirement:

```bash
> kubectl version
```

The server version should >= 1.18.
If you do not have existing k8s installed or your cluster does not meet the version requirement, you use `kind` to create a test one locally (follow the [offical installation guide](https://kind.sigs.k8s.io/docs/user/quick-start/#installation) to install `kind` if you do not have kind CLI available):

```bash
kind create cluster --name mo
```

For advanced usage, you can customize you kind cluster by following [this guide](https://kind.sigs.k8s.io/docs/user/configuration/), e.g. version and node count.

## Install MatrixOne Operator

1. Clone the `matrixone-operator` git repo:

```bash
> git clone https://github.com/matrixorigin/matrixone-operator.git
```

2. Create a namespace for `matrixone-operator`:

```bash
> kubectl create namespace mo-system
```

3. Install matrixone-operator with `helm`:

```bash
> helm -n mo-system install mo ./charts/matrixone-operator --dependency-update
```

4. Verify the installation:

```bash
> helm -n mo-system ls
> kubectl -n mo-system get po
```

You should see at least one ready `matrixone-operator` Pod.

## Deploy MatrixOne Cluster

1. MO cluster requires an external shared storage like S3 and MinIO. You can deploy a minial MinIO for test purpose if you do not have an available shared storage:

    ```bash
    > kubectl -n mo-system apply -f https://raw.githubusercontent.com/matrixorigin/matrixone-operator/main/examples/minio.yaml
    ```

2. Create a namespace to deploy your MO cluster:

    ```bash
    > kubectl create ns mo
    ```

3. Create a secret in `mo` namespace to access the MinIO instance deployed at step 1:

    ```bash
    # if you change the default AK/SK in the yaml spec at step 1, change it in this step accordingly
    > kubectl -n mo create secret generic minio --from-literal=AWS_ACCESS_KEY_ID=minio --from-literal=AWS_SECRET_ACCESS_KEY=minio123
    ```

4. Create a YAML spec of your MO cluster, edit the fields to match your requirment:

    ```bash
    > cat>mo.yaml<<EOF
    apiVersion: core.matrixorigin.io/v1alpha1
    kind: MatrixOneCluster
    metadata:
      name: mo
    spec:
      imageRepository: matrixorigin/matrixone
      # Version of the MO to deploy
      version: nightly-c371317c
      logService:
        replicas: 3
        sharedStorage:
          s3:
            path: matrixone
            # endpoint and secretRef are used to access the MinIO instance deployed at step 1
            endpoint: minio-0.mo-system:9000
            secretRef:
              name: minio
        volume:
          size: 10Gi
      dn:
        replicas: 2
        cacheVolume:
          size: 10Gi
      tp:
        replicas: 2
        cacheVolume:
          size: 10Gi
    EOF
    # Create the cluster
    > kubectl -n mo apply -f mo.yaml
    ```

5. Wait your cluster to become ready:

    ```bash
    > kubectl -n mo get matrixonecluster
    > kubectl -n mo get pod
    ```

6. Get the credential of the cluster from the cluster status:

   ```bash
   # get the secret name that contains the initial credential
   > SECRET_NAME=$(kubectl -n mo get matrixonecluster mo --template='{{.status.credentialRef.name}}')
   # get the 
   > USERNAME=$(kubectl -n mo get secret ${SECRET_NAME} --template='{{.data.username}}' | base64 -d)
   > PASSWORD=$(kubectl -n mo get secret ${SECRET_NAME} --template='{{.data.password}}' | base64 -d)
   ```
   
7. After there are CN pods running, you can access the cluster via the CN service:

    ```bash
    > nohup kubectl -n mo port-forward svc/mo-tp-cn 6001:6001 &
    > mysql -h 127.0.0.1 -P6001 -u${USERNAME} -p${PASSWORD}
    ```
   

## Teardown MO cluster

To teardown the cluster, delete the mo object in k8s:

```bash
> kubectl -n mo get matrixonecluster
NAME   LOG   DN    TP    AP    UI    VERSION            AGE
mo     3     2     2                 nightly-c371317c   13m
> kubectl -n mo delete matrixonecluster mo
matrixonecluster.core.matrixorigin.io "mo" deleted
```
