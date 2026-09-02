# Kruise webhook availability policy

The bundled Kruise chart deliberately uses different failure policies based on
the scope of the admitted resource.

## Pod admission

The `mpod.kb.io`, `vpod.kb.io`, and `vpodeviction.kb.io` webhooks use
`failurePolicy: Fail`. A Kruise webhook outage therefore blocks Pod creation,
update, deletion, and eviction requests covered by those rules.

This preserves the contract of the enabled `PodUnavailableBudgetDeleteGate` and
`PodUnavailableBudgetUpdateGate` features. It also prevents Pods from being
created without SidecarSet, WorkloadSpread, PersistentPodState, or other Kruise
mutations that cannot be applied retroactively after the webhook recovers.

Other bundled webhooks for built-in Deployments, ReplicaSets, StatefulSets,
Namespaces, Services, Ingresses, and CustomResourceDefinitions use
`failurePolicy: Ignore`. API resources not selected by any Kruise webhook remain
available during the outage.

## Kruise custom resources

Webhooks whose rules only target `apps.kruise.io` or `policy.kruise.io` resources
use `failurePolicy: Fail`. Their defaulting and validation are part of the
Kruise resource contract, and an outage therefore blocks changes only to the
affected Kruise APIs instead of blocking general Kubernetes workloads.

The chart regression checks can be run with:

```sh
make verify-chart
```

The Kind E2E workflow also disconnects the Kruise webhook Service temporarily.
It verifies that unrelated core API operations remain available, Pod and Kruise
custom-resource admission remain fail-closed, and a Helm upgrade restores the
Service and Pod admission path. The outage scenario runs after the established
operator E2E suite so it cannot alter that suite's initial state. Run the
integration coverage with `make e2e-kind`.
