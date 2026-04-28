# Lifecycle and Reconciliation

This provider reconciles Akuity resources through Crossplane managed resources. The sections below describe what is continuously reconciled, what is applied once, and what is intentionally additive.

## Crossplane Controls

All managed resources in `core.akuity.crossplane.io/v1alpha1` opt in to Crossplane `managementPolicies`.

- Use the default policies when Crossplane should create, update, observe, and delete the Akuity resource.
- Use `managementPolicies: ["Observe"]` to watch an existing Akuity resource without applying changes.
- Use `deletionPolicy: Orphan` when deleting the Kubernetes managed resource should leave the Akuity resource in place.

`ProviderConfigUsage` cleanup follows Crossplane runtime behavior. Remove dependent managed resources before deleting the provider if you want an orderly teardown.

## Parent References

Child resources that belong to an Akuity parent usually accept either a direct Akuity ID or a Crossplane reference:

- `Cluster`: `instanceId` or `instanceRef`.
- `InstanceIpAllowList`: `instanceId` or `instanceRef`.
- `KargoAgent`: `kargoInstanceId` or `kargoInstanceRef`.
- `KargoDefaultShardAgent`: `kargoInstanceId` or `kargoInstanceRef`.

New manifests should set exactly one of the two fields. Existing stored resources that contain both fields are still accepted for upgrade compatibility; the controller resolves the managed-resource reference first and falls back to the direct ID.

`Instance`, `KargoInstance`, and `KargoAgent` accept workspace IDs or workspace names. When the field is omitted on workspace-scoped resources, the provider resolves the organization default workspace and reports the canonical workspace ID in status when it is observable.

## Agent Manifest Installs

`Cluster` and `KargoAgent` can apply Akuity-generated install manifests to a target Kubernetes cluster when one of these kubeconfig sources is configured:

- `kubeconfigSecretRef`
- `enableInClusterKubeconfig`

The install is a create-time operation. The provider applies the manifests before it records the external name. If the platform row is created but the manifest install fails, the provider rolls the platform row back and records a terminal error for the failed spec.

Updates to the managed resource do not reapply generated manifests to the target cluster. If the target cluster needs a fresh generated manifest set, recreate the managed resource or apply the generated manifests through your cluster deployment workflow.

`removeAgentResourcesOnDestroy` removes installed manifests during delete only when a kubeconfig source is configured.

## Additive Payloads

Several platform payloads are intentionally additive:

- `Instance.spec.forProvider.resources` manages `Application`, `ApplicationSet`, and `AppProject` children without pruning remote children omitted from the spec.
- `KargoInstance.spec.forProvider.resources` manages supported Kargo children without pruning remote children omitted from the spec.
- ConfigMap fields are key-owned. Removing a key from the spec stops managing that key, but does not delete the key from Akuity.
- Removing a Secret reference stops applying that platform-side Secret, but does not delete it from Akuity.

Delete unwanted platform-side children, ConfigMap keys, or credential Secrets through the Akuity UI or API.

## Platform-Owned Fields

Some fields are normalized to platform-observed values because the Akuity API owns the effective value:

- Cluster `autoscalerConfig` is reconciled for `clusterSpec.data.size: "auto"`. For `small`, `medium`, and `large`, the platform stamps size-profile defaults and the provider treats those observed values as authoritative.
- Akuity-managed Kargo agents do not accept self-hosted customization fields such as `size`, `targetVersion`, `kustomization`, `allowedJobSa`, `autoscalerConfig`, or self-hosted Argo CD settings. Use `akuityManaged: false` for self-hosted agents.
- Kargo agent maintenance mode is routed through a dedicated maintenance RPC rather than the normal apply payload.

Prefer the focused examples in `examples/` for field combinations that match the validated platform behavior.
