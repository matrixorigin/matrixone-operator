# Full Configuration

## replicas

Matrixone replicas size

## image

Matrixone image

## command

[Define a Command for container](https://kubernetes.io/docs/tasks/inject-data-application/define-command-argument-container/)

## imagePullSecrets

[Pull an Image from a Private Registry](https://kubernetes.io/docs/tasks/configure-pod-container/pull-image-private-registry/#create-a-pod-that-uses-your-secret)

## terminationGracePeriodSeconds

TerminationGracePeriodSeconds can define elegant closed grace period, that is, after I receive my stop request, how many time for resources to release or do other operations, if the maximum time has not stopped, will be forced to end.
***default value: 30**

## deleteOrphanPvc

Default is set to true, orphaned ( unmounted pvc's ) shall be cleaned up by the operator.

## DisablePVCDeletionFinalizer

Default is set to false, pvc shall be deleted on deletion of CR

## dnsPolicy

pod specific [DNS policies](https://kubernetes.io/docs/concepts/services-networking/dns-pod-service/#pod-s-dns-policy)

## dnsConfig

[DNS settings for a Pod](DNS settings for a Pod)

## imagePullPolicy

[how to pull image](https://kubernetes.io/docs/concepts/containers/images/#image-pull-policy): `Always`, `IfNotPresent`, `Never`

## storageClass

A [StorageClass](https://kubernetes.io/docs/concepts/storage/storage-classes/) provides a way for administrators to describe the "classes" of storage they offer.

## podAnnotations

You can use [Kubernetes annotations](https://kubernetes.io/docs/concepts/overview/working-with-objects/annotations/) to attach arbitrary non-identifying metadata to objects. Clients such as tools and libraries can retrieve this metadata.


## logVolumeCap

Matrixone log volume capacity


## dataVolumeCap

Matrixone data volume capacity

## serviceType

k8s [service type](https://kubernetes.io/docs/concepts/services-networking/service/#publishing-services-service-types): `ClusterIP`, `NodePort`, `LoadBalancer`

## podName

Inject podName parameter when pod is created

## updateStrategy

[updateStrategy](https://kubernetes.io/docs/concepts/workloads/controllers/statefulset/#update-strategies) allows you to configure and disable automated rolling updates for containers, labels, resource request/limits, and annotations for the Pods in a StatefulSet.

## limits

pod runtime resource limits: cpu, memory

## requests

pod runtime resource requests: cpu, memory

## affinity

Rules of pod schedule, see more:  [Affinity and anti-affinity](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#affinity-and-anti-affinity)

## nodeSelector

Configure rules to schedule pods to specific nodes， see more: [nodeSelector](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#nodeselector)

## podManagementPolicy

For some distributed systems, the StatefulSet ordering guarantees are unnecessary and/or undesirable. These systems require only uniqueness and identity. To address this, in Kubernetes 1.7,  introduced [.spec.podManagementPolicy](https://kubernetes.io/docs/tutorials/stateful-application/basic-stateful-set/#pod-management-policy) to the StatefulSet API Object
