# Kruise webhook availability policy

The bundled Kruise chart deliberately uses different failure policies based on
the scope of the admitted resource.

## Built-in Kubernetes resources

Webhooks for Pods, Pod eviction, Deployments, ReplicaSets, StatefulSets,
Namespaces, Services, Ingresses, and CustomResourceDefinitions use
`failurePolicy: Ignore`.

This keeps core Kubernetes API operations available while the Kruise webhook
service is starting, upgrading, or temporarily unavailable. Features implemented
by those webhooks, such as Pod mutation and deletion protection, are not
guaranteed during the outage and resume when the webhook recovers.

## Kruise custom resources

Webhooks whose rules only target `apps.kruise.io` or `policy.kruise.io` resources
use `failurePolicy: Fail`. Their defaulting and validation are part of the
Kruise resource contract, and an outage therefore blocks changes only to the
affected Kruise APIs instead of blocking general Kubernetes workloads.

## StorageClass informer

Kruise v1.8.3 starts a read-only StorageClass informer even when
`StatefulSetAutoResizePVCGate` is disabled. The bundled ClusterRole grants
unconditional `get`, `list`, and `watch` access to StorageClasses so the informer
can run without repeated authorization errors. This permission does not enable
PVC auto-resize and grants no PVC mutation verb.

The chart regression checks can be run with:

```sh
make verify-chart
```

The Kind E2E workflow also disconnects the Kruise webhook Service temporarily.
It verifies that ordinary Pod admission remains available, Kruise custom
resources remain fail-closed, and a Helm upgrade restores the Service and its
admission path. Run that integration coverage with `make e2e-kind`.
