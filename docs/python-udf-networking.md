# Same-Pod Python UDF networking

Python enablement must preserve the platform's existing CN ingress policy. The
Operator does not create an additional CN ingress allowlist. Kubernetes
NetworkPolicy grants are additive: even a restricted new allowlist can widen
an existing policy, and isolating a previously non-isolated CN can block UDP
Gossip or application-specific traffic. A policy omitting port 50051 cannot
deny a grant made by another policy.

The supported unisolated same-Pod adapter instead enforces its worker listener
as `grpc://127.0.0.1:<port>` in the authoritative container command. It does not
expose a worker Service, hostPort or hostNetwork, and the existing overlay
restrictions remain mandatory. This protects against cross-Pod network access,
not against CN code, Pod exec, privileged nodes or a sandbox escape. External
workers remain outside this profile and require their own authenticated and
restricted network design.

The worker image must include `/usr/bin/tini`. The Operator's authoritative
command runs `tini -- python -u worker.py`; overriding the image entrypoint must
not discard this init process. Python waits for direct handler/watchdog children,
while init adopts and reaps orphaned descendants after a handler leader exits.
Process-group termination alone does not release zombie PID entries. Existing
worker images without init must be replaced before using this renderer.

On upgrade an obsolete deterministic Python policy controlled by this CNSet UID
is retired conservatively. Before removing its grants, a platform ingress
baseline must select both the stable CNSet labels and the fully rendered future
Pod labels, as well as current Pods, and cover the numeric service/protocol tuples
below. Negative selector expressions and named-port-only coverage are not
accepted as proof. Missing coverage produces `UDFNetworkPolicyMigrationRequired`;
the controller does not invent replacement sources or grants.

Once covered, the legacy object becomes a **zero-grant isolation anchor**. It
remains across Python enable/disable and later platform-policy deletion, avoiding
the last-policy deletion transition to unrestricted ingress. Only workload
finalization after **all CN Pods**, including CN-only and terminating Pods, have
disappeared removes it. Platform policies and unowned same-name collisions are
never changed. This migration check proves destination selection and protocol
coverage; the platform remains responsible for authorizing the actual CN/TN,
SQL-client and monitoring peers. It does not certify connectivity from those peers.

A platform policy should explicitly describe its own trusted peers and clients.
CN service requirements include pipeline TCP/6002, lock TCP/6003, query TCP/6004,
Gossip **TCP and UDP/6005**, shard TCP/6006, SQL TCP/6001 and metrics TCP/7001
(subject to the rendered configuration). An omitted protocol means TCP, not
both TCP and UDP. Python enable/disable must neither add grants to these ports
nor start selecting previously non-isolated CN Pods.

Validation uses fresh connections under both non-isolated and restricted
platform policies; trusted traffic, untrusted denial and Gossip behavior must
be unchanged when Python is toggled. Unit rendering tests alone are not CNI
verification.

Run the CNI contract probe against an explicitly selected disposable cluster:

```sh
python3 hack/test-udf-network-policy.py --kubeconfig /path/to/kubeconfig \
  --image registry/worker@sha256:<digest>
```

The image must provide `python`. The probe creates and removes its own namespace,
uses fresh TCP/UDP sockets, reproduces the legacy additive grant, and checks the
zero-grant anchor after platform-policy deletion. Its CN endpoint is an echo
fixture: passing proves CNI behavior, not actual CN Gossip membership or an
end-to-end Operator rollout. The Go controller tests cover migration, source
preservation, selectors, ownership and finalization separately.

Reference: [Kubernetes Network Policies](https://kubernetes.io/docs/concepts/services-networking/network-policies/).
