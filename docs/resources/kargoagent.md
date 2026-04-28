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

`kargoInstanceId` and `kargoInstanceRef` are immutable. Set one in new manifests. Existing both-set resources are accepted for upgrade compatibility and the controller resolves the reference first.

`kubeconfigSecretRef` and `enableInClusterKubeconfig` are mutually exclusive. When either is set, generated agent manifests are applied during create. Updates to the `KargoAgent` managed resource do not reapply generated manifests to the target cluster.

`kargoAgentSpec.data.akuityManaged` is immutable after create because the Akuity API ignores updates to that field.

## Akuity-Managed And Self-Hosted Agents

For Akuity-managed agents, set `akuityManaged: true` and `remoteArgocd` to the resolved Akuity Argo CD instance ID from `Instance.status.atProvider.id`, not the Crossplane `Instance` managed resource name. Do not set self-hosted customization fields such as `size`, `targetVersion`, `kustomization`, `argocdNamespace`, `selfManagedArgocdUrl`, `allowedJobSa`, or `autoscalerConfig`; the platform owns or rejects those fields for Akuity-managed agents.

For self-hosted agents, set `akuityManaged: false`, provide the self-managed Argo CD connection details, and use agent versions such as `0.5.88` without a leading `v`. Autoscaler bounds are intended for `size: auto`.

## Examples

- [Basic agent](../../examples/kargoagent/basic.yaml)
- [Detailed agent](../../examples/kargoagent/detailed.yaml)
- [Autoscaled agent](../../examples/kargoagent/autoscaler.yaml)
- [Akuity-managed agent](../../examples/kargoagent/akuity-managed.yaml)
- [Self-hosted agent](../../examples/kargoagent/self-hosted.yaml)

For the full schema, use [doc.crds.dev](https://doc.crds.dev/github.com/akuityio/provider-crossplane-akuity).
