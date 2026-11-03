# Python UDF Operator integration: supported stage

Revision: 2026-09-20. Scope: explicitly enabled, trusted same-Pod Python worker.
This records the stage authorized by the project owner for implementation and
review; it is not production isolation approval. The broader architecture remains
subject to its Runtime, Catalog, CNPool and security rollout gates.

Depends on [MatrixOne PR #29152](https://github.com/matrixorigin/matrixone/pull/29152).
Related to [MatrixOne issue #28132](https://github.com/matrixorigin/matrixone/issues/28132).

## Problem and decision

The old demo sidecar cannot execute the new typed Catalog/SDK/Arrow/Flight
contract. Silently retaining its configuration would imply compatibility that
neither the CN nor worker provides. The Operator must explicitly enable the new
runtime, reject incompatible configuration, and preserve ordinary SQL and the
platform's network authorization boundaries.

Use one separate worker container per enabled CN Pod. The CN contains its own
Gateway library and connects directly to the worker's Flight server on loopback.
Flight is not a third daemon. Each invocation remains owned by the CN/runtime;
the Operator never admits, acknowledges or replays individual invocation work.

Alternatives considered:

| Option | Decision |
| --- | --- |
| Keep the demo sidecar/adapter | Reject: no supported old demo execution or automatic migration. Recreate functions with new DDL. |
| Same-Pod separate Python container | Implement this stage: stable loopback binding fits random CloneSet Pod identities and a shared CNSet ConfigMap. |
| External paired worker | Defer: needs stable CN identity/discovery and route ownership before it can preserve instance affinity. |
| Shared worker pool behind a random Service | Reject in this stage: connection balancing does not implement invocation lease affinity or scheduling. |
| Python inside CN | Not adopted: changes the process and failure boundary. |

## Public API and admission

`CNSetSpec.udfWorker` is the effective typed policy. MatrixOneCluster propagation,
direct CNSet writes and `CNPoolSpec.template` use the same validation contract.
Nil policy is disabled; explicit disabled policy is presence-aware. Conflicting
policy sources are rejected rather than merged into an implicit configuration.
CNPool requires a registered, deployed validating webhook, not only a helper.
Controllers revalidate effective configuration when reconciling stored objects.

Enabled policy requires `paired`, `python-image`, `allowUnisolated: true`, an
immutable image digest, and CPU/memory limits. The image supplies Python,
PyArrow, the matching timezone data, worker code and `/usr/bin/tini`.
The rendered worker runs with UID/GID 1000. Digest syntax validation is not
image-content authentication; image provenance remains the deployer's responsibility.
Unsupported topology/launcher combinations and every explicitly supplied legacy
demo sidecar field (including disabled or empty) are rejected. Image digests identify artifacts; compatibility is determined by
the CN/worker contract, not by equal software version strings.

The Operator owns the worker command, loopback bind, security context and status
metadata. Reachable overlays cannot inject additional containers, share PID
namespace, override worker code/command, expose host ports or supply privileged
Pod identity. Controlled fields are validated at all effective policy boundaries.
The worker is injected after general overlay processing so the overlay cannot
silently remove it. Resource limits bound the container, not arbitrary per-call
Python heap. Same-Pod scheduling, service account and volumes are shared failure
and trust boundaries, not tenant isolation.

## Execution, status and lifecycle

The authoritative command retains init/reaping:
`/usr/bin/tini -- python -u worker.py --address=grpc://127.0.0.1:<port>`.
The CN client uses the same Pod-local port. There is no worker Service or
arbitrary command/argument overlay. The worker handles bounded handler processes;
Operator configuration does not turn hard-coded handler capabilities into a
new deployment policy.

CN client configuration owns K, batch/invocation bounds, timeout and ledger
limits. Current W is 1 and cumulative ACK is disabled. Defaults come from the
matching runtime; zero values select defaults rather than unlimited resources.
Worker and Gateway ledger capacities/TTLs need to be compatible, not equal.

Per-Pod observation follows:

```text
CN store controller -> that CN's query service -> Gateway capability handshake
                    <- bounded status snapshot <- worker Flight capabilities
```

The status wire bridge validates size, fields and stable errors. Observations are
bound to Pod UID, CN identity, rendered policy generation and worker lease. An
old Pod must not be stamped with desired-generation readiness. Cluster aggregation
also fences each child against the UID and metadata generation returned by its
reconciliation write, the current policy hash and current condition observations.
Missing/stale children contribute no ready workers and make the aggregate
incomplete/degraded; the cluster status generation denotes the desired policy. Missing, stale,
invalid or incompatible observations fail Python readiness; they are not cached
as success for a replacement Pod. Controller query/observation errors stay
distinct from valid CN-reported runtime failure classes.

Status strings, lists and annotations have explicit bounds. Controller ownership
and Kubernetes resource versions govern updates; the Operator does not copy
source, arguments or unbounded traceback into status. Observability is not a
security admission token: the CN validates the execution contract again.

Capability readiness is not added to Pod readiness, and the worker receives no
capability readiness/liveness probe. A stalled worker can require whole-Pod
replacement. Actual worker CrashLoop, OOM and eviction can still remove the CN
Pod from SQL service endpoints. This is the accepted same-Pod availability cost.
Worker/client changes enter the CloneSet-managed CN Pod rollout, which may use
in-place container updates. This does not promise a new Pod UID or CN process
for every worker-only image change; cross-claim reclaim below does require both.

Python-enabled CN Pods cannot transfer warm CN/Gateway/worker state across claim
owners. Normal reclaim, migration, overlapping claim finalization and legacy
reclaim shortcuts must delete the old Pod instead of returning it Idle or merely
changing owner labels. Pool revision retirement is controller-owned CNSet
metadata projected onto Pods after overlays, so admission cannot reject normal
retirement as user-supplied lifecycle state. Markers are controller-owned; policy/container evidence
prevents marker removal from authorizing reuse. A new owner receives a fresh
Pod UID. Failed/ambiguous ownership evidence fails closed.

The runtime owns cancel, drain, terminal records and no-replay of STARTED work.
Tini reaps adopted descendants while Python owns direct child waits. Kubernetes
owns Pod replacement. The Operator neither retries user work nor transfers a
runtime lease to another worker after failure.

## Networking and upgrade

The current network decision supersedes the earlier blanket proposal to add a
CN allowlist whenever Python is enabled. Kubernetes grants are additive; a new
allowlist can widen existing authorization or isolate previously working CN
traffic, including Gossip UDP. Loopback-only binding is the current cross-Pod
worker boundary; it does not defend against CN code, Pod exec or privileged nodes.

The Operator preserves platform policies. A legacy broad policy owned by this
CNSet UID is converted only after a platform ingress baseline covers stable,
rendered and existing Pod selectors and the required numeric port/protocol tuples.
It becomes a zero-grant anchor retained until all CN Pods, including terminating
and CN-only Pods, disappear. A platform policy disappearing later must not make
the last isolation policy disappear. Unowned same-name objects are preserved and not migrated. Missing coverage for
an owned legacy object blocks migration; the controller never invents source
authorization.

See [networking](python-udf-networking.md) for the exact migration contract and
platform peer responsibilities, including CN/TN SQL TCP/6001 and Gossip UDP/6005.

Install CRDs/webhooks and compatible CN readers/executors before opt-in. A worker
or timezone/environment mismatch rejects Python before handler execution;
ordinary SQL remains available subject to the shared-Pod failure caveat above.
Restoring compatible images can restore existing definitions; do not delete or
rewrite Catalog data to conceal an environment mismatch. Disable/reenable must
preserve existing function identity/revision. Deploying older Operators that do
not understand the new policy is not an automatic downgrade/migration path.

## Evidence and rollout boundaries

Operator `cd4cff5`, MO `0d4640af38` (executable code `37889f08b7`) were tested in
single-node kind with Calico/Kruise, two CN Pods and real MinIO. The test matrix
included installed admission (34 cases), Python SQL BVT (352 cases on each CN),
million-row scalar/vector correctness, cancellation, worker death, descendant
reaping, environment mismatch/rollback, enable/disable, owner reclaim/new UID,
Operator outage/recovery, real TCP/UDP networking and policy finalization.

Four concurrent million-row queries exceeded owner handler capacity; that is not
a successful concurrency benchmark. A barrier-based four-slot test proved fifth
request rejection and recovery after releasing the slots. Roughly 51 seconds for
an individual million-row query is an observation, not a production SLA.

The test runtime image is CI-compatible rather than the official production
image. Later MO revisions require semantic evidence reconciliation. Multi-node
failure, explicit OOM/eviction, long-running ledger capacity, external/pool
workers, sandbox and authenticated transport remain outside this test claim.
Do not mark this stage as production-ready solely because its functional tests
pass. Keep the dependency PR and matching distributable-image validation visible
before merging or enabling this integration.

## Design review record

The project owner authorized this staged implementation; this does not stand in
for the broader production owner signoffs. This stage derives from the local
Reviewed R8 design and its 2026-09-20 network-ownership amendment (source content
SHA256 `e661dd0e16512bf51ffe161c8753f0d016637ec1fc8eec2b9ee273062d99ff79`).
The repository copy here is the reviewable delivery contract for this stage.

A separate GPT-6 medium design review (`gpt-6-astra`, medium, session
`01a0bdea-62fc-7fb0-9a29-e39e7b8a85a5`) accepted the stage before these delivery
changes, against Operator base `10aef15` and implementation `cd4cff5`. It required
this versioned artifact, explicit dependency/evidence boundaries, and the
repository-mandated copyright updates. Final implementation review is separate
from that design decision.
