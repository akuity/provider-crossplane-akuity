# provider-akuity

The `provider-akuity` repository is the Crossplane infrastructure provider for
[Akuity Platform](https://akuity.io/akuity-platform/). The provider that is built from the source code
in this repository can be installed into a Crossplane control plane and adds the 
following new functionality:

* Custom Resource Definitions (CRDs) that model Akuity services(e.g, Argo CD instances and clusters)
* Controllers to provision these resources in Akuity Cloud based on users 
desired state captured in the CRDs they create

## Installation

### Prerequisites

- [kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl/)
- [Crossplane](https://docs.crossplane.io/latest/software/install/)

### Configure an API Key
The Akuity Crossplane provider needs to be configured with Akuity API credentials. Please check how to create an API key on  [Akuity Platform API Key Documentation](https://docs.akuity.io/organizations/api-keys). 

Once you have an API key and secret, create a JSON file with the following contents (replace the placeholders for API key 
ID and secret with the credentials generated above):

```
{"apiKeyId": "MY_AKUITY_API_KEY_ID", "apiKeySecret": "MY_AKUITY_API_KEY_SECRET"}
```

Next, base64 encode the above content before pasting it in the Kubernetes `Secret` in [examples/provider/provider.yaml](./examples/provider/provider.yaml):

```
cat myfile.json | base64
```

Install the provider by applying the [examples/provider/provider.yaml](./examples/provider/provider.yaml) in your cluster:

```
kubectl apply -f examples/provider/provider.yaml
```

### Configure the Organization ID
Once the provider is ready, update the `ProviderConfig` resource in [examples/provider/config.yaml](./examples/provider/config.yaml) with your
Akuity organization ID. Your organization ID can be found by logging in to [https://akuity.cloud](https://akuity.cloud) and
navigating to your organization page. The organization ID is displayed on the top right corner of the screen.

Once you have completed the steps above to replace the placeholders for API credentials, organization ID in
[examples/provider/config.yaml](./examples/provider/config.yaml), you can configure the provider by applying the `ProviderConfig` resources to your cluster:

```
kubectl apply -f examples/provider/config.yaml
```

Now you can start managing Akuity instances and clusters using Crossplane.

### API surface: `core.akuity.crossplane.io/v1alpha1` (cluster-scoped)

All managed resources are cluster-scoped and live in the
`core.akuity.crossplane.io/v1alpha1` group. Existing `v1alpha1` manifests
continue to work unchanged — the upgrade is additive.

- [Argo CD Instance](./examples/instance/basic.yaml)
- [Cluster](./examples/cluster/basic.yaml)
- [Argo CD Instance — detailed](./examples/instance/detailed.yaml)
- [Cluster — detailed](./examples/cluster/detailed.yaml)

v2.0.0 adds four new cluster-scoped MR types under the same group:
`KargoInstance`, `KargoAgent`, `KargoDefaultShardAgent`,
`InstanceIpAllowList`. See the CRD reference for their schemas.

### Compatibility

- **Crossplane core**: Crossplane 1.19+ and 2.x both supported out of
  the same artifact. `spec.crossplane.version: ">=v1.19.0"` is
  declared in `package/crossplane.yaml` so older 1.x cores will refuse
  to install the provider instead of loading it against an untested
  runtime. safe-start is a 2.x-only capability and no-ops on 1.x.
- **Existing v1alpha1 manifests**: work unchanged.
- **ESS (`publishConnectionDetailsTo`, `StoreConfig`)**: removed at the
  runtime layer in v2.0.0. If you previously used these fields, migrate
  to [external-secrets-operator](https://external-secrets.io),
  `crossplane-contrib/provider-vault`, or `crossplane-contrib/provider-sops`
  before upgrading.

For all supported fields for Akuity resources, please check [https://doc.crds.dev/github.com/akuity/provider-crossplane-akuity](https://doc.crds.dev/github.com/akuity/provider-crossplane-akuity/).

**Note** - Instance and KargoInstance secret payloads are managed through
namespaced Kubernetes `Secret` references so plaintext stays out of managed
resource specs. Akuity export APIs do not return secret values, so the provider
uses a hash of the local source `Secret` as its drift signal and reapplies only
when the provider-side desired source changes. Platform-side edits or deletions
are not detected, and removing a ref from the managed resource stops applying
that platform secret but does not delete it from Akuity. Empty referenced
Secrets are treated as terminal configuration errors because the platform apply
APIs treat omitted or empty secret payloads as "no opinion", not as remote
delete/clear requests.

**Note** - Instance and KargoInstance ConfigMap fields are additive and
key-owned. The provider compares only keys present in the managed resource
spec, ignores platform-added/default keys, and reapplies when a managed key
differs. Removing a ConfigMap key from the spec stops managing that key; it does
not delete or clear the key from Akuity.

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
