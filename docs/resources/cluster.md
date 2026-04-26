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

`instanceId` and `instanceRef` are immutable. Set one. If both exist from an older stored resource, the controller resolves `instanceRef` first.

## Agent Sizing

Use `clusterSpec.data.size: small`, `medium`, `large`, `custom`, or `auto` according to the target platform behavior. For `auto`, set `autoscalerConfig` and keep `autoUpgradeDisabled: false`.

## Examples

- [Basic cluster](../../examples/cluster/basic.yaml)
- [Detailed cluster](../../examples/cluster/detailed.yaml)
- [Auto agent size](../../examples/cluster/auto-agent-size.yaml)
- [Custom agent size](../../examples/cluster/custom-agent-size.yaml)
- [In-cluster agent install](../../examples/cluster/in-cluster.yaml)

For the full schema, use [doc.crds.dev](https://doc.crds.dev/github.com/akuityio/provider-crossplane-akuity).
