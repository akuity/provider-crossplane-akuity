# Cluster

`Cluster` attaches a Kubernetes cluster to an Akuity Argo CD instance.

## Basic Example

```yaml
apiVersion: core.akuity.crossplane.io/v1alpha1
kind: Cluster
metadata:
  name: my-cluster
spec:
  forProvider:
    instanceRef:
      name: my-instance
    name: my-cluster
  providerConfigRef:
    name: akuity
```

## Common Fields

| Field | Description |
| --- | --- |
| `spec.forProvider.instanceRef.name` | References an `Instance` managed by Crossplane. |
| `spec.forProvider.instanceId` | Direct Akuity instance ID. Use instead of `instanceRef`. |
| `spec.forProvider.name` | Cluster name. Immutable after create. |
| `spec.forProvider.namespace` | Namespace where the Akuity agent is installed. |
| `spec.forProvider.clusterSpec` | Gateway payload for description, namespace scope, size, autoscaling, and agent settings. |
| `spec.forProvider.kubeconfigSecretRef` | Secret containing a kubeconfig under the `kubeconfig` key. |
| `spec.forProvider.enableInClusterKubeconfig` | Use the provider pod in-cluster config to install the agent. |
| `spec.forProvider.removeAgentResourcesOnDestroy` | Remove agent manifests when deleting the cluster. |

`instanceId` and `instanceRef` are immutable. Set one in new manifests. If both exist from an older stored resource, the controller resolves `instanceRef` first.

`kubeconfigSecretRef` and `enableInClusterKubeconfig` are mutually exclusive. When either is set, generated agent manifests are applied during create. Updates to the `Cluster` managed resource do not reapply generated manifests to the target cluster.

For `enableInClusterKubeconfig: true`, install the provider with a stable,
customer-managed ServiceAccount that has permission to apply the generated
agent resources in the same cluster. Do not bind permissions to Crossplane's
generated ProviderRevision ServiceAccount names, because those names rotate
during provider upgrades. See
[In-cluster agent install RBAC](../../examples/cluster/in-cluster-rbac.yaml)
and set the Provider's `spec.runtimeConfigRef.name` to `akuity-in-cluster`.
Without this RBAC, `Cluster` create fails with Kubernetes `Forbidden` errors
and the managed resource remains `Synced=False`.

## Agent Sizing

Use `clusterSpec.data.size: small`, `medium`, `large`, `custom`, or `auto` according to the target platform behavior.

For `auto`, set `autoscalerConfig` and keep `autoUpgradeDisabled: false`. This is the size mode where the provider continuously reconciles user-provided autoscaler bounds.

For `small`, `medium`, and `large`, the platform stamps size-profile autoscaler defaults. If `autoscalerConfig` is present for those sizes, the provider treats the platform-observed values as authoritative to avoid drift.

## Examples

- [Basic cluster](../../examples/cluster/basic.yaml)
- [Detailed cluster](../../examples/cluster/detailed.yaml)
- [Auto agent size](../../examples/cluster/auto-agent-size.yaml)
- [Custom agent size](../../examples/cluster/custom-agent-size.yaml)
- [In-cluster agent install](../../examples/cluster/in-cluster.yaml)
- [In-cluster agent install RBAC](../../examples/cluster/in-cluster-rbac.yaml)

For the full schema, use [doc.crds.dev](https://doc.crds.dev/github.com/akuityio/provider-crossplane-akuity).
