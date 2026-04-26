# KargoAgent

`KargoAgent` manages a Kargo agent attached to a Kargo instance.

## Basic Example

```yaml
apiVersion: core.akuity.crossplane.io/v1alpha1
kind: KargoAgent
metadata:
  name: my-kargo-agent
spec:
  forProvider:
    kargoInstanceRef:
      name: my-kargo
    name: my-kargo-agent
    kargoAgentSpec:
      data:
        size: small
  providerConfigRef:
    name: akuity
```

## Common Fields

| Field | Description |
| --- | --- |
| `spec.forProvider.kargoInstanceRef.name` | References a `KargoInstance` managed by Crossplane. |
| `spec.forProvider.kargoInstanceId` | Direct Akuity Kargo instance ID. Use instead of `kargoInstanceRef`. |
| `spec.forProvider.workspace` | Workspace ID or name. Inherits from the referenced Kargo instance when omitted with `kargoInstanceRef`. |
| `spec.forProvider.name` | Agent name. Immutable after create. |
| `spec.forProvider.namespace` | Namespace where the agent is installed. |
| `spec.forProvider.kargoAgentSpec` | Agent payload for description, size, mode, autoscaling, remote Argo CD, and install customizations. |
| `spec.forProvider.kubeconfigSecretRef` | Secret containing a kubeconfig under the `kubeconfig` key. |
| `spec.forProvider.enableInClusterKubeconfig` | Use provider pod in-cluster config to install manifests. |
| `spec.forProvider.removeAgentResourcesOnDestroy` | Remove installed agent manifests during delete. |

`kargoInstanceId` and `kargoInstanceRef` are immutable. `kargoAgentSpec.data.akuityManaged` is immutable after create because the Akuity API ignores updates to that field.

## Examples

- [Basic agent](../../examples/kargoagent/basic.yaml)
- [Detailed agent](../../examples/kargoagent/detailed.yaml)
- [Autoscaled agent](../../examples/kargoagent/autoscaler.yaml)
- [Akuity-managed agent](../../examples/kargoagent/akuity-managed.yaml)
- [Self-hosted agent](../../examples/kargoagent/self-hosted.yaml)

For the full schema, use [doc.crds.dev](https://doc.crds.dev/github.com/akuityio/provider-crossplane-akuity).
