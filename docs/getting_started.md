# Get Started with matrixone-operator

`matrixone-operator` helps you manage MatrixOne(MO) clusters in Kubernetes(k8s). This document will guide you install `matirxone-operator` on your k8s and manage MO clusters using the operator.

Prerequisites:

- Kubernetes 1.18 or later;
   - or [Docker](https://docs.docker.com/engine/install/) and [Kind](https://kind.sigs.k8s.io/docs/user/quick-start/#installation) if you do not have an existing k8s cluster available;
- [kubectl 1.18 or later](https://kubernetes.io/docs/tasks/tools/);
- [helm 3](https://helm.sh/docs/intro/install/).

## Prepare Your Kubernetes Cluster

Verify the cluster meets the minimal version requirement, refer to [kubernetes setup](setup-k8s.md) to setup a kubernetes cluster if you don't have one:

```bash
> kubectl version
```

The server version should >= 1.18.

## Install MatrixOne Operator

```bash
helm repo add matrixone-operator https://matrixorigin.github.io/matrixone-operator
helm repo update
helm install mo matrixone-operator/matrixone-operator --version=1.0.0-alpha.1
```

## Deploy MatrixOne Cluster

1. MO cluster requires an external shared storage like S3 and MinIO. You can deploy a minial MinIO for test purpose if you do not have an available shared storage:

    ```bash
    kubectl apply -f https://raw.githubusercontent.com/matrixorigin/matrixone-operator/main/examples/minio.yaml
    ```

2. Create a namespace to deploy your MO cluster:

    ```bash
    NS=mo
    kubectl create ns ${NS}
    ```

3. Create a secret in `mo` namespace to access the MinIO instance deployed at step 1:

    ```bash
    # if you change the default AK/SK in the yaml spec at step 1, change it in this step accordingly
    kubectl -n ${NS} create secret generic minio --from-literal=AWS_ACCESS_KEY_ID=minio --from-literal=AWS_SECRET_ACCESS_KEY=minio123
    ```

4. Create a YAML spec of your MO cluster, edit the fields to match your requirment:

    ```bash
    curl https://raw.githubusercontent.com/matrixorigin/matrixone-operator/main/examples/mo-cluster.yaml | sed 's/#TAG/1.0.0-rc1/g' > mo.yaml
    # edit mo.yaml to match your environment, if you're exactly following this guide so far, no change is required
    vim mo.yaml
    # create the cluster
    kubectl -n ${NS} apply -f mo.yaml
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

## Backup and Restore

Create a `BackupJob` to backup your cluster

```bash
# change this variable if you change the cluster name in YAML
CLUSTER_NAME=mo
curl https://raw.githubusercontent.com/matrixorigin/matrixone-operator/main/examples/backupjob.yaml | sed "s/#SourceClusterName/$CLUSTER_NAME/g" > backupjob.yaml
# inspect the backupjob spec   
cat backupjob.yaml
# apply the job if the looks good
kubectl -n ${NS} apply -f backupjob.yaml
# get job progress
kubectl -n ${NS} get backupjob
# list all backups in the cluster
kubectl -n ${NS} get backup
```

Then you have two options to use the backup:

- Option 1: Create a new cluster to launch from the backup, mo-operator will automatically restore the backup to the object storage location the cluster want to use and then launch the cluster for you:

```bash
BACKUP_NAME=<the backup you want to use, you can query available backups using "kubectl get backup">
curl https://raw.githubusercontent.com/matrixorigin/matrixone-operator/main/examples/mo-from-backup.yaml | sed 's/#TAG/1.0.0-alpha.1/g' | sed "s/#BackupName/$BACKUP_NAME/g" > new-mo.yaml
# inspect the cluster spec
cat new-mo.yaml
# create the new cluster
kubectl apply -f new-mo.yaml
```

- Option 2: Restore the backup to a new location and manually launch a cluster using the new data:

```bash
BACKUP_NAME=<the backup you want to use, you can query available backups using "kubectl get backup">
curl https://raw.githubusercontent.com/matrixorigin/matrixone-operator/main/examples/restorejob.yaml | sed "s/#BackupName/$BACKUP_NAME/g" > restorejob.yaml
# inspect the restorejob
cat restorejob.yaml
# start restore data
kubectl create -f restorejob.yaml
# get restore progress
kubectl get restorejob
# after restore complete, create a cluster to start form the restored data dir
```

## Teardown MO cluster

To teardown the cluster, delete the mo object in k8s:

```bash
> kubectl -n ${NS} delete matrixonecluster mo
matrixonecluster.core.matrixorigin.io "mo" deleted
```
