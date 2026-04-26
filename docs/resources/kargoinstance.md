# KargoInstance

`KargoInstance` manages an Akuity Kargo instance.

## Basic Example

```yaml
apiVersion: core.akuity.crossplane.io/v1alpha1
kind: KargoInstance
metadata:
  name: my-kargo
spec:
  forProvider:
    name: my-kargo
    kargo:
      version: "v1.8.1"
  providerConfigRef:
    name: akuity
```

## Common Fields

| Field | Description |
| --- | --- |
| `spec.forProvider.name` | Akuity Kargo instance name. Immutable after create. |
| `spec.forProvider.workspace` | Optional workspace ID or name. Omit to use the organization default workspace. |
| `spec.forProvider.kargo.version` | Kargo version. |
| `spec.forProvider.kargo.description` | Instance description. |
| `spec.forProvider.kargo.fqdn` | Custom FQDN. Use either `fqdn` or `subdomain` according to platform rules. |
| `spec.forProvider.kargo.subdomain` | Akuity-managed subdomain. |
| `spec.forProvider.kargo.oidcConfig` | OIDC and Dex settings. Prefer `dexConfigSecretRef` for secret data. |
| `spec.forProvider.kargo.kargoInstanceSpec` | Instance features such as allow list, default shard agent, AI, GC, and global namespaces. |
| `spec.forProvider.kargoConfigMap` | Managed keys for `kargo-cm`. |
| `spec.forProvider.kargoSecretRef` | Secret data sent as `kargo-secret`. |
| `spec.forProvider.kargoRepoCredentialSecretRefs` | Kargo repository credentials from Kubernetes Secret refs. |
| `spec.forProvider.resources` | Declarative Kargo child resources. |

## Declarative Resources

`resources` supports Kargo `Project`, `Warehouse`, `Stage`, `AnalysisTemplate`, `PromotionTask`, and `ClusterPromotionTask` resources. See [Managing Declarative Kargo Resources](../guides/managing-kargo-resources.md).

## Examples

- [Basic Kargo instance](../../examples/kargoinstance/basic.yaml)
- [Detailed Kargo instance](../../examples/kargoinstance/detailed.yaml)
- [Declarative Kargo resources](../../examples/kargoinstance/resources.yaml)
- [OIDC with Secret refs](../../examples/kargoinstance/oidc-secret-ref.yaml)

For the full schema, use [doc.crds.dev](https://doc.crds.dev/github.com/akuityio/provider-crossplane-akuity).
