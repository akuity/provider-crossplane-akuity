# KargoDefaultShardAgent

`KargoDefaultShardAgent` pins the default shard agent for a Kargo instance.

## Example

```yaml
apiVersion: core.akuity.crossplane.io/v1alpha1
kind: KargoDefaultShardAgent
metadata:
  name: my-kargo-default-shard-agent
spec:
  forProvider:
    kargoInstanceRef:
      name: my-kargo
    agentName: my-kargo-agent
  providerConfigRef:
    name: akuity
```

## Fields

| Field | Description |
| --- | --- |
| `spec.forProvider.kargoInstanceRef.name` | References a `KargoInstance` managed by Crossplane. |
| `spec.forProvider.kargoInstanceId` | Direct Akuity Kargo instance ID. Use instead of `kargoInstanceRef`. |
| `spec.forProvider.agentName` | Name of the agent to use as the default shard agent. |

`kargoInstanceId` and `kargoInstanceRef` are immutable. Set one in new manifests. Existing both-set resources are accepted for upgrade compatibility and the controller resolves the reference first.

The controller caches the resolved Kargo instance ID in status so delete can clear the remote default shard setting even if the parent has already been removed.

## Examples

- [Basic default shard agent](../../examples/kargodefaultshardagent/basic.yaml)

For the full schema, use [doc.crds.dev](https://doc.crds.dev/github.com/akuity/provider-crossplane-akuity).
