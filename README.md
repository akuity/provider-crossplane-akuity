# provider-akuity

The `provider-akuity` repository is the Crossplane infrastructure provider for
[Akuity Platform](https://akuity.io/akuity-platform/). It installs Kubernetes
CRDs and controllers that let Crossplane manage Akuity resources from managed
resource specs.

## Documentation

Start with [docs/index.md](./docs/index.md). The docs include provider setup,
resource pages, and focused guides for secrets, ConfigMaps, Config Management
Plugins, and declarative Kargo resources.

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
[doc.crds.dev/github.com/akuityio/provider-crossplane-akuity](https://doc.crds.dev/github.com/akuityio/provider-crossplane-akuity).

## Compatibility

- Crossplane 1.19+ and 2.x are supported by the same artifact.
- Existing `v1alpha1` manifests continue to work unchanged.
- `spec.crossplane.version: ">=v1.19.0"` is declared in
  [package/crossplane.yaml](./package/crossplane.yaml).
- ESS runtime fields (`publishConnectionDetailsTo`, `StoreConfig`) are not
  supported by the runtime-v2 provider. Use external secret tooling such as
  external-secrets-operator, provider-vault, or provider-sops.
- Before upgrading from v0.3.x: the v2 runtime does not support
  `publishConnectionDetailsTo` or `StoreConfig`. Migrate any manifests using
  ESS-style connection publishing before installing v2.0.0, or the controller
  will reject those resources at apply.

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
open an [issue](https://github.com/akuityio/provider-crossplane-akuity/issues).
