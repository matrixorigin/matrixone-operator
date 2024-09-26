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

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `core.matrixorigin.io/v1alpha1` | | |
| `kind` _string_ | `Backup` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `meta` _[BackupMeta](#backupmeta)_ | Meta is the backupMeta |  |  |


#### BackupJob



A BackupJob is a resource that represents an MO backup job



_Appears in:_
- [BackupJobList](#backupjoblist)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `core.matrixorigin.io/v1alpha1` | | |
| `kind` _string_ | `BackupJob` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[BackupJobSpec](#backupjobspec)_ | Spec is the backupJobSpec |  |  |


#### BackupJobList



BackupJobList contains a list of BackupJob





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `core.matrixorigin.io/v1alpha1` | | |
| `kind` _string_ | `BackupJobList` | | |
| `metadata` _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#listmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `items` _[BackupJob](#backupjob) array_ |  |  |  |


#### BackupJobSpec



BackupJobSpec specifies the backup job



_Appears in:_
- [BackupJob](#backupjob)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `ttl` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#duration-v1-meta)_ | ttl defines the time to live of the backup job after completed or failed |  |  |
| `source` _[BackupSource](#backupsource)_ | source the backup source |  |  |
| `target` _[SharedStorageProvider](#sharedstorageprovider)_ |  |  |  |
| `overlay` _[Overlay](#overlay)_ |  |  |  |




#### BackupList



BackupList contains a list of BackupJ





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `core.matrixorigin.io/v1alpha1` | | |
| `kind` _string_ | `BackupList` | | |
| `metadata` _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#listmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `items` _[Backup](#backup) array_ |  |  |  |


#### BackupMeta



BackupMeta specifies the backup



_Appears in:_
- [Backup](#backup)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `location` _[SharedStorageProvider](#sharedstorageprovider)_ | location is the data location of the backup |  |  |
| `id` _string_ | id uniquely identifies the backup |  |  |
| `size` _[Quantity](#quantity)_ | size is the backup data size |  |  |
| `atTime` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#time-v1-meta)_ | atTime is the backup start time |  |  |
| `completeTime` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#time-v1-meta)_ | completeTime the backup complete time |  |  |
| `sourceRef` _string_ | clusterRef is the reference to the cluster that produce this backup |  |  |
| `raw` _string_ |  |  |  |


#### BackupSource



BackupSource is the source of the backup job



_Appears in:_
- [BackupJobSpec](#backupjobspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `clusterRef` _string_ | clusterRef is the name of the cluster to back up, mutual exclusive with cnSetRef |  |  |
| `cnSetRef` _string_ | cnSetRef is the name of the cnSet to back up, mutual exclusive with clusterRef |  |  |
| `secretRef` _string_ | optional, secretRef is the name of the secret to use for authentication |  |  |


#### BucketClaim



A BucketClaim is a resource that represents the object storage bucket resource used by a mo cluster



_Appears in:_
- [BucketClaimList](#bucketclaimlist)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `core.matrixorigin.io/v1alpha1` | | |
| `kind` _string_ | `BucketClaim` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[BucketClaimSpec](#bucketclaimspec)_ | Spec is the desired state of BucketClaim |  |  |


#### BucketClaimList



BucketClaimList contains a list of BucketClaim





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `core.matrixorigin.io/v1alpha1` | | |
| `kind` _string_ | `BucketClaimList` | | |
| `metadata` _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#listmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `items` _[BucketClaim](#bucketclaim) array_ |  |  |  |


#### BucketClaimSpec







_Appears in:_
- [BucketClaim](#bucketclaim)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `s3` _[S3Provider](#s3provider)_ | S3 specifies an S3 bucket as the shared storage provider, mutual-exclusive with other providers. |  |  |
| `logSetSpec` _[PodTemplateSpec](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#podtemplatespec-v1-core)_ | LogSetTemplate is a complete copy version of kruise statefulset PodTemplateSpec |  | Schemaless: {} <br /> |




#### CNClaim



CNClaim claim a CN to use



_Appears in:_
- [CNClaimList](#cnclaimlist)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `core.matrixorigin.io/v1alpha1` | | |
| `kind` _string_ | `CNClaim` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[CNClaimSpec](#cnclaimspec)_ |  |  |  |


#### CNClaimList



CNClaimList contains a list of CNClaims





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `core.matrixorigin.io/v1alpha1` | | |
| `kind` _string_ | `CNClaimList` | | |
| `metadata` _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#listmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `items` _[CNClaim](#cnclaim) array_ |  |  |  |


#### CNClaimPhase

_Underlying type:_ _string_





_Appears in:_
- [CNClaimStatus](#cnclaimstatus)



#### CNClaimSet



CNClaimSet orchestrates a set of CNClaims



_Appears in:_
- [CNClaimSetList](#cnclaimsetlist)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `core.matrixorigin.io/v1alpha1` | | |
| `kind` _string_ | `CNClaimSet` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[CNClaimSetSpec](#cnclaimsetspec)_ |  |  |  |


#### CNClaimSetList



CNClaimSetList contains a list of CNClaimSet





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `core.matrixorigin.io/v1alpha1` | | |
| `kind` _string_ | `CNClaimSetList` | | |
| `metadata` _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#listmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `items` _[CNClaimSet](#cnclaimset) array_ |  |  |  |


#### CNClaimSetSpec







_Appears in:_
- [CNClaimSet](#cnclaimset)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `replicas` _integer_ |  |  |  |
| `template` _[CNClaimTemplate](#cnclaimtemplate)_ |  |  |  |
| `selector` _[LabelSelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#labelselector-v1-meta)_ |  |  |  |




#### CNClaimSpec







_Appears in:_
- [CNClaim](#cnclaim)
- [CNClaimTemplate](#cnclaimtemplate)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `podName` _string_ | PodName is usually populated by controller and would be part of the claim spec<br />that must be persisted once bound |  |  |
| `nodeName` _string_ | NodeName is usually populated by controller and would be part of the claim spec |  |  |
| `sourcePod` _[ClaimPodRef](#claimpodref)_ | sourcePod is the pod that previously owned by this claim and is now being migrated |  |  |
| `selector` _[LabelSelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#labelselector-v1-meta)_ |  |  |  |
| `cnLabels` _[CNLabel](#cnlabel) array_ |  |  |  |
| `ownerName` _string_ |  |  |  |
| `additionalPodLabels` _object (keys:string, values:string)_ | AdditionalPodLabels specifies the addition labels added to Pod after the Pod is claimed by this claim |  |  |
| `poolName` _string_ | PoolName is usually populated by controller that which pool the claim is nominated |  |  |


#### CNClaimStatus







_Appears in:_
- [CNClaimSetStatus](#cnclaimsetstatus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `phase` _[CNClaimPhase](#cnclaimphase)_ |  |  |  |
| `store` _[CNStoreStatus](#cnstorestatus)_ |  |  |  |
| `boundTime` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#time-v1-meta)_ |  |  |  |
| `migrate` _[MigrateStatus](#migratestatus)_ | migrate is the migrating status of Pods under CNClaim |  |  |


#### CNClaimTemplate







_Appears in:_
- [CNClaimSetSpec](#cnclaimsetspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `metadata` _[EmbeddedMetadata](#embeddedmetadata)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[CNClaimSpec](#cnclaimspec)_ |  |  |  |


#### CNGroup







_Appears in:_
- [MatrixOneClusterSpec](#matrixoneclusterspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `image` _string_ | Image is the docker image of the main container |  |  |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#resourcerequirements-v1-core)_ | Resources is the resource requirement of the main conainer |  |  |
| `overlay` _[Overlay](#overlay)_ |  |  | Schemaless: {} <br /> |
| `replicas` _integer_ | Replicas is the desired number of pods of this set |  |  |
| `topologySpread` _string array_ | TopologyEvenSpread specifies what topology domains the Pods in set should be<br />evenly spread in.<br />This will be overridden by .overlay.TopologySpreadConstraints |  |  |
| `nodeSelector` _object (keys:string, values:string)_ |  |  |  |
| `config` _[TomlConfig](#tomlconfig)_ | Config is the raw config for pods |  | Type: string <br /> |
| `dnsBasedIdentity` _boolean_ | If enabled, use the Pod dns name as the Pod identity<br />Deprecated: DNSBasedIdentity is barely for keeping backward compatibility |  |  |
| `clusterDomain` _string_ | ClusterDomain is the cluster-domain of current kubernetes cluster,<br />refer https://kubernetes.io/docs/concepts/services-networking/dns-pod-service/ for details |  |  |
| `serviceArgs` _string array_ | ServiceArgs define command line options for process, used by logset/cnset/dnset service.<br />NOTE: user should not define "-cfg" argument in this field, which is defined default by controller |  |  |
| `memoryLimitPercent` _integer_ | MemoryLimitPercent is percent used to set GOMEMLIMIT env, its value must be in interval (0, 100].<br />GOMEMLIMIT = limits.memory * MemoryLimitPercent / 100 |  |  |
| `exportToPrometheus` _boolean_ | ExportToPrometheus enables the pod to be discovered scraped by Prometheus |  |  |
| `promDiscoveryScheme` _[PromDiscoveryScheme](#promdiscoveryscheme)_ | PromDiscoveryScheme indicates how the Pod will be discovered by prometheus, options:<br />- Pod: the pod will be discovered via will-known labels on the Pod<br />- Service: the pod will be discovered via will-known annotations in the service which expose endpoints to the pods<br />default to Service |  |  |
| `semanticVersion` _string_ | SemanticVersion override the semantic version of CN if set,<br />the semantic version of CN will be default to the image tag,<br />if the semantic version is not set, nor the image tag is a valid semantic version,<br />operator will treat the MO as unknown version and will not apply any version-specific<br />reconciliations |  |  |
| `operatorVersion` _string_ | OperatorVersion is the controller version of mo-operator that should be used to<br />reconcile this set |  |  |
| `cacheVolume` _[Volume](#volume)_ | CacheVolume is the desired local cache volume for CNSet,<br />node storage will be used if not specified |  |  |
| `sharedStorageCache` _[SharedStorageCache](#sharedstoragecache)_ | SharedStorageCache is the configuration of the S3 sharedStorageCache |  |  |
| `pythonUdfSidecar` _[PythonUdfSidecar](#pythonudfsidecar)_ | PythonUdfSidecar is the python udf server in CN |  |  |
| `serviceType` _[ServiceType](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#servicetype-v1-core)_ | ServiceType is the service type of cn service | ClusterIP | Enum: [ClusterIP NodePort LoadBalancer] <br /> |
| `serviceAnnotations` _object (keys:string, values:string)_ | ServiceAnnotations are the annotations for the cn service |  |  |
| `nodePort` _integer_ | NodePort specifies the node port to use when ServiceType is NodePort or LoadBalancer,<br />reconciling will fail if the node port is not available. |  |  |
| `role` _[CNRole](#cnrole)_ | [TP, AP], default to TP<br />Deprecated: use labels instead |  |  |
| `cnLabels` _[CNLabel](#cnlabel) array_ | Labels are the CN labels for all the CN stores managed by this CNSet |  |  |
| `scalingConfig` _[ScalingConfig](#scalingconfig)_ | ScalingConfig declares the CN scaling behavior |  |  |
| `updateStrategy` _[RollingUpdateStrategy](#rollingupdatestrategy)_ | UpdateStrategy is the rolling-update strategy of CN |  |  |
| `terminationPolicy` _[CNSetTerminationPolicy](#cnsetterminationpolicy)_ |  |  |  |
| `podManagementPolicy` _string_ | PodManagementPolicy is the pod management policy of the Pod in this Set |  |  |
| `podsToDelete` _string array_ | PodsToDelete are the Pods to delete in the CNSet |  |  |
| `pauseUpdate` _boolean_ | PauseUpdate means the CNSet should pause rolling-update |  |  |
| `reusePVC` _boolean_ | ReusePVC means whether CNSet should reuse PVC |  |  |
| `name` _string_ | Name is the CNGroup name, an error will be raised if duplicated name is found in a mo cluster |  |  |


#### CNGroupStatus







_Appears in:_
- [CNGroupsStatus](#cngroupsstatus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ |  |  |  |
| `host` _string_ |  |  |  |
| `ready` _boolean_ |  |  |  |
| `synced` _boolean_ |  |  |  |




#### CNLabel







_Appears in:_
- [CNClaimSpec](#cnclaimspec)
- [CNGroup](#cngroup)
- [CNSetSpec](#cnsetspec)
- [CNStoreStatus](#cnstorestatus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `key` _string_ | Key is the store label key |  |  |
| `values` _string array_ | Values are the store label values |  |  |


#### CNPool



CNPool maintains a pool of CN Pods



_Appears in:_
- [CNPoolList](#cnpoollist)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `core.matrixorigin.io/v1alpha1` | | |
| `kind` _string_ | `CNPool` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[CNPoolSpec](#cnpoolspec)_ |  |  |  |


#### CNPoolList



CNPoolList contains a list of CNPool





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `core.matrixorigin.io/v1alpha1` | | |
| `kind` _string_ | `CNPoolList` | | |
| `metadata` _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#listmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `items` _[CNPool](#cnpool) array_ |  |  |  |


#### CNPoolSpec







_Appears in:_
- [CNPool](#cnpool)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `template` _[CNSetSpec](#cnsetspec)_ | Template is the CNSet template of the Pool |  |  |
| `podLabels` _object (keys:string, values:string)_ | PodLabels is the Pod labels of the CN in Pool |  |  |
| `deps` _[CNSetDeps](#cnsetdeps)_ | Deps is the dependencies of the Pool |  |  |
| `strategy` _[PoolStrategy](#poolstrategy)_ |  |  |  |




#### CNRole

_Underlying type:_ _string_





_Appears in:_
- [CNGroup](#cngroup)
- [CNSetSpec](#cnsetspec)



#### CNSet



A CNSet is a resource that represents a set of MO's CN instances



_Appears in:_
- [WebUIDeps](#webuideps)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `core.matrixorigin.io/v1alpha1` | | |
| `kind` _string_ | `CNSet` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[CNSetSpec](#cnsetspec)_ | Spec is the desired state of CNSet |  |  |
| `deps` _[CNSetDeps](#cnsetdeps)_ | Deps is the dependencies of CNSet |  |  |


#### CNSetDeps







_Appears in:_
- [CNPoolSpec](#cnpoolspec)
- [CNSet](#cnset)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `logSet` _[LogSet](#logset)_ | The LogSet it depends on, mutual exclusive with ExternalLogSet |  | Schemaless: {} <br />Type: object <br /> |
| `externalLogSet` _[ExternalLogSet](#externallogset)_ | An external LogSet the CNSet should connected to,<br />mutual exclusive with LogSet<br />TODO: rethink the schema of ExternalLogSet |  |  |
| `dnSet` _[DNSet](#dnset)_ | The DNSet it depends on |  | Schemaless: {} <br />Type: object <br /> |


#### CNSetSpec







_Appears in:_
- [CNGroup](#cngroup)
- [CNPoolSpec](#cnpoolspec)
- [CNSet](#cnset)
- [MatrixOneClusterSpec](#matrixoneclusterspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `image` _string_ | Image is the docker image of the main container |  |  |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#resourcerequirements-v1-core)_ | Resources is the resource requirement of the main conainer |  |  |
| `overlay` _[Overlay](#overlay)_ |  |  | Schemaless: {} <br /> |
| `replicas` _integer_ | Replicas is the desired number of pods of this set |  |  |
| `topologySpread` _string array_ | TopologyEvenSpread specifies what topology domains the Pods in set should be<br />evenly spread in.<br />This will be overridden by .overlay.TopologySpreadConstraints |  |  |
| `nodeSelector` _object (keys:string, values:string)_ |  |  |  |
| `config` _[TomlConfig](#tomlconfig)_ | Config is the raw config for pods |  | Type: string <br /> |
| `dnsBasedIdentity` _boolean_ | If enabled, use the Pod dns name as the Pod identity<br />Deprecated: DNSBasedIdentity is barely for keeping backward compatibility |  |  |
| `clusterDomain` _string_ | ClusterDomain is the cluster-domain of current kubernetes cluster,<br />refer https://kubernetes.io/docs/concepts/services-networking/dns-pod-service/ for details |  |  |
| `serviceArgs` _string array_ | ServiceArgs define command line options for process, used by logset/cnset/dnset service.<br />NOTE: user should not define "-cfg" argument in this field, which is defined default by controller |  |  |
| `memoryLimitPercent` _integer_ | MemoryLimitPercent is percent used to set GOMEMLIMIT env, its value must be in interval (0, 100].<br />GOMEMLIMIT = limits.memory * MemoryLimitPercent / 100 |  |  |
| `exportToPrometheus` _boolean_ | ExportToPrometheus enables the pod to be discovered scraped by Prometheus |  |  |
| `promDiscoveryScheme` _[PromDiscoveryScheme](#promdiscoveryscheme)_ | PromDiscoveryScheme indicates how the Pod will be discovered by prometheus, options:<br />- Pod: the pod will be discovered via will-known labels on the Pod<br />- Service: the pod will be discovered via will-known annotations in the service which expose endpoints to the pods<br />default to Service |  |  |
| `semanticVersion` _string_ | SemanticVersion override the semantic version of CN if set,<br />the semantic version of CN will be default to the image tag,<br />if the semantic version is not set, nor the image tag is a valid semantic version,<br />operator will treat the MO as unknown version and will not apply any version-specific<br />reconciliations |  |  |
| `operatorVersion` _string_ | OperatorVersion is the controller version of mo-operator that should be used to<br />reconcile this set |  |  |
| `cacheVolume` _[Volume](#volume)_ | CacheVolume is the desired local cache volume for CNSet,<br />node storage will be used if not specified |  |  |
| `sharedStorageCache` _[SharedStorageCache](#sharedstoragecache)_ | SharedStorageCache is the configuration of the S3 sharedStorageCache |  |  |
| `pythonUdfSidecar` _[PythonUdfSidecar](#pythonudfsidecar)_ | PythonUdfSidecar is the python udf server in CN |  |  |
| `serviceType` _[ServiceType](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#servicetype-v1-core)_ | ServiceType is the service type of cn service | ClusterIP | Enum: [ClusterIP NodePort LoadBalancer] <br /> |
| `serviceAnnotations` _object (keys:string, values:string)_ | ServiceAnnotations are the annotations for the cn service |  |  |
| `nodePort` _integer_ | NodePort specifies the node port to use when ServiceType is NodePort or LoadBalancer,<br />reconciling will fail if the node port is not available. |  |  |
| `role` _[CNRole](#cnrole)_ | [TP, AP], default to TP<br />Deprecated: use labels instead |  |  |
| `cnLabels` _[CNLabel](#cnlabel) array_ | Labels are the CN labels for all the CN stores managed by this CNSet |  |  |
| `scalingConfig` _[ScalingConfig](#scalingconfig)_ | ScalingConfig declares the CN scaling behavior |  |  |
| `updateStrategy` _[RollingUpdateStrategy](#rollingupdatestrategy)_ | UpdateStrategy is the rolling-update strategy of CN |  |  |
| `terminationPolicy` _[CNSetTerminationPolicy](#cnsetterminationpolicy)_ |  |  |  |
| `podManagementPolicy` _string_ | PodManagementPolicy is the pod management policy of the Pod in this Set |  |  |
| `podsToDelete` _string array_ | PodsToDelete are the Pods to delete in the CNSet |  |  |
| `pauseUpdate` _boolean_ | PauseUpdate means the CNSet should pause rolling-update |  |  |
| `reusePVC` _boolean_ | ReusePVC means whether CNSet should reuse PVC |  |  |


#### CNSetTerminationPolicy

_Underlying type:_ _string_





_Appears in:_
- [CNGroup](#cngroup)
- [CNSetSpec](#cnsetspec)





#### CNStoreStatus







_Appears in:_
- [CNClaimStatus](#cnclaimstatus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `serviceID` _string_ |  |  |  |
| `lockServiceAddress` _string_ |  |  |  |
| `pipelineServiceAddress` _string_ |  |  |  |
| `sqlAddress` _string_ |  |  |  |
| `queryAddress` _string_ |  |  |  |
| `workState` _integer_ |  |  |  |
| `labels` _[CNLabel](#cnlabel) array_ |  |  |  |
| `string` _string_ | PodName is the CN PodName |  |  |
| `boundTime` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#time-v1-meta)_ | BoundTime is the time when the CN is bound |  |  |


#### CertificateRef







_Appears in:_
- [S3Provider](#s3provider)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | secret name |  |  |
| `files` _string array_ | cert files in the secret |  |  |


#### ClaimPodRef







_Appears in:_
- [CNClaimSpec](#cnclaimspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `podName` _string_ | PodName is usually populated by controller and would be part of the claim spec<br />that must be persisted once bound |  |  |
| `nodeName` _string_ | NodeName is usually populated by controller and would be part of the claim spec |  |  |




#### ConditionalStatus







_Appears in:_
- [BackupJobStatus](#backupjobstatus)
- [BucketClaimStatus](#bucketclaimstatus)
- [ProxySetStatus](#proxysetstatus)
- [RestoreJobStatus](#restorejobstatus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#condition-v1-meta) array_ |  |  |  |


#### ConfigThatChangeCNSpec



ConfigThatChangeCNSpec is an auxiliary struct to hold the config that can change CN spec



_Appears in:_
- [CNGroup](#cngroup)
- [CNSetSpec](#cnsetspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `cacheVolume` _[Volume](#volume)_ | CacheVolume is the desired local cache volume for CNSet,<br />node storage will be used if not specified |  |  |
| `sharedStorageCache` _[SharedStorageCache](#sharedstoragecache)_ | SharedStorageCache is the configuration of the S3 sharedStorageCache |  |  |
| `pythonUdfSidecar` _[PythonUdfSidecar](#pythonudfsidecar)_ | PythonUdfSidecar is the python udf server in CN |  |  |


#### DNSet



A DNSet is a resource that represents a set of MO's DN instances



_Appears in:_
- [CNSetDeps](#cnsetdeps)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `core.matrixorigin.io/v1alpha1` | | |
| `kind` _string_ | `DNSet` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[DNSetSpec](#dnsetspec)_ | Spec is the desired state of DNSet |  |  |
| `deps` _[DNSetDeps](#dnsetdeps)_ | Deps is the dependencies of DNSet |  |  |


#### DNSetDeps







_Appears in:_
- [DNSet](#dnset)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `logSet` _[LogSet](#logset)_ | The LogSet it depends on, mutual exclusive with ExternalLogSet |  | Schemaless: {} <br />Type: object <br /> |
| `externalLogSet` _[ExternalLogSet](#externallogset)_ | An external LogSet the CNSet should connected to,<br />mutual exclusive with LogSet<br />TODO: rethink the schema of ExternalLogSet |  |  |


#### DNSetSpec







_Appears in:_
- [DNSet](#dnset)
- [MatrixOneClusterSpec](#matrixoneclusterspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `image` _string_ | Image is the docker image of the main container |  |  |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#resourcerequirements-v1-core)_ | Resources is the resource requirement of the main conainer |  |  |
| `overlay` _[Overlay](#overlay)_ |  |  | Schemaless: {} <br /> |
| `replicas` _integer_ | Replicas is the desired number of pods of this set |  |  |
| `topologySpread` _string array_ | TopologyEvenSpread specifies what topology domains the Pods in set should be<br />evenly spread in.<br />This will be overridden by .overlay.TopologySpreadConstraints |  |  |
| `nodeSelector` _object (keys:string, values:string)_ |  |  |  |
| `config` _[TomlConfig](#tomlconfig)_ | Config is the raw config for pods |  | Type: string <br /> |
| `dnsBasedIdentity` _boolean_ | If enabled, use the Pod dns name as the Pod identity<br />Deprecated: DNSBasedIdentity is barely for keeping backward compatibility |  |  |
| `clusterDomain` _string_ | ClusterDomain is the cluster-domain of current kubernetes cluster,<br />refer https://kubernetes.io/docs/concepts/services-networking/dns-pod-service/ for details |  |  |
| `serviceArgs` _string array_ | ServiceArgs define command line options for process, used by logset/cnset/dnset service.<br />NOTE: user should not define "-cfg" argument in this field, which is defined default by controller |  |  |
| `memoryLimitPercent` _integer_ | MemoryLimitPercent is percent used to set GOMEMLIMIT env, its value must be in interval (0, 100].<br />GOMEMLIMIT = limits.memory * MemoryLimitPercent / 100 |  |  |
| `exportToPrometheus` _boolean_ | ExportToPrometheus enables the pod to be discovered scraped by Prometheus |  |  |
| `promDiscoveryScheme` _[PromDiscoveryScheme](#promdiscoveryscheme)_ | PromDiscoveryScheme indicates how the Pod will be discovered by prometheus, options:<br />- Pod: the pod will be discovered via will-known labels on the Pod<br />- Service: the pod will be discovered via will-known annotations in the service which expose endpoints to the pods<br />default to Service |  |  |
| `semanticVersion` _string_ | SemanticVersion override the semantic version of CN if set,<br />the semantic version of CN will be default to the image tag,<br />if the semantic version is not set, nor the image tag is a valid semantic version,<br />operator will treat the MO as unknown version and will not apply any version-specific<br />reconciliations |  |  |
| `operatorVersion` _string_ | OperatorVersion is the controller version of mo-operator that should be used to<br />reconcile this set |  |  |
| `cacheVolume` _[Volume](#volume)_ | CacheVolume is the desired local cache volume for DNSet,<br />node storage will be used if not specified |  |  |
| `sharedStorageCache` _[SharedStorageCache](#sharedstoragecache)_ |  |  |  |




#### EmbeddedMetadata







_Appears in:_
- [CNClaimTemplate](#cnclaimtemplate)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ |  |  |  |
| `labels` _object (keys:string, values:string)_ |  |  |  |
| `annotations` _object (keys:string, values:string)_ |  |  |  |


#### ExternalLogSet







_Appears in:_
- [CNSetDeps](#cnsetdeps)
- [DNSetDeps](#dnsetdeps)
- [LogSetRef](#logsetref)
- [ProxySetDeps](#proxysetdeps)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `haKeeperEndpoint` _string_ | HAKeeperEndpoint of the ExternalLogSet |  |  |


#### FailedPodStrategy

_Underlying type:_ _string_





_Appears in:_
- [LogSetSpec](#logsetspec)





#### FileSystemProvider







_Appears in:_
- [SharedStorageProvider](#sharedstorageprovider)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `path` _string_ | Path the path that the shared fileSystem mounted to |  |  |




#### InitialConfig







_Appears in:_
- [LogSetSpec](#logsetspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `logShards` _integer_ | LogShards is the initial number of log shards,<br />cannot be tuned after cluster creation currently.<br />default to 1 |  |  |
| `dnShards` _integer_ | DNShards is the initial number of DN shards,<br />cannot be tuned after cluster creation currently.<br />default to 1 |  |  |
| `logShardReplicas` _integer_ | LogShardReplicas is the replica numbers of each log shard,<br />cannot be tuned after cluster creation currently.<br />default to 3 if LogSet replicas >= 3, to 1 otherwise |  |  |
| `restoreFrom` _string_ | RestoreFrom declares the HAKeeper data should be restored<br />from the given path when hakeeper is bootstrapped |  |  |


#### LogSet



A LogSet is a resource that represents a set of MO's LogService instances



_Appears in:_
- [CNSetDeps](#cnsetdeps)
- [DNSetDeps](#dnsetdeps)
- [LogSetRef](#logsetref)
- [ProxySetDeps](#proxysetdeps)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `core.matrixorigin.io/v1alpha1` | | |
| `kind` _string_ | `LogSet` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[LogSetSpec](#logsetspec)_ | Spec is the desired state of LogSet |  |  |




#### LogSetRef



LogSetRef reference to an LogSet, either internal or external



_Appears in:_
- [CNSetDeps](#cnsetdeps)
- [DNSetDeps](#dnsetdeps)
- [ProxySetDeps](#proxysetdeps)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `logSet` _[LogSet](#logset)_ | The LogSet it depends on, mutual exclusive with ExternalLogSet |  | Schemaless: {} <br />Type: object <br /> |
| `externalLogSet` _[ExternalLogSet](#externallogset)_ | An external LogSet the CNSet should connected to,<br />mutual exclusive with LogSet<br />TODO: rethink the schema of ExternalLogSet |  |  |


#### LogSetSpec







_Appears in:_
- [LogSet](#logset)
- [MatrixOneClusterSpec](#matrixoneclusterspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `image` _string_ | Image is the docker image of the main container |  |  |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#resourcerequirements-v1-core)_ | Resources is the resource requirement of the main conainer |  |  |
| `overlay` _[Overlay](#overlay)_ |  |  | Schemaless: {} <br /> |
| `replicas` _integer_ | Replicas is the desired number of pods of this set |  |  |
| `topologySpread` _string array_ | TopologyEvenSpread specifies what topology domains the Pods in set should be<br />evenly spread in.<br />This will be overridden by .overlay.TopologySpreadConstraints |  |  |
| `nodeSelector` _object (keys:string, values:string)_ |  |  |  |
| `config` _[TomlConfig](#tomlconfig)_ | Config is the raw config for pods |  | Type: string <br /> |
| `dnsBasedIdentity` _boolean_ | If enabled, use the Pod dns name as the Pod identity<br />Deprecated: DNSBasedIdentity is barely for keeping backward compatibility |  |  |
| `clusterDomain` _string_ | ClusterDomain is the cluster-domain of current kubernetes cluster,<br />refer https://kubernetes.io/docs/concepts/services-networking/dns-pod-service/ for details |  |  |
| `serviceArgs` _string array_ | ServiceArgs define command line options for process, used by logset/cnset/dnset service.<br />NOTE: user should not define "-cfg" argument in this field, which is defined default by controller |  |  |
| `memoryLimitPercent` _integer_ | MemoryLimitPercent is percent used to set GOMEMLIMIT env, its value must be in interval (0, 100].<br />GOMEMLIMIT = limits.memory * MemoryLimitPercent / 100 |  |  |
| `exportToPrometheus` _boolean_ | ExportToPrometheus enables the pod to be discovered scraped by Prometheus |  |  |
| `promDiscoveryScheme` _[PromDiscoveryScheme](#promdiscoveryscheme)_ | PromDiscoveryScheme indicates how the Pod will be discovered by prometheus, options:<br />- Pod: the pod will be discovered via will-known labels on the Pod<br />- Service: the pod will be discovered via will-known annotations in the service which expose endpoints to the pods<br />default to Service |  |  |
| `semanticVersion` _string_ | SemanticVersion override the semantic version of CN if set,<br />the semantic version of CN will be default to the image tag,<br />if the semantic version is not set, nor the image tag is a valid semantic version,<br />operator will treat the MO as unknown version and will not apply any version-specific<br />reconciliations |  |  |
| `operatorVersion` _string_ | OperatorVersion is the controller version of mo-operator that should be used to<br />reconcile this set |  |  |
| `volume` _[Volume](#volume)_ | Volume is the local persistent volume for each LogService instance |  |  |
| `sharedStorage` _[SharedStorageProvider](#sharedstorageprovider)_ | SharedStorage is an external shared storage shared by all LogService instances |  |  |
| `initialConfig` _[InitialConfig](#initialconfig)_ | InitialConfig is the initial configuration of HAKeeper<br />InitialConfig is immutable |  |  |
| `storeFailureTimeout` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#duration-v1-meta)_ | StoreFailureTimeout is the timeout to fail-over the logset Pod after a failure of it is observed |  |  |
| `failedPodStrategy` _[FailedPodStrategy](#failedpodstrategy)_ | FailedPodStrategy controls how to handle failed pod when failover happens, default to Delete |  |  |
| `pvcRetentionPolicy` _[PVCRetentionPolicy](#pvcretentionpolicy)_ | PVCRetentionPolicy defines the retention policy of orphaned PVCs due to cluster deletion, scale-in<br />or failover. Available options:<br />- Delete: delete orphaned PVCs<br />- Retain: keep orphaned PVCs, if the corresponding Pod get created again (e.g. scale-in and scale-out, recreate the cluster),<br />the Pod will reuse the retained PVC which contains previous data. Retained PVCs require manual cleanup if they are no longer needed.<br />The default policy is Delete. |  |  |




#### MainContainer



MainContainer is the description of the main container of a Pod



_Appears in:_
- [CNGroup](#cngroup)
- [CNSetSpec](#cnsetspec)
- [DNSetSpec](#dnsetspec)
- [LogSetSpec](#logsetspec)
- [PodSet](#podset)
- [ProxySetSpec](#proxysetspec)
- [WebUISpec](#webuispec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `image` _string_ | Image is the docker image of the main container |  |  |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#resourcerequirements-v1-core)_ | Resources is the resource requirement of the main conainer |  |  |


#### MainContainerOverlay







_Appears in:_
- [Overlay](#overlay)
- [PythonUdfSidecar](#pythonudfsidecar)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `command` _string array_ |  |  |  |
| `args` _string array_ |  |  |  |
| `envFrom` _[EnvFromSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#envfromsource-v1-core) array_ |  |  | Schemaless: {} <br /> |
| `env` _[EnvVar](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#envvar-v1-core) array_ |  |  | Schemaless: {} <br /> |
| `imagePullPolicy` _[PullPolicy](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#pullpolicy-v1-core)_ | ImagePullPolicy is the pull policy of MatrixOne image. The default value is the same as the<br />default of Kubernetes. | IfNotPresent | Enum: [Always Never IfNotPresent] <br /> |
| `volumeMounts` _[VolumeMount](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#volumemount-v1-core) array_ |  |  | Schemaless: {} <br /> |
| `livenessProbe` _[Probe](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#probe-v1-core)_ |  |  | Schemaless: {} <br /> |
| `readinessProbe` _[Probe](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#probe-v1-core)_ |  |  | Schemaless: {} <br /> |
| `startupProbe` _[Probe](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#probe-v1-core)_ |  |  | Schemaless: {} <br /> |
| `lifecycle` _[Lifecycle](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#lifecycle-v1-core)_ |  |  | Schemaless: {} <br /> |
| `mainContainerSecurityContext` _[SecurityContext](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#securitycontext-v1-core)_ |  |  | Schemaless: {} <br /> |


#### MatrixOneCluster



A MatrixOneCluster is a resource that represents a MatrixOne Cluster





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `core.matrixorigin.io/v1alpha1` | | |
| `kind` _string_ | `MatrixOneCluster` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[MatrixOneClusterSpec](#matrixoneclusterspec)_ | Spec is the desired state of MatrixOneCluster |  |  |


#### MatrixOneClusterSpec



MatrixOneClusterSpec defines the desired state of MatrixOneCluster
Note that MatrixOneCluster does not support specify overlay for underlying sets directly due to the size limitation
of kubernetes apiserver



_Appears in:_
- [MatrixOneCluster](#matrixonecluster)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `tp` _[CNSetSpec](#cnsetspec)_ | TP is the default CN pod set that accepts client connections and execute queries<br />Deprecated: use cnGroups instead |  |  |
| `ap` _[CNSetSpec](#cnsetspec)_ | AP is an optional CN pod set that accept MPP sub-plans to accelerate sql queries<br />Deprecated: use cnGroups instead |  |  |
| `cnGroups` _[CNGroup](#cngroup) array_ | CNGroups are CN pod sets that have different spec like resources, arch, store labels |  |  |
| `dn` _[DNSetSpec](#dnsetspec)_ | DN is the default DN pod set of this Cluster<br />Deprecated: use TN instead |  |  |
| `tn` _[DNSetSpec](#dnsetspec)_ | TN is the default TN pod set of this Cluster |  |  |
| `logService` _[LogSetSpec](#logsetspec)_ | LogService is the default LogService pod set of this cluster |  |  |
| `webui` _[WebUISpec](#webuispec)_ | WebUI is the default web ui pod of this cluster |  |  |
| `proxy` _[ProxySetSpec](#proxysetspec)_ | Proxy defines an optional MO Proxy of this cluster |  |  |
| `version` _string_ | Version is the version of the cluster, which translated<br />to the docker image tag used for each component.<br />default to the recommended version of the operator |  |  |
| `imageRepository` _string_ | ImageRepository allows user to override the default image<br />repository in order to use a docker registry proxy or private<br />registry. |  |  |
| `topologySpread` _string array_ | TopologyEvenSpread specifies default topology policy for all components,<br />this will be overridden by component-level config |  |  |
| `nodeSelector` _object (keys:string, values:string)_ | NodeSelector specifies default node selector for all components,<br />this will be overridden by component-level config |  |  |
| `imagePullPolicy` _[PullPolicy](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#pullpolicy-v1-core)_ |  |  |  |
| `restoreFrom` _string_ |  |  |  |
| `metricReaderEnabled` _boolean_ | MetricReaderEnabled enables metric reader for operator and other apps to query<br />metric from MO cluster |  |  |
| `semanticVersion` _string_ | SemanticVersion override the semantic version of CN if set,<br />the semantic version of CN will be default to the image tag,<br />if the semantic version is not set, nor the image tag is a valid semantic version,<br />operator will treat the MO as unknown version and will not apply any version-specific<br />reconciliations |  |  |


#### MigrateStatus







_Appears in:_
- [CNClaimStatus](#cnclaimstatus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `source` _[Workload](#workload)_ |  |  |  |




#### Overlay



Overlay allows advanced customization of the pod spec in the set



_Appears in:_
- [BackupJobSpec](#backupjobspec)
- [CNGroup](#cngroup)
- [CNSetSpec](#cnsetspec)
- [DNSetSpec](#dnsetspec)
- [LogSetSpec](#logsetspec)
- [PodSet](#podset)
- [ProxySetSpec](#proxysetspec)
- [RestoreJob](#restorejob)
- [WebUISpec](#webuispec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `command` _string array_ |  |  |  |
| `args` _string array_ |  |  |  |
| `envFrom` _[EnvFromSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#envfromsource-v1-core) array_ |  |  | Schemaless: {} <br /> |
| `env` _[EnvVar](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#envvar-v1-core) array_ |  |  | Schemaless: {} <br /> |
| `imagePullPolicy` _[PullPolicy](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#pullpolicy-v1-core)_ | ImagePullPolicy is the pull policy of MatrixOne image. The default value is the same as the<br />default of Kubernetes. | IfNotPresent | Enum: [Always Never IfNotPresent] <br /> |
| `volumeMounts` _[VolumeMount](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#volumemount-v1-core) array_ |  |  | Schemaless: {} <br /> |
| `livenessProbe` _[Probe](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#probe-v1-core)_ |  |  | Schemaless: {} <br /> |
| `readinessProbe` _[Probe](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#probe-v1-core)_ |  |  | Schemaless: {} <br /> |
| `startupProbe` _[Probe](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#probe-v1-core)_ |  |  | Schemaless: {} <br /> |
| `lifecycle` _[Lifecycle](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#lifecycle-v1-core)_ |  |  | Schemaless: {} <br /> |
| `mainContainerSecurityContext` _[SecurityContext](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#securitycontext-v1-core)_ |  |  | Schemaless: {} <br /> |
| `volumes` _[Volume](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#volume-v1-core) array_ |  |  | Schemaless: {} <br /> |
| `volumeClaims` _[PersistentVolumeClaim](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#persistentvolumeclaim-v1-core) array_ |  |  | Schemaless: {} <br /> |
| `initContainers` _[Container](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#container-v1-core) array_ |  |  | Schemaless: {} <br /> |
| `sidecarContainers` _[Container](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#container-v1-core) array_ |  |  | Schemaless: {} <br /> |
| `serviceAccountName` _string_ |  |  |  |
| `securityContext` _[PodSecurityContext](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#podsecuritycontext-v1-core)_ |  |  | Schemaless: {} <br /> |
| `imagePullSecrets` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#localobjectreference-v1-core) array_ |  |  | Schemaless: {} <br /> |
| `affinity` _[Affinity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#affinity-v1-core)_ |  |  | Schemaless: {} <br /> |
| `tolerations` _[Toleration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#toleration-v1-core) array_ |  |  | Schemaless: {} <br /> |
| `priorityClassName` _string_ |  |  |  |
| `terminationGracePeriodSeconds` _integer_ |  |  |  |
| `hostAliases` _[HostAlias](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#hostalias-v1-core) array_ |  |  | Schemaless: {} <br /> |
| `topologySpreadConstraints` _[TopologySpreadConstraint](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#topologyspreadconstraint-v1-core) array_ |  |  | Schemaless: {} <br /> |
| `runtimeClassName` _string_ |  |  |  |
| `dnsConfig` _[PodDNSConfig](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#poddnsconfig-v1-core)_ |  |  | Schemaless: {} <br /> |
| `podLabels` _object (keys:string, values:string)_ |  |  |  |
| `podAnnotations` _object (keys:string, values:string)_ |  |  |  |
| `shareProcessNamespace` _boolean_ |  |  |  |


#### PVCRetentionPolicy

_Underlying type:_ _string_





_Appears in:_
- [LogSetSpec](#logsetspec)
- [S3Provider](#s3provider)



#### PodSet



PodSet is an auxiliary struct to describe a set of isomorphic pods.



_Appears in:_
- [CNGroup](#cngroup)
- [CNSetSpec](#cnsetspec)
- [DNSetSpec](#dnsetspec)
- [LogSetSpec](#logsetspec)
- [ProxySetSpec](#proxysetspec)
- [WebUISpec](#webuispec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `image` _string_ | Image is the docker image of the main container |  |  |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#resourcerequirements-v1-core)_ | Resources is the resource requirement of the main conainer |  |  |
| `overlay` _[Overlay](#overlay)_ |  |  | Schemaless: {} <br /> |
| `replicas` _integer_ | Replicas is the desired number of pods of this set |  |  |
| `topologySpread` _string array_ | TopologyEvenSpread specifies what topology domains the Pods in set should be<br />evenly spread in.<br />This will be overridden by .overlay.TopologySpreadConstraints |  |  |
| `nodeSelector` _object (keys:string, values:string)_ |  |  |  |
| `config` _[TomlConfig](#tomlconfig)_ | Config is the raw config for pods |  | Type: string <br /> |
| `dnsBasedIdentity` _boolean_ | If enabled, use the Pod dns name as the Pod identity<br />Deprecated: DNSBasedIdentity is barely for keeping backward compatibility |  |  |
| `clusterDomain` _string_ | ClusterDomain is the cluster-domain of current kubernetes cluster,<br />refer https://kubernetes.io/docs/concepts/services-networking/dns-pod-service/ for details |  |  |
| `serviceArgs` _string array_ | ServiceArgs define command line options for process, used by logset/cnset/dnset service.<br />NOTE: user should not define "-cfg" argument in this field, which is defined default by controller |  |  |
| `memoryLimitPercent` _integer_ | MemoryLimitPercent is percent used to set GOMEMLIMIT env, its value must be in interval (0, 100].<br />GOMEMLIMIT = limits.memory * MemoryLimitPercent / 100 |  |  |
| `exportToPrometheus` _boolean_ | ExportToPrometheus enables the pod to be discovered scraped by Prometheus |  |  |
| `promDiscoveryScheme` _[PromDiscoveryScheme](#promdiscoveryscheme)_ | PromDiscoveryScheme indicates how the Pod will be discovered by prometheus, options:<br />- Pod: the pod will be discovered via will-known labels on the Pod<br />- Service: the pod will be discovered via will-known annotations in the service which expose endpoints to the pods<br />default to Service |  |  |
| `semanticVersion` _string_ | SemanticVersion override the semantic version of CN if set,<br />the semantic version of CN will be default to the image tag,<br />if the semantic version is not set, nor the image tag is a valid semantic version,<br />operator will treat the MO as unknown version and will not apply any version-specific<br />reconciliations |  |  |
| `operatorVersion` _string_ | OperatorVersion is the controller version of mo-operator that should be used to<br />reconcile this set |  |  |


#### PoolScaleStrategy







_Appears in:_
- [PoolStrategy](#poolstrategy)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `maxIdle` _integer_ |  |  |  |
| `maxPods` _integer_ | MaxPods allowed in this Pool, nil means no limit |  |  |


#### PoolStrategy







_Appears in:_
- [CNPoolSpec](#cnpoolspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `updateStrategy` _[PoolUpdateStrategy](#poolupdatestrategy)_ | UpdateStrategy defines the strategy for pool updating |  |  |
| `scaleStrategy` _[PoolScaleStrategy](#poolscalestrategy)_ | UpdateStrategy defines the strategy for pool scaling |  |  |


#### PoolUpdateStrategy







_Appears in:_
- [PoolStrategy](#poolstrategy)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `reclaimTimeout` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#duration-v1-meta)_ |  |  |  |


#### PromDiscoveryScheme

_Underlying type:_ _string_





_Appears in:_
- [CNGroup](#cngroup)
- [CNSetSpec](#cnsetspec)
- [DNSetSpec](#dnsetspec)
- [LogSetSpec](#logsetspec)
- [PodSet](#podset)
- [ProxySetSpec](#proxysetspec)
- [WebUISpec](#webuispec)



#### ProxySet



A ProxySet is a resource that represents a set of MO's Proxy instances



_Appears in:_
- [ProxySetList](#proxysetlist)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `core.matrixorigin.io/v1alpha1` | | |
| `kind` _string_ | `ProxySet` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[ProxySetSpec](#proxysetspec)_ | Spec is the desired state of ProxySet |  |  |
| `deps` _[ProxySetDeps](#proxysetdeps)_ | Deps is the dependencies of ProxySet |  |  |


#### ProxySetDeps







_Appears in:_
- [ProxySet](#proxyset)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `logSet` _[LogSet](#logset)_ | The LogSet it depends on, mutual exclusive with ExternalLogSet |  | Schemaless: {} <br />Type: object <br /> |
| `externalLogSet` _[ExternalLogSet](#externallogset)_ | An external LogSet the CNSet should connected to,<br />mutual exclusive with LogSet<br />TODO: rethink the schema of ExternalLogSet |  |  |


#### ProxySetList



ProxySetList contains a list of Proxy





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `core.matrixorigin.io/v1alpha1` | | |
| `kind` _string_ | `ProxySetList` | | |
| `metadata` _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#listmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `items` _[ProxySet](#proxyset) array_ |  |  |  |


#### ProxySetSpec







_Appears in:_
- [MatrixOneClusterSpec](#matrixoneclusterspec)
- [ProxySet](#proxyset)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `image` _string_ | Image is the docker image of the main container |  |  |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#resourcerequirements-v1-core)_ | Resources is the resource requirement of the main conainer |  |  |
| `overlay` _[Overlay](#overlay)_ |  |  | Schemaless: {} <br /> |
| `replicas` _integer_ | Replicas is the desired number of pods of this set |  |  |
| `topologySpread` _string array_ | TopologyEvenSpread specifies what topology domains the Pods in set should be<br />evenly spread in.<br />This will be overridden by .overlay.TopologySpreadConstraints |  |  |
| `nodeSelector` _object (keys:string, values:string)_ |  |  |  |
| `config` _[TomlConfig](#tomlconfig)_ | Config is the raw config for pods |  | Type: string <br /> |
| `dnsBasedIdentity` _boolean_ | If enabled, use the Pod dns name as the Pod identity<br />Deprecated: DNSBasedIdentity is barely for keeping backward compatibility |  |  |
| `clusterDomain` _string_ | ClusterDomain is the cluster-domain of current kubernetes cluster,<br />refer https://kubernetes.io/docs/concepts/services-networking/dns-pod-service/ for details |  |  |
| `serviceArgs` _string array_ | ServiceArgs define command line options for process, used by logset/cnset/dnset service.<br />NOTE: user should not define "-cfg" argument in this field, which is defined default by controller |  |  |
| `memoryLimitPercent` _integer_ | MemoryLimitPercent is percent used to set GOMEMLIMIT env, its value must be in interval (0, 100].<br />GOMEMLIMIT = limits.memory * MemoryLimitPercent / 100 |  |  |
| `exportToPrometheus` _boolean_ | ExportToPrometheus enables the pod to be discovered scraped by Prometheus |  |  |
| `promDiscoveryScheme` _[PromDiscoveryScheme](#promdiscoveryscheme)_ | PromDiscoveryScheme indicates how the Pod will be discovered by prometheus, options:<br />- Pod: the pod will be discovered via will-known labels on the Pod<br />- Service: the pod will be discovered via will-known annotations in the service which expose endpoints to the pods<br />default to Service |  |  |
| `semanticVersion` _string_ | SemanticVersion override the semantic version of CN if set,<br />the semantic version of CN will be default to the image tag,<br />if the semantic version is not set, nor the image tag is a valid semantic version,<br />operator will treat the MO as unknown version and will not apply any version-specific<br />reconciliations |  |  |
| `operatorVersion` _string_ | OperatorVersion is the controller version of mo-operator that should be used to<br />reconcile this set |  |  |
| `serviceType` _[ServiceType](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#servicetype-v1-core)_ | ServiceType is the service type of proxy service | ClusterIP | Enum: [ClusterIP NodePort LoadBalancer] <br /> |
| `serviceAnnotations` _object (keys:string, values:string)_ | ServiceAnnotations are the annotations for the proxy service |  |  |
| `nodePort` _integer_ | NodePort specifies the node port to use when ServiceType is NodePort or LoadBalancer,<br />reconciling will fail if the node port is not available. |  |  |
| `minReadySeconds` _integer_ |  |  |  |




#### PythonUdfSidecar







_Appears in:_
- [CNGroup](#cngroup)
- [CNSetSpec](#cnsetspec)
- [ConfigThatChangeCNSpec](#configthatchangecnspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enabled` _boolean_ |  |  |  |
| `port` _integer_ |  |  |  |
| `image` _string_ | Image is the docker image of the python udf server |  |  |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#resourcerequirements-v1-core)_ | Resources is the resource requirement of the python udf server |  |  |
| `overlay` _[MainContainerOverlay](#maincontaineroverlay)_ |  |  | Schemaless: {} <br /> |




#### RestoreJob



A RestoreJob is a resource that represents an MO restore job



_Appears in:_
- [RestoreJobList](#restorejoblist)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `core.matrixorigin.io/v1alpha1` | | |
| `kind` _string_ | `RestoreJob` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[RestoreJobSpec](#restorejobspec)_ | Spec is the restoreJobSpec |  |  |
| `overlay` _[Overlay](#overlay)_ |  |  |  |


#### RestoreJobList



RestoreJobList contains a list of RestoreJob





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `core.matrixorigin.io/v1alpha1` | | |
| `kind` _string_ | `RestoreJobList` | | |
| `metadata` _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#listmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `items` _[RestoreJob](#restorejob) array_ |  |  |  |


#### RestoreJobSpec







_Appears in:_
- [RestoreJob](#restorejob)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `ttl` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#duration-v1-meta)_ | ttl defines the time to live of the backup job after completed or failed |  |  |
| `backupName` _string_ | backupName specifies the backup to restore, must be set UNLESS externalSource is set |  |  |
| `externalSource` _[SharedStorageProvider](#sharedstorageprovider)_ | optional, restore from an external source, mutual exclusive with backupName |  |  |
| `target` _[SharedStorageProvider](#sharedstorageprovider)_ | target specifies the restore location |  |  |




#### RollingUpdateStrategy







_Appears in:_
- [CNGroup](#cngroup)
- [CNSetSpec](#cnsetspec)
- [WebUISpec](#webuispec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `maxSurge` _[IntOrString](#intorstring)_ | MaxSurge is an optional field that specifies the maximum number of Pods that<br />can be created over the desired number of Pods. |  |  |
| `maxUnavailable` _[IntOrString](#intorstring)_ | MaxUnavailable an optional field that specifies the maximum number of Pods that<br />can be unavailable during the update process. |  |  |


#### S3Provider







_Appears in:_
- [BucketClaimSpec](#bucketclaimspec)
- [SharedStorageProvider](#sharedstorageprovider)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `path` _string_ | Path is the s3 storage path in <bucket-name>/<folder> format, e.g. "my-bucket/my-folder" |  |  |
| `type` _[S3ProviderType](#s3providertype)_ | S3ProviderType is type of this s3 provider, options: [aws, minio]<br />default to aws |  |  |
| `region` _string_ | Region of the bucket<br />the default region will be inferred from the deployment environment |  |  |
| `endpoint` _string_ | Endpoint is the endpoint of the S3 compatible service<br />default to aws S3 well known endpoint |  |  |
| `secretRef` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#localobjectreference-v1-core)_ | Credentials for s3, the client will automatically discover credential sources<br />from the environment if not specified |  |  |
| `certificateRef` _[CertificateRef](#certificateref)_ | CertificateRef allow specifies custom CA certificate for the object storage |  |  |
| `s3RetentionPolicy` _[PVCRetentionPolicy](#pvcretentionpolicy)_ | S3RetentionPolicy defines the retention policy of orphaned S3 bucket storage |  | Enum: [Delete Retain] <br /> |


#### S3ProviderType

_Underlying type:_ _string_





_Appears in:_
- [S3Provider](#s3provider)



#### ScalingConfig







_Appears in:_
- [CNGroup](#cngroup)
- [CNSetSpec](#cnsetspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `storeDrainEnabled` _boolean_ | StoreDrainEnabled is the flag to enable store draining |  |  |
| `storeDrainTimeout` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#duration-v1-meta)_ | StoreDrainTimeout is the timeout for draining a CN store |  |  |
| `minDelaySeconds` _integer_ | minDelaySeconds is the minimum delay when drain CN store, usually<br />be used to waiting for CN draining be propagated to the whole cluster |  |  |


#### SharedStorageCache







_Appears in:_
- [CNGroup](#cngroup)
- [CNSetSpec](#cnsetspec)
- [ConfigThatChangeCNSpec](#configthatchangecnspec)
- [DNSetSpec](#dnsetspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `memoryCacheSize` _[Quantity](#quantity)_ | MemoryCacheSize specifies how much memory would be used to cache the object in shared storage,<br />the default size would be 50% of the container memory request<br />MemoryCache cannot be completely disabled due to MO limitation currently, you can set MemoryCacheSize<br />to 1B to achieve an effect similar to disabling |  |  |
| `diskCacheSize` _[Quantity](#quantity)_ | DiskCacheSize specifies how much disk space can be used to cache the object in shared storage,<br />the default size would be 90% of the cacheVolume size to reserve some space to the filesystem metadata<br />and avoid disk space exhaustion<br />DiskCache would be disabled if CacheVolume is not set for DN/CN, and if DiskCacheSize is set while the CacheVolume<br />is not set for DN/CN, an error would be raised to indicate the misconfiguration.<br />NOTE: Unless there is a specific reason not to set this field, it is usually more reasonable to let the operator<br />set the available disk cache size according to the actual size of the cacheVolume. |  |  |


#### SharedStorageProvider







_Appears in:_
- [BackupJobSpec](#backupjobspec)
- [BackupMeta](#backupmeta)
- [LogSetSpec](#logsetspec)
- [RestoreJobSpec](#restorejobspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `s3` _[S3Provider](#s3provider)_ | S3 specifies an S3 bucket as the shared storage provider,<br />mutual-exclusive with other providers. |  |  |
| `fileSystem` _[FileSystemProvider](#filesystemprovider)_ | FileSystem specified a fileSystem path as the shared storage provider,<br />it assumes a shared filesystem is mounted to this path and instances can<br />safely read-write this path in current manner. |  |  |


#### State

_Underlying type:_ _string_





_Appears in:_
- [BucketClaimStatus](#bucketclaimstatus)



#### Store







_Appears in:_
- [FailoverStatus](#failoverstatus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `podName` _string_ |  |  |  |
| `phase` _string_ |  |  |  |
| `lastTransition` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#time-v1-meta)_ |  |  |  |


#### TomlConfig



TomlConfig is an auxiliary struct that serialize a nested struct to raw string
in toml format on serialization and vise-versa

_Validation:_
- Type: string

_Appears in:_
- [CNGroup](#cngroup)
- [CNSetSpec](#cnsetspec)
- [DNSetSpec](#dnsetspec)
- [LogSetSpec](#logsetspec)
- [PodSet](#podset)
- [ProxySetSpec](#proxysetspec)
- [WebUISpec](#webuispec)



#### Volume







_Appears in:_
- [CNGroup](#cngroup)
- [CNSetSpec](#cnsetspec)
- [ConfigThatChangeCNSpec](#configthatchangecnspec)
- [DNSetSpec](#dnsetspec)
- [LogSetSpec](#logsetspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `size` _[Quantity](#quantity)_ | Size is the desired storage size of the volume |  |  |
| `storageClassName` _string_ | StorageClassName reference to the storageclass of the desired volume,<br />the default storageclass of the cluster would be used if no specified. |  |  |
| `memoryCacheSize` _[Quantity](#quantity)_ | Deprecated: use SharedStorageCache instead |  |  |


#### WebUI



WebUI  is a resource that represents a set of MO's webui instances





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `core.matrixorigin.io/v1alpha1` | | |
| `kind` _string_ | `WebUI` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[WebUISpec](#webuispec)_ | Spec is the desired state of WebUI |  |  |
| `deps` _[WebUIDeps](#webuideps)_ | Deps is the dependencies of WebUI |  |  |


#### WebUIDeps







_Appears in:_
- [WebUI](#webui)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `cnset` _[CNSet](#cnset)_ | The WebUI it depends on |  | Schemaless: {} <br />Type: object <br /> |


#### WebUISpec







_Appears in:_
- [MatrixOneClusterSpec](#matrixoneclusterspec)
- [WebUI](#webui)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `image` _string_ | Image is the docker image of the main container |  |  |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#resourcerequirements-v1-core)_ | Resources is the resource requirement of the main conainer |  |  |
| `overlay` _[Overlay](#overlay)_ |  |  | Schemaless: {} <br /> |
| `replicas` _integer_ | Replicas is the desired number of pods of this set |  |  |
| `topologySpread` _string array_ | TopologyEvenSpread specifies what topology domains the Pods in set should be<br />evenly spread in.<br />This will be overridden by .overlay.TopologySpreadConstraints |  |  |
| `nodeSelector` _object (keys:string, values:string)_ |  |  |  |
| `config` _[TomlConfig](#tomlconfig)_ | Config is the raw config for pods |  | Type: string <br /> |
| `dnsBasedIdentity` _boolean_ | If enabled, use the Pod dns name as the Pod identity<br />Deprecated: DNSBasedIdentity is barely for keeping backward compatibility |  |  |
| `clusterDomain` _string_ | ClusterDomain is the cluster-domain of current kubernetes cluster,<br />refer https://kubernetes.io/docs/concepts/services-networking/dns-pod-service/ for details |  |  |
| `serviceArgs` _string array_ | ServiceArgs define command line options for process, used by logset/cnset/dnset service.<br />NOTE: user should not define "-cfg" argument in this field, which is defined default by controller |  |  |
| `memoryLimitPercent` _integer_ | MemoryLimitPercent is percent used to set GOMEMLIMIT env, its value must be in interval (0, 100].<br />GOMEMLIMIT = limits.memory * MemoryLimitPercent / 100 |  |  |
| `exportToPrometheus` _boolean_ | ExportToPrometheus enables the pod to be discovered scraped by Prometheus |  |  |
| `promDiscoveryScheme` _[PromDiscoveryScheme](#promdiscoveryscheme)_ | PromDiscoveryScheme indicates how the Pod will be discovered by prometheus, options:<br />- Pod: the pod will be discovered via will-known labels on the Pod<br />- Service: the pod will be discovered via will-known annotations in the service which expose endpoints to the pods<br />default to Service |  |  |
| `semanticVersion` _string_ | SemanticVersion override the semantic version of CN if set,<br />the semantic version of CN will be default to the image tag,<br />if the semantic version is not set, nor the image tag is a valid semantic version,<br />operator will treat the MO as unknown version and will not apply any version-specific<br />reconciliations |  |  |
| `operatorVersion` _string_ | OperatorVersion is the controller version of mo-operator that should be used to<br />reconcile this set |  |  |
| `serviceType` _[ServiceType](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#servicetype-v1-core)_ | ServiceType is the service type of cn service | ClusterIP | Enum: [ClusterIP NodePort LoadBalancer] <br /> |
| `updateStrategy` _[RollingUpdateStrategy](#rollingupdatestrategy)_ | UpdateStrategy rolling update strategy |  |  |
| `imagePullPolicy` _[PullPolicy](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#pullpolicy-v1-core)_ |  |  |  |


#### Workload







_Appears in:_
- [MigrateStatus](#migratestatus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `connections` _integer_ |  |  |  |
| `pipelines` _integer_ |  |  |  |


