# Instance

`Instance` manages an Akuity Argo CD instance.

## Basic Example

```yaml
apiVersion: core.akuity.crossplane.io/v1alpha1
kind: Instance
metadata:
  name: my-instance
spec:
  forProvider:
    name: my-instance
    argocd:
      spec:
        version: "v3.3.8-ak.87"
  providerConfigRef:
    name: akuity
```

## Common Fields

| Field | Description |
| --- | --- |
| `spec.forProvider.name` | Akuity instance name. Immutable after create. |
| `spec.forProvider.workspace` | Optional workspace ID or name. Omit to use the organization default workspace. |
| `spec.forProvider.argocd.spec.version` | Argo CD version to run. |
| `spec.forProvider.argocd.spec.instanceSpec` | Argo CD feature, extension, customization, and networking settings. |
| `spec.forProvider.configManagementPlugins` | Config Management Plugins v2. |
| `spec.forProvider.resources` | Declarative Argo CD child resources: `Application`, `ApplicationSet`, and `AppProject`. |
| `spec.forProvider.*SecretRef` | References to Kubernetes Secrets whose data is sent to Akuity. |
| `spec.providerConfigRef.name` | Usually `akuity`. |

## Supported Child Resources

`resources` accepts `argoproj.io/v1alpha1` resources of kind `Application`, `ApplicationSet`, and `AppProject`. Inline `v1/Secret` entries are rejected; use typed Secret refs instead.

Child resources are additive. Removing a child from the Crossplane spec stops managing that child, but does not delete it from Akuity. If a namespaced child omits `metadata.namespace`, matching succeeds only when exactly one observed child has the same apiVersion, kind, and name.

## Examples

- [Basic instance](../../examples/instance/basic.yaml)
- [Detailed instance](../../examples/instance/detailed.yaml)
- [Config Management Plugins](../../examples/instance/config-management-plugins.yaml)
- [Declarative Argo CD resources](../../examples/instance/declarative-resources.yaml)

## Notes

ConfigMap fields are key-owned and additive. See [Secrets and ConfigMaps](../guides/secrets-and-configmaps.md).

For the full schema, use [doc.crds.dev](https://doc.crds.dev/github.com/akuityio/provider-crossplane-akuity).
