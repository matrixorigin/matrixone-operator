# Run an E2E Operator safely in a shared cluster

Do not replace the image of a shared cluster-level MatrixOne Operator to test a
development build. Commands such as the following make the test binary manage
every MatrixOne resource watched by the shared deployment:

```bash
kubectl -n matrixone set image deployment/<shared-operator> \
  matrixone-operator=<test-image>
```

When the development build renders a different ConfigMap, OpenKruise can
restart LogService, DN/TN, CN, and Proxy containers in place. Existing database
connections can be interrupted even though their MatrixOne cluster is in a
different namespace.

## Isolated installation

Use the guarded installer for development and E2E validation in a shared
Kubernetes cluster:

```bash
./hack/install-isolated-operator.sh \
  --namespace mo-python-udf-e2e-20260917 \
  --release python-udf-e2e \
  --repository ccr.ccs.tencentyun.com/matrixone-dev/matrixone-operator \
  --tag udf-e2e-20260917
```

The installer:

- refuses shared namespaces such as `matrixone`, `mo-system`, and `default`;
- accepts only namespace names with an `e2e` segment;
- refuses an existing namespace unless it already has the isolation label;
- enables `onlyWatchReleasedNS=true`, restricting the controller cache;
- adds a namespace selector to every Operator admission webhook;
- disables installation of another cluster-level Kruise controller; and
- uses a separate Helm release instead of modifying the shared deployment.

New namespaces are labelled automatically:

```text
matrixorigin.io/operator-e2e=true
app.kubernetes.io/managed-by=matrixone-operator-e2e
```

For a pre-created namespace, apply the labels deliberately before running the
installer:

```bash
kubectl label namespace <e2e-namespace> \
  matrixorigin.io/operator-e2e=true \
  app.kubernetes.io/managed-by=matrixone-operator-e2e \
  --overwrite
```

The namespace-scoped Operator must only manage MatrixOne custom resources in
that namespace. Do not use this procedure when the test requires changing
cluster-wide CRDs incompatibly; use a dedicated Kubernetes cluster for that
case.
