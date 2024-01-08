# API Reference

## Packages
- [core.matrixorigin.io/v1alpha1](#corematrixoriginiov1alpha1)


## core.matrixorigin.io/v1alpha1

Package v1alpha1 contains API Schema definitions of the matrixone v1alpha1 API group.
The MatrixOneCluster resource type helps you provision and manage MatrixOne clusters on Kubernetes.
Other resources types represent sub-resources managed by MatrixOneCluster but also allows separated usage for fine-grained control.
Refer to https://docs.matrixorigin.io/ for more information about MatrixOne database and matrixone-operator.

### Resource Types
- [Backup](#backup)
- [BackupJob](#backupjob)
- [BackupJobList](#backupjoblist)
- [BackupList](#backuplist)
- [BucketClaim](#bucketclaim)
- [BucketClaimList](#bucketclaimlist)
- [CNClaim](#cnclaim)
- [CNClaimList](#cnclaimlist)
- [CNClaimSet](#cnclaimset)
- [CNClaimSetList](#cnclaimsetlist)
- [CNPool](#cnpool)
- [CNPoolList](#cnpoollist)
- [CNSet](#cnset)
- [DNSet](#dnset)
- [LogSet](#logset)
- [MatrixOneCluster](#matrixonecluster)
- [ProxySet](#proxyset)
- [ProxySetList](#proxysetlist)
- [RestoreJob](#restorejob)
- [RestoreJobList](#restorejoblist)
- [WebUI](#webui)



#### Backup



A Backup is a resource that represents an MO physical backup

_Appears in:_
- [BackupList](#backuplist)

| Field | Description |
| --- | --- |
| `apiVersion` _string_ | `core.matrixorigin.io/v1alpha1`
| `kind` _string_ | `Backup`
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |
| `meta` _[BackupMeta](#backupmeta)_ | Meta is the backupMeta |


#### BackupJob



A BackupJob is a resource that represents an MO backup job

_Appears in:_
- [BackupJobList](#backupjoblist)

| Field | Description |
| --- | --- |
| `apiVersion` _string_ | `core.matrixorigin.io/v1alpha1`
| `kind` _string_ | `BackupJob`
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |
| `spec` _[BackupJobSpec](#backupjobspec)_ | Spec is the backupJobSpec |


#### BackupJobList



BackupJobList contains a list of BackupJob



| Field | Description |
| --- | --- |
| `apiVersion` _string_ | `core.matrixorigin.io/v1alpha1`
| `kind` _string_ | `BackupJobList`
| `metadata` _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#listmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |
| `items` _[BackupJob](#backupjob) array_ |  |


#### BackupJobSpec



BackupJobSpec specifies the backup job

_Appears in:_
- [BackupJob](#backupjob)

| Field | Description |
| --- | --- |
| `ttl` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#duration-v1-meta)_ | ttl defines the time to live of the backup job after completed or failed |
| `source` _[BackupSource](#backupsource)_ | source the backup source |
| `target` _[SharedStorageProvider](#sharedstorageprovider)_ |  |
| `overlay` _[Overlay](#overlay)_ |  |




#### BackupList



BackupList contains a list of BackupJ



| Field | Description |
| --- | --- |
| `apiVersion` _string_ | `core.matrixorigin.io/v1alpha1`
| `kind` _string_ | `BackupList`
| `metadata` _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#listmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |
| `items` _[Backup](#backup) array_ |  |


#### BackupMeta



BackupMeta specifies the backup

_Appears in:_
- [Backup](#backup)

| Field | Description |
| --- | --- |
| `location` _[SharedStorageProvider](#sharedstorageprovider)_ | location is the data location of the backup |
| `id` _string_ | id uniquely identifies the backup |
| `size` _Quantity_ | size is the backup data size |
| `atTime` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#time-v1-meta)_ | atTime is the backup start time |
| `completeTime` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#time-v1-meta)_ | completeTime the backup complete time |
| `sourceRef` _string_ | clusterRef is the reference to the cluster that produce this backup |
| `raw` _string_ |  |


#### BackupSource



BackupSource is the source of the backup job

_Appears in:_
- [BackupJobSpec](#backupjobspec)

| Field | Description |
| --- | --- |
| `clusterRef` _string_ | clusterRef is the name of the cluster to back up, mutual exclusive with cnSetRef |
| `cnSetRef` _string_ | cnSetRef is the name of the cnSet to back up, mutual exclusive with clusterRef |
| `secretRef` _string_ | optional, secretRef is the name of the secret to use for authentication |


#### BucketClaim



A BucketClaim is a resource that represents the object storage bucket resource used by a mo cluster

_Appears in:_
- [BucketClaimList](#bucketclaimlist)

| Field | Description |
| --- | --- |
| `apiVersion` _string_ | `core.matrixorigin.io/v1alpha1`
| `kind` _string_ | `BucketClaim`
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |
| `spec` _[BucketClaimSpec](#bucketclaimspec)_ | Spec is the desired state of BucketClaim |


#### BucketClaimList



BucketClaimList contains a list of BucketClaim



| Field | Description |
| --- | --- |
| `apiVersion` _string_ | `core.matrixorigin.io/v1alpha1`
| `kind` _string_ | `BucketClaimList`
| `metadata` _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#listmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |
| `items` _[BucketClaim](#bucketclaim) array_ |  |


#### BucketClaimSpec





_Appears in:_
- [BucketClaim](#bucketclaim)

| Field | Description |
| --- | --- |
| `s3` _[S3Provider](#s3provider)_ | S3 specifies an S3 bucket as the shared storage provider, mutual-exclusive with other providers. |
| `logSetSpec` _[PodTemplateSpec](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#podtemplatespec-v1-core)_ | LogSetTemplate is a complete copy version of kruise statefulset PodTemplateSpec |




#### CNClaim



CNClaim claim a CN to use

_Appears in:_
- [CNClaimList](#cnclaimlist)

| Field | Description |
| --- | --- |
| `apiVersion` _string_ | `core.matrixorigin.io/v1alpha1`
| `kind` _string_ | `CNClaim`
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |
| `spec` _[CNClaimSpec](#cnclaimspec)_ |  |


#### CNClaimList



CNClaimList contains a list of CNClaims



| Field | Description |
| --- | --- |
| `apiVersion` _string_ | `core.matrixorigin.io/v1alpha1`
| `kind` _string_ | `CNClaimList`
| `metadata` _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#listmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |
| `items` _[CNClaim](#cnclaim) array_ |  |


#### CNClaimSet



CNClaimSet orchestrates a set of CNClaims

_Appears in:_
- [CNClaimSetList](#cnclaimsetlist)

| Field | Description |
| --- | --- |
| `apiVersion` _string_ | `core.matrixorigin.io/v1alpha1`
| `kind` _string_ | `CNClaimSet`
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |
| `spec` _[CNClaimSetSpec](#cnclaimsetspec)_ |  |


#### CNClaimSetList



CNClaimSetList contains a list of CNClaimSet



| Field | Description |
| --- | --- |
| `apiVersion` _string_ | `core.matrixorigin.io/v1alpha1`
| `kind` _string_ | `CNClaimSetList`
| `metadata` _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#listmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |
| `items` _[CNClaimSet](#cnclaimset) array_ |  |


#### CNClaimSetSpec





_Appears in:_
- [CNClaimSet](#cnclaimset)

| Field | Description |
| --- | --- |
| `replicas` _integer_ |  |
| `template` _[CNClaimTemplate](#cnclaimtemplate)_ |  |
| `selector` _[LabelSelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#labelselector-v1-meta)_ |  |




#### CNClaimSpec





_Appears in:_
- [CNClaim](#cnclaim)
- [CNClaimTemplate](#cnclaimtemplate)

| Field | Description |
| --- | --- |
| `selector` _[LabelSelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#labelselector-v1-meta)_ |  |
| `cnLabels` _[CNLabel](#cnlabel) array_ |  |
| `ownerName` _string_ |  |
| `podName` _string_ | PodName is usually populated by controller and would be part of the claim spec that must be persisted once bound |
| `poolName` _string_ | PoolName is usually populated by controller that which pool the claim is nominated |


#### CNClaimStatus





_Appears in:_
- [CNClaimSetStatus](#cnclaimsetstatus)

| Field | Description |
| --- | --- |
| `phase` _CNClaimPhase_ |  |
| `store` _[CNStoreStatus](#cnstorestatus)_ |  |
| `boundTime` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#time-v1-meta)_ |  |


#### CNClaimTemplate





_Appears in:_
- [CNClaimSetSpec](#cnclaimsetspec)

| Field | Description |
| --- | --- |
| `metadata` _[EmbeddedMetadata](#embeddedmetadata)_ | Refer to Kubernetes API documentation for fields of `metadata`. |
| `spec` _[CNClaimSpec](#cnclaimspec)_ |  |


#### CNGroup





_Appears in:_
- [MatrixOneClusterSpec](#matrixoneclusterspec)

| Field | Description |
| --- | --- |
| `CNSetSpec` _[CNSetSpec](#cnsetspec)_ |  |
| `name` _string_ | Name is the CNGroup name, an error will be raised if duplicated name is found in a mo cluster |


#### CNGroupStatus





_Appears in:_
- [CNGroupsStatus](#cngroupsstatus)

| Field | Description |
| --- | --- |
| `name` _string_ |  |
| `host` _string_ |  |
| `ready` _boolean_ |  |
| `synced` _boolean_ |  |




#### CNLabel





_Appears in:_
- [CNClaimSpec](#cnclaimspec)
- [CNSetSpec](#cnsetspec)
- [CNStoreStatus](#cnstorestatus)

| Field | Description |
| --- | --- |
| `key` _string_ | Key is the store label key |
| `values` _string array_ | Values are the store label values |


#### CNPool



CNPool maintains a pool of CN Pods

_Appears in:_
- [CNPoolList](#cnpoollist)

| Field | Description |
| --- | --- |
| `apiVersion` _string_ | `core.matrixorigin.io/v1alpha1`
| `kind` _string_ | `CNPool`
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |
| `spec` _[CNPoolSpec](#cnpoolspec)_ |  |


#### CNPoolList



CNPoolList contains a list of CNPool



| Field | Description |
| --- | --- |
| `apiVersion` _string_ | `core.matrixorigin.io/v1alpha1`
| `kind` _string_ | `CNPoolList`
| `metadata` _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#listmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |
| `items` _[CNPool](#cnpool) array_ |  |


#### CNPoolSpec





_Appears in:_
- [CNPool](#cnpool)

| Field | Description |
| --- | --- |
| `template` _[CNSetSpec](#cnsetspec)_ | Template is the CNSet template of the Pool |
| `podLabels` _object (keys:string, values:string)_ | PodLabels is the Pod labels of the CN in Pool |
| `deps` _[CNSetDeps](#cnsetdeps)_ | Deps is the dependencies of the Pool |
| `strategy` _[PoolStrategy](#poolstrategy)_ |  |




#### CNRole

_Underlying type:_ `string`



_Appears in:_
- [CNSetSpec](#cnsetspec)



#### CNSet



A CNSet is a resource that represents a set of MO's CN instances

_Appears in:_
- [WebUIDeps](#webuideps)

| Field | Description |
| --- | --- |
| `apiVersion` _string_ | `core.matrixorigin.io/v1alpha1`
| `kind` _string_ | `CNSet`
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |
| `spec` _[CNSetSpec](#cnsetspec)_ | Spec is the desired state of CNSet |
| `deps` _[CNSetDeps](#cnsetdeps)_ | Deps is the dependencies of CNSet |


#### CNSetDeps





_Appears in:_
- [CNPoolSpec](#cnpoolspec)
- [CNSet](#cnset)

| Field | Description |
| --- | --- |
| `LogSetRef` _[LogSetRef](#logsetref)_ |  |
| `dnSet` _[DNSet](#dnset)_ | The DNSet it depends on |


#### CNSetSpec





_Appears in:_
- [CNGroup](#cngroup)
- [CNPoolSpec](#cnpoolspec)
- [CNSet](#cnset)
- [MatrixOneClusterSpec](#matrixoneclusterspec)

| Field | Description |
| --- | --- |
| `PodSet` _[PodSet](#podset)_ |  |
| `serviceType` _[ServiceType](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#servicetype-v1-core)_ | ServiceType is the service type of cn service |
| `serviceAnnotations` _object (keys:string, values:string)_ | ServiceAnnotations are the annotations for the cn service |
| `nodePort` _integer_ | NodePort specifies the node port to use when ServiceType is NodePort or LoadBalancer, reconciling will fail if the node port is not available. |
| `cacheVolume` _[Volume](#volume)_ | CacheVolume is the desired local cache volume for CNSet, node storage will be used if not specified |
| `sharedStorageCache` _[SharedStorageCache](#sharedstoragecache)_ | SharedStorageCache is the configuration of the S3 sharedStorageCache |
| `role` _[CNRole](#cnrole)_ | [TP, AP], default to TP Deprecated: use labels instead |
| `cnLabels` _[CNLabel](#cnlabel) array_ | Labels are the CN labels for all the CN stores managed by this CNSet |
| `scalingConfig` _[ScalingConfig](#scalingconfig)_ | ScalingConfig declares the CN scaling behavior |
| `metricsSecretRef` _[ObjectRef](#objectref)_ | MetricsSecretRef is the secret reference for the operator to access CN metrics |
| `updateStrategy` _[RollingUpdateStrategy](#rollingupdatestrategy)_ | UpdateStrategy is the rolling-update strategy of CN |
| `pythonUdfSidecar` _[PythonUdfSidecar](#pythonudfsidecar)_ | PythonUdfSidecar is the python udf server in CN |
| `podManagementPolicy` _string_ | PodManagementPolicy is the pod management policy of the Pod in this Set |
| `podsToDelete` _string array_ | PodsToDelete are the Pods to delete in the CNSet |
| `pauseUpdate` _boolean_ | PauseUpdate means the CNSet should pause rolling-update |




#### CNStoreStatus





_Appears in:_
- [CNClaimStatus](#cnclaimstatus)

| Field | Description |
| --- | --- |
| `serviceID` _string_ |  |
| `lockServiceAddress` _string_ |  |
| `pipelineServiceAddress` _string_ |  |
| `sqlAddress` _string_ |  |
| `queryAddress` _string_ |  |
| `workState` _integer_ |  |
| `labels` _[CNLabel](#cnlabel) array_ |  |


#### CertificateRef





_Appears in:_
- [S3Provider](#s3provider)

| Field | Description |
| --- | --- |
| `name` _string_ | secret name |
| `files` _string array_ | cert files in the secret |




#### ConditionalStatus





_Appears in:_
- [BackupJobStatus](#backupjobstatus)
- [BucketClaimStatus](#bucketclaimstatus)
- [ProxySetStatus](#proxysetstatus)
- [RestoreJobStatus](#restorejobstatus)

| Field | Description |
| --- | --- |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#condition-v1-meta) array_ |  |


#### DNSet



A DNSet is a resource that represents a set of MO's DN instances

_Appears in:_
- [CNSetDeps](#cnsetdeps)

| Field | Description |
| --- | --- |
| `apiVersion` _string_ | `core.matrixorigin.io/v1alpha1`
| `kind` _string_ | `DNSet`
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |
| `spec` _[DNSetSpec](#dnsetspec)_ | Spec is the desired state of DNSet |
| `deps` _[DNSetDeps](#dnsetdeps)_ | Deps is the dependencies of DNSet |


#### DNSetDeps





_Appears in:_
- [DNSet](#dnset)

| Field | Description |
| --- | --- |
| `LogSetRef` _[LogSetRef](#logsetref)_ |  |


#### DNSetSpec





_Appears in:_
- [DNSet](#dnset)
- [MatrixOneClusterSpec](#matrixoneclusterspec)

| Field | Description |
| --- | --- |
| `PodSet` _[PodSet](#podset)_ |  |
| `cacheVolume` _[Volume](#volume)_ | CacheVolume is the desired local cache volume for DNSet, node storage will be used if not specified |
| `sharedStorageCache` _[SharedStorageCache](#sharedstoragecache)_ |  |




#### EmbeddedMetadata





_Appears in:_
- [CNClaimTemplate](#cnclaimtemplate)

| Field | Description |
| --- | --- |
| `name` _string_ |  |
| `labels` _object (keys:string, values:string)_ |  |
| `annotations` _object (keys:string, values:string)_ |  |


#### ExternalLogSet





_Appears in:_
- [LogSetRef](#logsetref)

| Field | Description |
| --- | --- |
| `haKeeperEndpoint` _string_ | HAKeeperEndpoint of the ExternalLogSet |


#### FailedPodStrategy

_Underlying type:_ `string`



_Appears in:_
- [LogSetSpec](#logsetspec)





#### FileSystemProvider





_Appears in:_
- [SharedStorageProvider](#sharedstorageprovider)

| Field | Description |
| --- | --- |
| `path` _string_ | Path the path that the shared fileSystem mounted to |


#### InitialConfig





_Appears in:_
- [LogSetSpec](#logsetspec)

| Field | Description |
| --- | --- |
| `logShards` _[int](#int)_ | LogShards is the initial number of log shards, cannot be tuned after cluster creation currently. default to 1 |
| `dnShards` _[int](#int)_ | DNShards is the initial number of DN shards, cannot be tuned after cluster creation currently. default to 1 |
| `logShardReplicas` _[int](#int)_ | LogShardReplicas is the replica numbers of each log shard, cannot be tuned after cluster creation currently. default to 3 if LogSet replicas >= 3, to 1 otherwise |
| `restoreFrom` _[string](#string)_ | RestoreFrom declares the HAKeeper data should be restored from the given path when hakeeper is bootstrapped |


#### LogSet



A LogSet is a resource that represents a set of MO's LogService instances

_Appears in:_
- [LogSetRef](#logsetref)

| Field | Description |
| --- | --- |
| `apiVersion` _string_ | `core.matrixorigin.io/v1alpha1`
| `kind` _string_ | `LogSet`
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |
| `spec` _[LogSetSpec](#logsetspec)_ | Spec is the desired state of LogSet |




#### LogSetRef



LogSetRef reference to an LogSet, either internal or external

_Appears in:_
- [CNSetDeps](#cnsetdeps)
- [DNSetDeps](#dnsetdeps)
- [ProxySetDeps](#proxysetdeps)

| Field | Description |
| --- | --- |
| `logSet` _[LogSet](#logset)_ | The LogSet it depends on, mutual exclusive with ExternalLogSet |
| `externalLogSet` _[ExternalLogSet](#externallogset)_ | An external LogSet the CNSet should connected to, mutual exclusive with LogSet TODO: rethink the schema of ExternalLogSet |


#### LogSetSpec





_Appears in:_
- [LogSet](#logset)
- [MatrixOneClusterSpec](#matrixoneclusterspec)

| Field | Description |
| --- | --- |
| `PodSet` _[PodSet](#podset)_ |  |
| `volume` _[Volume](#volume)_ | Volume is the local persistent volume for each LogService instance |
| `sharedStorage` _[SharedStorageProvider](#sharedstorageprovider)_ | SharedStorage is an external shared storage shared by all LogService instances |
| `initialConfig` _[InitialConfig](#initialconfig)_ | InitialConfig is the initial configuration of HAKeeper InitialConfig is immutable |
| `storeFailureTimeout` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#duration-v1-meta)_ | StoreFailureTimeout is the timeout to fail-over the logset Pod after a failure of it is observed |
| `failedPodStrategy` _[FailedPodStrategy](#failedpodstrategy)_ | FailedPodStrategy controls how to handle failed pod when failover happens, default to Delete |
| `pvcRetentionPolicy` _[PVCRetentionPolicy](#pvcretentionpolicy)_ | PVCRetentionPolicy defines the retention policy of orphaned PVCs due to cluster deletion, scale-in or failover. Available options: - Delete: delete orphaned PVCs - Retain: keep orphaned PVCs, if the corresponding Pod get created again (e.g. scale-in and scale-out, recreate the cluster), the Pod will reuse the retained PVC which contains previous data. Retained PVCs require manual cleanup if they are no longer needed. The default policy is Delete. |


#### MainContainer



MainContainer is the description of the main container of a Pod

_Appears in:_
- [PodSet](#podset)

| Field | Description |
| --- | --- |
| `image` _string_ | Image is the docker image of the main container |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#resourcerequirements-v1-core)_ | Resources is the resource requirement of the main conainer |


#### MainContainerOverlay





_Appears in:_
- [Overlay](#overlay)
- [PythonUdfSidecar](#pythonudfsidecar)

| Field | Description |
| --- | --- |
| `command` _string array_ |  |
| `args` _string array_ |  |
| `envFrom` _[EnvFromSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#envfromsource-v1-core) array_ |  |
| `env` _[EnvVar](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#envvar-v1-core) array_ |  |
| `imagePullPolicy` _[PullPolicy](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#pullpolicy-v1-core)_ | ImagePullPolicy is the pull policy of MatrixOne image. The default value is the same as the default of Kubernetes. |
| `volumeMounts` _[VolumeMount](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#volumemount-v1-core) array_ |  |
| `livenessProbe` _[Probe](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#probe-v1-core)_ |  |
| `readinessProbe` _[Probe](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#probe-v1-core)_ |  |
| `startupProbe` _[Probe](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#probe-v1-core)_ |  |
| `lifecycle` _[Lifecycle](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#lifecycle-v1-core)_ |  |


#### MatrixOneCluster



A MatrixOneCluster is a resource that represents a MatrixOne Cluster



| Field | Description |
| --- | --- |
| `apiVersion` _string_ | `core.matrixorigin.io/v1alpha1`
| `kind` _string_ | `MatrixOneCluster`
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |
| `spec` _[MatrixOneClusterSpec](#matrixoneclusterspec)_ | Spec is the desired state of MatrixOneCluster |


#### MatrixOneClusterSpec



MatrixOneClusterSpec defines the desired state of MatrixOneCluster Note that MatrixOneCluster does not support specify overlay for underlying sets directly due to the size limitation of kubernetes apiserver

_Appears in:_
- [MatrixOneCluster](#matrixonecluster)

| Field | Description |
| --- | --- |
| `tp` _[CNSetSpec](#cnsetspec)_ | TP is the default CN pod set that accepts client connections and execute queries Deprecated: use cnGroups instead |
| `ap` _[CNSetSpec](#cnsetspec)_ | AP is an optional CN pod set that accept MPP sub-plans to accelerate sql queries Deprecated: use cnGroups instead |
| `cnGroups` _[CNGroup](#cngroup) array_ | CNGroups are CN pod sets that have different spec like resources, arch, store labels |
| `dn` _[DNSetSpec](#dnsetspec)_ | DN is the default DN pod set of this Cluster Deprecated: use TN instead |
| `tn` _[DNSetSpec](#dnsetspec)_ | TN is the default TN pod set of this Cluster |
| `logService` _[LogSetSpec](#logsetspec)_ | LogService is the default LogService pod set of this cluster |
| `webui` _[WebUISpec](#webuispec)_ | WebUI is the default web ui pod of this cluster |
| `proxy` _[ProxySetSpec](#proxysetspec)_ | Proxy defines an optional MO Proxy of this cluster |
| `version` _string_ | Version is the version of the cluster, which translated to the docker image tag used for each component. default to the recommended version of the operator |
| `imageRepository` _string_ | ImageRepository allows user to override the default image repository in order to use a docker registry proxy or private registry. |
| `topologySpread` _string array_ | TopologyEvenSpread specifies default topology policy for all components, this will be overridden by component-level config |
| `nodeSelector` _object (keys:string, values:string)_ | NodeSelector specifies default node selector for all components, this will be overridden by component-level config |
| `imagePullPolicy` _[PullPolicy](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#pullpolicy-v1-core)_ |  |
| `restoreFrom` _string_ |  |
| `metricReaderEnabled` _boolean_ | MetricReaderEnabled enables metric reader for operator and other apps to query metric from MO cluster |


#### ObjectRef





_Appears in:_
- [CNSetSpec](#cnsetspec)

| Field | Description |
| --- | --- |
| `namespace` _string_ |  |
| `name` _string_ |  |


#### Overlay



Overlay allows advanced customization of the pod spec in the set

_Appears in:_
- [BackupJobSpec](#backupjobspec)
- [PodSet](#podset)
- [RestoreJob](#restorejob)

| Field | Description |
| --- | --- |
| `MainContainerOverlay` _[MainContainerOverlay](#maincontaineroverlay)_ |  |
| `volumes` _[Volume](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#volume-v1-core) array_ |  |
| `volumeClaims` _[PersistentVolumeClaim](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#persistentvolumeclaim-v1-core) array_ |  |
| `initContainers` _[Container](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#container-v1-core) array_ |  |
| `sidecarContainers` _[Container](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#container-v1-core) array_ |  |
| `serviceAccountName` _string_ |  |
| `securityContext` _[PodSecurityContext](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#podsecuritycontext-v1-core)_ |  |
| `imagePullSecrets` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#localobjectreference-v1-core) array_ |  |
| `affinity` _[Affinity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#affinity-v1-core)_ |  |
| `tolerations` _[Toleration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#toleration-v1-core) array_ |  |
| `priorityClassName` _string_ |  |
| `terminationGracePeriodSeconds` _integer_ |  |
| `hostAliases` _[HostAlias](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#hostalias-v1-core) array_ |  |
| `topologySpreadConstraints` _[TopologySpreadConstraint](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#topologyspreadconstraint-v1-core) array_ |  |
| `runtimeClassName` _string_ |  |
| `dnsConfig` _[PodDNSConfig](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#poddnsconfig-v1-core)_ |  |
| `podLabels` _object (keys:string, values:string)_ |  |
| `podAnnotations` _object (keys:string, values:string)_ |  |


#### PVCRetentionPolicy

_Underlying type:_ `string`



_Appears in:_
- [LogSetSpec](#logsetspec)
- [S3Provider](#s3provider)



#### PodSet



PodSet is an auxiliary struct to describe a set of isomorphic pods.

_Appears in:_
- [CNSetSpec](#cnsetspec)
- [DNSetSpec](#dnsetspec)
- [LogSetSpec](#logsetspec)
- [ProxySetSpec](#proxysetspec)
- [WebUISpec](#webuispec)

| Field | Description |
| --- | --- |
| `MainContainer` _[MainContainer](#maincontainer)_ |  |
| `overlay` _[Overlay](#overlay)_ |  |
| `replicas` _integer_ | Replicas is the desired number of pods of this set |
| `topologySpread` _string array_ | TopologyEvenSpread specifies what topology domains the Pods in set should be evenly spread in. This will be overridden by .overlay.TopologySpreadConstraints |
| `nodeSelector` _object (keys:string, values:string)_ |  |
| `config` _[TomlConfig](#tomlconfig)_ | Config is the raw config for pods |
| `dnsBasedIdentity` _boolean_ | If enabled, use the Pod dns name as the Pod identity |
| `clusterDomain` _string_ | ClusterDomain is the cluster-domain of current kubernetes cluster, refer https://kubernetes.io/docs/concepts/services-networking/dns-pod-service/ for details |
| `serviceArgs` _string array_ | ServiceArgs define command line options for process, used by logset/cnset/dnset service. NOTE: user should not define "-cfg" argument in this field, which is defined default by controller |
| `memoryLimitPercent` _integer_ | MemoryLimitPercent is percent used to set GOMEMLIMIT env, its value must be in interval (0, 100]. GOMEMLIMIT = limits.memory * MemoryLimitPercent / 100 |
| `exportToPrometheus` _boolean_ |  |


#### PoolScaleStrategy





_Appears in:_
- [PoolStrategy](#poolstrategy)

| Field | Description |
| --- | --- |
| `maxIdle` _integer_ |  |
| `maxPods` _integer_ | MaxPods allowed in this Pool, nil means no limit |


#### PoolStrategy





_Appears in:_
- [CNPoolSpec](#cnpoolspec)

| Field | Description |
| --- | --- |
| `updateStrategy` _[PoolUpdateStrategy](#poolupdatestrategy)_ | UpdateStrategy defines the strategy for pool updating |
| `scaleStrategy` _[PoolScaleStrategy](#poolscalestrategy)_ | UpdateStrategy defines the strategy for pool scaling |


#### PoolUpdateStrategy





_Appears in:_
- [PoolStrategy](#poolstrategy)

| Field | Description |
| --- | --- |
| `reclaimTimeout` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#duration-v1-meta)_ |  |


#### ProxySet



A ProxySet is a resource that represents a set of MO's Proxy instances

_Appears in:_
- [ProxySetList](#proxysetlist)

| Field | Description |
| --- | --- |
| `apiVersion` _string_ | `core.matrixorigin.io/v1alpha1`
| `kind` _string_ | `ProxySet`
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |
| `spec` _[ProxySetSpec](#proxysetspec)_ | Spec is the desired state of ProxySet |
| `deps` _[ProxySetDeps](#proxysetdeps)_ | Deps is the dependencies of ProxySet |


#### ProxySetDeps





_Appears in:_
- [ProxySet](#proxyset)

| Field | Description |
| --- | --- |
| `LogSetRef` _[LogSetRef](#logsetref)_ |  |


#### ProxySetList



ProxySetList contains a list of Proxy



| Field | Description |
| --- | --- |
| `apiVersion` _string_ | `core.matrixorigin.io/v1alpha1`
| `kind` _string_ | `ProxySetList`
| `metadata` _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#listmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |
| `items` _[ProxySet](#proxyset) array_ |  |


#### ProxySetSpec





_Appears in:_
- [MatrixOneClusterSpec](#matrixoneclusterspec)
- [ProxySet](#proxyset)

| Field | Description |
| --- | --- |
| `PodSet` _[PodSet](#podset)_ |  |
| `serviceType` _[ServiceType](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#servicetype-v1-core)_ | ServiceType is the service type of proxy service |
| `nodePort` _integer_ | NodePort specifies the node port to use when ServiceType is NodePort or LoadBalancer, reconciling will fail if the node port is not available. |




#### PythonUdfSidecar





_Appears in:_
- [CNSetSpec](#cnsetspec)

| Field | Description |
| --- | --- |
| `enabled` _boolean_ |  |
| `port` _integer_ |  |
| `image` _string_ | Image is the docker image of the python udf server |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#resourcerequirements-v1-core)_ | Resources is the resource requirement of the python udf server |
| `overlay` _[MainContainerOverlay](#maincontaineroverlay)_ |  |




#### RestoreJob



A RestoreJob is a resource that represents an MO restore job

_Appears in:_
- [RestoreJobList](#restorejoblist)

| Field | Description |
| --- | --- |
| `apiVersion` _string_ | `core.matrixorigin.io/v1alpha1`
| `kind` _string_ | `RestoreJob`
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |
| `spec` _[RestoreJobSpec](#restorejobspec)_ | Spec is the restoreJobSpec |
| `overlay` _[Overlay](#overlay)_ |  |


#### RestoreJobList



RestoreJobList contains a list of RestoreJob



| Field | Description |
| --- | --- |
| `apiVersion` _string_ | `core.matrixorigin.io/v1alpha1`
| `kind` _string_ | `RestoreJobList`
| `metadata` _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#listmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |
| `items` _[RestoreJob](#restorejob) array_ |  |


#### RestoreJobSpec





_Appears in:_
- [RestoreJob](#restorejob)

| Field | Description |
| --- | --- |
| `ttl` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#duration-v1-meta)_ | ttl defines the time to live of the backup job after completed or failed |
| `backupName` _string_ | backupName specifies the backup to restore, must be set UNLESS externalSource is set |
| `externalSource` _[SharedStorageProvider](#sharedstorageprovider)_ | optional, restore from an external source, mutual exclusive with backupName |
| `target` _[SharedStorageProvider](#sharedstorageprovider)_ | target specifies the restore location |




#### RollingUpdateStrategy





_Appears in:_
- [CNSetSpec](#cnsetspec)
- [WebUISpec](#webuispec)

| Field | Description |
| --- | --- |
| `maxSurge` _IntOrString_ | MaxSurge is an optional field that specifies the maximum number of Pods that can be created over the desired number of Pods. |
| `maxUnavailable` _IntOrString_ | MaxUnavailable an optional field that specifies the maximum number of Pods that can be unavailable during the update process. |


#### S3Provider





_Appears in:_
- [BucketClaimSpec](#bucketclaimspec)
- [SharedStorageProvider](#sharedstorageprovider)

| Field | Description |
| --- | --- |
| `path` _string_ | Path is the s3 storage path in <bucket-name>/<folder> format, e.g. "my-bucket/my-folder" |
| `type` _[S3ProviderType](#s3providertype)_ | S3ProviderType is type of this s3 provider, options: [aws, minio] default to aws |
| `region` _string_ | Region of the bucket the default region will be inferred from the deployment environment |
| `endpoint` _string_ | Endpoint is the endpoint of the S3 compatible service default to aws S3 well known endpoint |
| `secretRef` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#localobjectreference-v1-core)_ | Credentials for s3, the client will automatically discover credential sources from the environment if not specified |
| `certificateRef` _[CertificateRef](#certificateref)_ | CertificateRef allow specifies custom CA certificate for the object storage |
| `s3RetentionPolicy` _[PVCRetentionPolicy](#pvcretentionpolicy)_ | S3RetentionPolicy defines the retention policy of orphaned S3 bucket storage |


#### S3ProviderType

_Underlying type:_ `string`



_Appears in:_
- [S3Provider](#s3provider)



#### ScalingConfig





_Appears in:_
- [CNSetSpec](#cnsetspec)

| Field | Description |
| --- | --- |
| `storeDrainEnabled` _boolean_ | StoreDrainEnabled is the flag to enable store draining |
| `storeDrainTimeout` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#duration-v1-meta)_ | StoreDrainTimeout is the timeout for draining a CN store |


#### SharedStorageCache





_Appears in:_
- [CNSetSpec](#cnsetspec)
- [DNSetSpec](#dnsetspec)

| Field | Description |
| --- | --- |
| `memoryCacheSize` _Quantity_ | MemoryCacheSize specifies how much memory would be used to cache the object in shared storage, the default size would be 50% of the container memory request MemoryCache cannot be completely disabled due to MO limitation currently, you can set MemoryCacheSize to 1B to achieve an effect similar to disabling |
| `diskCacheSize` _Quantity_ | DiskCacheSize specifies how much disk space can be used to cache the object in shared storage, the default size would be 90% of the cacheVolume size to reserve some space to the filesystem metadata and avoid disk space exhaustion DiskCache would be disabled if CacheVolume is not set for DN/CN, and if DiskCacheSize is set while the CacheVolume is not set for DN/CN, an error would be raised to indicate the misconfiguration. NOTE: Unless there is a specific reason not to set this field, it is usually more reasonable to let the operator set the available disk cache size according to the actual size of the cacheVolume. |


#### SharedStorageProvider





_Appears in:_
- [BackupJobSpec](#backupjobspec)
- [BackupMeta](#backupmeta)
- [LogSetSpec](#logsetspec)
- [RestoreJobSpec](#restorejobspec)

| Field | Description |
| --- | --- |
| `s3` _[S3Provider](#s3provider)_ | S3 specifies an S3 bucket as the shared storage provider, mutual-exclusive with other providers. |
| `fileSystem` _[FileSystemProvider](#filesystemprovider)_ | FileSystem specified a fileSystem path as the shared storage provider, it assumes a shared filesystem is mounted to this path and instances can safely read-write this path in current manner. |


#### Store





_Appears in:_
- [FailoverStatus](#failoverstatus)

| Field | Description |
| --- | --- |
| `podName` _string_ |  |
| `phase` _string_ |  |
| `lastTransition` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#time-v1-meta)_ |  |


#### TomlConfig



TomlConfig is an auxiliary struct that serialize a nested struct to raw string in toml format on serialization and vise-versa

_Appears in:_
- [PodSet](#podset)



#### Volume





_Appears in:_
- [CNSetSpec](#cnsetspec)
- [DNSetSpec](#dnsetspec)
- [LogSetSpec](#logsetspec)

| Field | Description |
| --- | --- |
| `size` _Quantity_ | Size is the desired storage size of the volume |
| `storageClassName` _string_ | StorageClassName reference to the storageclass of the desired volume, the default storageclass of the cluster would be used if no specified. |
| `memoryCacheSize` _Quantity_ | Deprecated: use SharedStorageCache instead |


#### WebUI



WebUI  is a resource that represents a set of MO's webui instances



| Field | Description |
| --- | --- |
| `apiVersion` _string_ | `core.matrixorigin.io/v1alpha1`
| `kind` _string_ | `WebUI`
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |
| `spec` _[WebUISpec](#webuispec)_ | Spec is the desired state of WebUI |
| `deps` _[WebUIDeps](#webuideps)_ | Deps is the dependencies of WebUI |


#### WebUIDeps





_Appears in:_
- [WebUI](#webui)

| Field | Description |
| --- | --- |
| `cnset` _[CNSet](#cnset)_ | The WebUI it depends on |


#### WebUISpec





_Appears in:_
- [MatrixOneClusterSpec](#matrixoneclusterspec)
- [WebUI](#webui)

| Field | Description |
| --- | --- |
| `PodSet` _[PodSet](#podset)_ |  |
| `serviceType` _[ServiceType](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#servicetype-v1-core)_ | ServiceType is the service type of cn service |
| `updateStrategy` _[RollingUpdateStrategy](#rollingupdatestrategy)_ | UpdateStrategy rolling update strategy |
| `imagePullPolicy` _[PullPolicy](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#pullpolicy-v1-core)_ |  |


