# provider-crossplane-akuity

The `provider-crossplane-akuity` repository is the Crossplane infrastructure
provider for [Akuity Platform](https://akuity.io/akuity-platform/). It installs
Kubernetes CRDs and controllers that let Crossplane manage Akuity resources
from managed resource specs.

## Documentation

Start with [docs/index.md](./docs/index.md). The docs include provider setup,
resource pages, and focused guides for secrets, ConfigMaps, Config Management
Plugins, declarative resources, and reconciliation behavior.

## Quick Start

Install Crossplane, replace `REPLACE_WITH_VERSION` in
[examples/provider/provider.yaml](./examples/provider/provider.yaml), then
install the provider:

```shell
kubectl apply -f examples/provider/provider.yaml
kubectl get providers.pkg.crossplane.io
```

Create the provider credentials Secret. Either edit and apply
[examples/provider/credentials-secret.yaml](./examples/provider/credentials-secret.yaml),
or create it from a local credentials file. The `credentials` key must contain
JSON with `apiKeyId` and `apiKeySecret`:

```shell
kubectl -n crossplane-system create secret generic akuity-provider-secret \
  --from-file=credentials=./akuity-credentials.json
```

Edit [examples/provider/config.yaml](./examples/provider/config.yaml) with your
Akuity organization ID, then apply it:

```shell
kubectl apply -f examples/provider/config.yaml
```

Create an Argo CD instance:

```shell
kubectl apply -f examples/instance/basic.yaml
kubectl get instances.core.akuity.crossplane.io
```

## Managed Resources

All managed resources are cluster-scoped and live in
`core.akuity.crossplane.io/v1alpha1`.

| Resource | Purpose | Examples |
| --- | --- | --- |
| `Instance` | Akuity Argo CD instance. | [examples/instance](./examples/instance) |
| `Cluster` | Kubernetes cluster attached to an Argo CD instance. | [examples/cluster](./examples/cluster) |
| `InstanceIpAllowList` | Standalone Argo CD instance IP allow list. | [examples/instanceipallowlist](./examples/instanceipallowlist) |
| `KargoInstance` | Akuity Kargo instance. | [examples/kargoinstance](./examples/kargoinstance) |
| `KargoAgent` | Kargo agent attached to a Kargo instance. | [examples/kargoagent](./examples/kargoagent) |
| `KargoDefaultShardAgent` | Default shard agent binding for a Kargo instance. | [examples/kargodefaultshardagent](./examples/kargodefaultshardagent) |

For the full CRD schema, use
[doc.crds.dev/github.com/akuity/provider-crossplane-akuity](https://doc.crds.dev/github.com/akuity/provider-crossplane-akuity).

## Crossplane Compatibility

- Crossplane 1.19.x and 2.x are supported by the same provider artifact.
- `spec.crossplane.version: ">=v1.19.0"` is declared in
  [package/crossplane.yaml](./package/crossplane.yaml).
- The provider declares the `safe-start` capability (Crossplane 2.x); 1.x
  cores ignore it as a no-op.
- `v1alpha1` manifests authored against earlier provider releases continue to
  work without edits.

## Provider Behavior

- **`managementPolicies` (beta):** every managed resource accepts the
  Crossplane `spec.managementPolicies` list, which scopes the operations the
  controller may perform on the external Akuity resource. `["*"]` (default)
  allows the full create/update/delete loop; `["Observe"]` makes the resource
  read-only — the provider syncs `.status.atProvider` and never writes to the
  Akuity API. Use it to import an existing Akuity resource without risk of
  drift-correction. See
  [Lifecycle and Reconciliation](./docs/guides/lifecycle-and-reconciliation.md)
  for the full policy matrix.
- **`deletionPolicy: Orphan`** leaves the Akuity-side resource in place when
  the Kubernetes managed resource is deleted.
- **Agent installs are one-shot.** `Cluster` and `KargoAgent` apply
  Akuity-generated install manifests during create when a kubeconfig source
  is configured. Updates do not reapply those manifests; recreate the MR or
  reapply manually.
- **External Secret Stores (ESS) is not supported.** The runtime-v2 provider
  rejects `publishConnectionDetailsTo` and `StoreConfig`. Use
  external-secrets-operator, provider-vault, or provider-sops instead.

## Upgrading from v0.3.x

- The v2 runtime does not support ESS connection publishing. Migrate any
  manifests using `publishConnectionDetailsTo` or `StoreConfig` before
  installing v2.0.0; the controller rejects them at apply.
- Reapplying existing v1alpha1 `Instance`, `Cluster`, `KargoInstance`,
  `KargoAgent`, `KargoDefaultShardAgent`, and `InstanceIpAllowList` manifests
  is sufficient; no schema changes affect the upgrade path.

## Local Development

### Crossplane

For general Crossplane getting started guides, installation, deployment, and administration, see
the Crossplane [Documentation](https://crossplane.io/docs).

### Initialize Git Submodules

Crossplane provides a build submodule that is used for most of the Makefile targets. To initialize the
submodule run:

`git submodule update --init`

### Generating the Akuity resource CRDs

To generate the CRDs used by Crossplane for Akuity resources:

`make generate`

The CRDs are generated from the types defined in [apis/core/v1alpha1/](./apis/core/v1alpha1/).

### Running the provider locally

To run the Crossplane provider locally using a Kind cluster and apply the CRDs generated above:

`make dev`

The above command should only be run once. It will error if a Kind cluster already exists.

If you need to make changes to the CRDs, you can regenerate them and apply them to the already
running cluster:

```
make generate
kubectl apply -f package/crds/
```

If you need to make changes to the provider Go code, you can Ctrl-C to quit the binary
started with `make dev` and start it again to include your changes:

`make run`

### Testing

- `make test` — unit, converter round-trip, drift-helper, and reason-classifier suites.
- `make test-envtest` — boots a real Kubernetes apiserver via `envtest` and validates the generated CRD CEL rules end-to-end (instance-id vs instance-ref, immutability, at-least-one). Requires the `envtest` assets managed by `setup-envtest`; the Makefile target installs them on first run.
- `make lint` — golangci-lint (v2.11.4 pin).

## Report a Bug

For filing bugs, suggesting improvements, or requesting new features, please
open an [issue](https://github.com/akuity/provider-crossplane-akuity/issues).
