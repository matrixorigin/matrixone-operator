# API Reference

## Packages
- [core.matrixorigin.io/v1alpha1](#corematrixoriginiov1alpha1)


## core.matrixorigin.io/v1alpha1

Package v1alpha1 contains API Schema definitions of the matrixone v1alpha1 API group.
The MatrixOneCluster resource type helps you provision and manage MatrixOne clusters on Kubernetes.
Other resources types represent sub-resources managed by MatrixOneCluster but also allows separated usage for fine-grained control.
Refer to https://docs.matrixorigin.io/ for more information about MatrixOne database and matrixone-operator.

### Resource Types
- [CNSet](#cnset)
- [DNSet](#dnset)
- [LogSet](#logset)
- [MatrixOneCluster](#matrixonecluster)
- [WebUI](#webui)



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


#### CNSetBasic





_Appears in:_
- [CNSetSpec](#cnsetspec)
- [MatrixOneClusterSpec](#matrixoneclusterspec)

| Field | Description |
| --- | --- |
| `PodSet` _[PodSet](#podset)_ |  |
| `serviceType` _[ServiceType](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#servicetype-v1-core)_ | ServiceType is the service type of cn service |
| `nodePort` _integer_ | NodePort specifies the node port to use when ServiceType is NodePort or LoadBalancer, reconciling will fail if the node port is not available. |
| `cacheVolume` _[Volume](#volume)_ | CacheVolume is the desired local cache volume for CNSet, node storage will be used if not specified |
| `sharedStorageCache` _[SharedStorageCache](#sharedstoragecache)_ |  |


#### CNSetDeps





_Appears in:_
- [CNSet](#cnset)

| Field | Description |
| --- | --- |
| `LogSetRef` _[LogSetRef](#logsetref)_ |  |
| `dnSet` _[DNSet](#dnset)_ | The DNSet it depends on |


#### CNSetSpec





_Appears in:_
- [CNSet](#cnset)

| Field | Description |
| --- | --- |
| `CNSetBasic` _[CNSetBasic](#cnsetbasic)_ |  |
| `overlay` _[Overlay](#overlay)_ |  |
| `role` _CNRole_ | [TP, AP], default to TP |




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


#### DNSetBasic





_Appears in:_
- [DNSetSpec](#dnsetspec)
- [MatrixOneClusterSpec](#matrixoneclusterspec)

| Field | Description |
| --- | --- |
| `PodSet` _[PodSet](#podset)_ |  |
| `cacheVolume` _[Volume](#volume)_ | CacheVolume is the desired local cache volume for DNSet, node storage will be used if not specified |
| `sharedStorageCache` _[SharedStorageCache](#sharedstoragecache)_ |  |


#### DNSetDeps





_Appears in:_
- [DNSet](#dnset)

| Field | Description |
| --- | --- |
| `LogSetRef` _[LogSetRef](#logsetref)_ |  |


#### DNSetSpec





_Appears in:_
- [DNSet](#dnset)

| Field | Description |
| --- | --- |
| `DNSetBasic` _[DNSetBasic](#dnsetbasic)_ |  |
| `overlay` _[Overlay](#overlay)_ |  |




#### ExternalLogSet





_Appears in:_
- [LogSetRef](#logsetref)

| Field | Description |
| --- | --- |
| `haKeeperEndpoint` _string_ | HAKeeperEndpoint of the ExternalLogSet |


#### FailedPodStrategy

_Underlying type:_ `string`



_Appears in:_
- [LogSetBasic](#logsetbasic)







#### InitialConfig





_Appears in:_
- [LogSetBasic](#logsetbasic)

| Field | Description |
| --- | --- |
| `logShards` _[int](#int)_ | LogShards is the initial number of log shards, cannot be tuned after cluster creation currently. default to 1 |
| `dnShards` _[int](#int)_ | DNShards is the initial number of DN shards, cannot be tuned after cluster creation currently. default to 1 |
| `logShardReplicas` _[int](#int)_ | LogShardReplicas is the replica numbers of each log shard, cannot be tuned after cluster creation currently. default to 3 if LogSet replicas >= 3, to 1 otherwise |


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


#### LogSetBasic





_Appears in:_
- [LogSetSpec](#logsetspec)
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




#### LogSetRef



LogSetRef reference to an LogSet, either internal or external

_Appears in:_
- [CNSetDeps](#cnsetdeps)
- [DNSetDeps](#dnsetdeps)

| Field | Description |
| --- | --- |
| `logSet` _[LogSet](#logset)_ | The LogSet it depends on, mutual exclusive with ExternalLogSet |
| `externalLogSet` _[ExternalLogSet](#externallogset)_ | An external LogSet the CNSet should connected to, mutual exclusive with LogSet TODO: rethink the schema of ExternalLogSet |


#### LogSetSpec





_Appears in:_
- [LogSet](#logset)

| Field | Description |
| --- | --- |
| `LogSetBasic` _[LogSetBasic](#logsetbasic)_ |  |
| `overlay` _[Overlay](#overlay)_ |  |


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
| `tp` _[CNSetBasic](#cnsetbasic)_ | TP is the default CN pod set that accepts client connections and execute queries |
| `ap` _[CNSetBasic](#cnsetbasic)_ | AP is an optional CN pod set that accept MPP sub-plans to accelerate sql queries |
| `dn` _[DNSetBasic](#dnsetbasic)_ | DN is the default DN pod set of this Cluster |
| `logService` _[LogSetBasic](#logsetbasic)_ | LogService is the default LogService pod set of this cluster |
| `webui` _[WebUIBasic](#webuibasic)_ | WebUI is the default web ui pod of this cluster |
| `version` _string_ | Version is the version of the cluster, which translated to the docker image tag used for each component. default to the recommended version of the operator |
| `imageRepository` _string_ | ImageRepository allows user to override the default image repository in order to use a docker registry proxy or private registry. |
| `topologySpread` _string array_ | TopologyEvenSpread specifies default topology policy for all components, this will be overridden by component-level config |
| `nodeSelector` _object (keys:string, values:string)_ | NodeSelector specifies default node selector for all components, this will be overridden by component-level config |
| `imagePullPolicy` _[PullPolicy](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#pullpolicy-v1-core)_ |  |


#### Overlay



Overlay allows advanced customization of the pod spec in the set

_Appears in:_
- [CNSetSpec](#cnsetspec)
- [DNSetSpec](#dnsetspec)
- [LogSetSpec](#logsetspec)
- [WebUISpec](#webuispec)

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
- [LogSetBasic](#logsetbasic)



#### PodSet



PodSet is an auxiliary struct to describe a set of isomorphic pods.

_Appears in:_
- [CNSetBasic](#cnsetbasic)
- [DNSetBasic](#dnsetbasic)
- [LogSetBasic](#logsetbasic)
- [WebUIBasic](#webuibasic)

| Field | Description |
| --- | --- |
| `MainContainer` _[MainContainer](#maincontainer)_ |  |
| `replicas` _integer_ | Replicas is the desired number of pods of this set |
| `topologySpread` _string array_ | TopologyEvenSpread specifies what topology domains the Pods in set should be evenly spread in. This will be overridden by .overlay.TopologySpreadConstraints |
| `nodeSelector` _object (keys:string, values:string)_ |  |
| `config` _[TomlConfig](#tomlconfig)_ | Config is the raw config for pods |
| `dnsBasedIdentity` _boolean_ | If enabled, use the Pod dns name as the Pod identity |
| `clusterDomain` _string_ | ClusterDomain is the cluster-domain of current kubernetes cluster, refer https://kubernetes.io/docs/concepts/services-networking/dns-pod-service/ for details |
| `serviceArgs` _string array_ | ServiceArgs define command line options for process, used by logset/cnset/dnset service. NOTE: user should not define "-cfg" argument in this field, which is defined default by controller |


#### RollingUpdateStrategy





_Appears in:_
- [WebUIBasic](#webuibasic)

| Field | Description |
| --- | --- |
| `maxSurge` _integer_ | MaxSurge is an optional field that specifies the maximum number of Pods that can be created over the desired number of Pods. |
| `maxUnavailable` _integer_ | MaxUnavailable an optional field that specifies the maximum number of Pods that can be unavailable during the update process. |




#### SharedStorageCache





_Appears in:_
- [CNSetBasic](#cnsetbasic)
- [DNSetBasic](#dnsetbasic)

| Field | Description |
| --- | --- |
| `memoryCacheSize` _Quantity_ |  |
| `diskCacheSize` _Quantity_ |  |


#### SharedStorageProvider





_Appears in:_
- [LogSetBasic](#logsetbasic)

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
- [CNSetBasic](#cnsetbasic)
- [DNSetBasic](#dnsetbasic)
- [LogSetBasic](#logsetbasic)

| Field | Description |
| --- | --- |
| `size` _Quantity_ | Size is the desired storage size of the volume |
| `storageClassName` _string_ | StorageClassName reference to the storageclass of the desired volume, the default storageclass of the cluster would be used if no specified. |
| `memoryCacheSize` _Quantity_ | MemoryCacheSize specifies the memory cache size for read/write this volume |


#### WebUI



WebUI  is a resource that represents a set of MO's webui instances



| Field | Description |
| --- | --- |
| `apiVersion` _string_ | `core.matrixorigin.io/v1alpha1`
| `kind` _string_ | `WebUI`
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |
| `spec` _[WebUISpec](#webuispec)_ | Spec is the desired state of WebUI |
| `deps` _[WebUIDeps](#webuideps)_ | Deps is the dependencies of WebUI |


#### WebUIBasic





_Appears in:_
- [MatrixOneClusterSpec](#matrixoneclusterspec)
- [WebUISpec](#webuispec)

| Field | Description |
| --- | --- |
| `PodSet` _[PodSet](#podset)_ |  |
| `serviceType` _[ServiceType](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#servicetype-v1-core)_ | ServiceType is the service type of cn service |
| `updateStrategy` _[RollingUpdateStrategy](#rollingupdatestrategy)_ | UpdateStrategy rolling update strategy |
| `imagePullPolicy` _[PullPolicy](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#pullpolicy-v1-core)_ |  |


#### WebUIDeps





_Appears in:_
- [WebUI](#webui)

| Field | Description |
| --- | --- |
| `cnset` _[CNSet](#cnset)_ | The WebUI it depends on |


#### WebUISpec





_Appears in:_
- [WebUI](#webui)

| Field | Description |
| --- | --- |
| `WebUIBasic` _[WebUIBasic](#webuibasic)_ |  |
| `overlay` _[Overlay](#overlay)_ |  |


