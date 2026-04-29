# Akuity Crossplane Provider Documentation

`provider-crossplane-akuity` installs Crossplane managed resources for Akuity Platform. Use it when a Crossplane control plane should own Akuity Argo CD instances, attached clusters, Kargo instances, Kargo agents, and related control-plane settings.

## Quick Links

- [Install and configure the provider](guides/install-and-configure.md)
- [Secret and ConfigMap handling](guides/secrets-and-configmaps.md)
- [Lifecycle and reconciliation behavior](guides/lifecycle-and-reconciliation.md)
- [Managing Argo CD Config Management Plugins](guides/managing-cmps.md)
- [Managing declarative Kargo resources](guides/managing-kargo-resources.md)
- [Full CRD reference on doc.crds.dev](https://doc.crds.dev/github.com/akuity/provider-crossplane-akuity)

## Resource Documentation

| Resource | Purpose | Examples |
| --- | --- | --- |
| [ProviderConfig](resources/providerconfig.md) | Akuity API authentication and organization routing. | [examples/provider](../examples/provider) |
| [Instance](resources/instance.md) | Manages an Akuity Argo CD instance. | [examples/instance](../examples/instance) |
| [Cluster](resources/cluster.md) | Attaches a Kubernetes cluster to an Argo CD instance. | [examples/cluster](../examples/cluster) |
| [InstanceIpAllowList](resources/instanceipallowlist.md) | Owns the standalone Argo CD instance IP allow list. | [examples/instanceipallowlist](../examples/instanceipallowlist) |
| [KargoInstance](resources/kargoinstance.md) | Manages an Akuity Kargo instance. | [examples/kargoinstance](../examples/kargoinstance) |
| [KargoAgent](resources/kargoagent.md) | Manages a Kargo agent attached to a Kargo instance. | [examples/kargoagent](../examples/kargoagent) |
| [KargoDefaultShardAgent](resources/kargodefaultshardagent.md) | Pins the default shard agent for a Kargo instance. | [examples/kargodefaultshardagent](../examples/kargodefaultshardagent) |

## Crossplane Notes

All managed resources in `core.akuity.crossplane.io/v1alpha1` are cluster-scoped. Their `providerConfigRef.name` normally points to the cluster-scoped `ProviderConfig` named `akuity`.

Use the [Lifecycle and Reconciliation](guides/lifecycle-and-reconciliation.md) guide for:

- Observe-only resources with Crossplane `managementPolicies`.
- `deletionPolicy: Orphan`.
- Additive child resources and ConfigMap keys.
- Create-time agent manifest installs.

The examples under `examples/` are Kubernetes manifests. Crossplane packages can include examples from this directory when building the provider package with the Crossplane CLI examples root.
