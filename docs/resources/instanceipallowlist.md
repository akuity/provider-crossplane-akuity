# InstanceIpAllowList

`InstanceIpAllowList` owns the standalone IP allow list for an Argo CD instance. This is preferred over the older inline `argocd.spec.instanceSpec.ipAllowList` field when the allow list should have its own Crossplane resource lifecycle.

## Example

```yaml
apiVersion: core.akuity.crossplane.io/v1alpha1
kind: InstanceIpAllowList
metadata:
  name: my-instance-allowlist
spec:
  forProvider:
    instanceRef:
      name: my-instance
    allowList:
      - ip: "10.0.0.0/24"
        description: "office vpn"
      - ip: "1.2.3.4"
        description: "bastion"
  providerConfigRef:
    name: akuity
```

## Fields

| Field | Description |
| --- | --- |
| `spec.forProvider.instanceRef.name` | References an `Instance` managed by Crossplane. |
| `spec.forProvider.instanceId` | Direct Akuity instance ID. Use instead of `instanceRef`. |
| `spec.forProvider.allowList` | IP address or CIDR entries to enforce. |

`instanceId` and `instanceRef` are immutable. The controller caches the resolved instance ID in status so delete can clear the remote allow list even if the parent `Instance` resource has already been removed.

## Examples

- [Basic allow list](../../examples/instanceipallowlist/basic.yaml)

For the full schema, use [doc.crds.dev](https://doc.crds.dev/github.com/akuityio/provider-crossplane-akuity).
