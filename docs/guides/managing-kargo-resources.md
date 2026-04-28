# Managing Declarative Kargo Resources

`KargoInstance.spec.forProvider.resources` can send declarative Kargo child resources with the instance apply request. This is useful when the Crossplane resource should bootstrap Projects, Warehouses, Stages, and promotion tasks alongside the Kargo instance.

Use [examples/kargoinstance/resources.yaml](../../examples/kargoinstance/resources.yaml) as the focused example.

## Supported Resources

Entries must use one of these apiVersion/kind pairs:

| apiVersion | Kinds |
| --- | --- |
| `kargo.akuity.io/v1alpha1` | `Project`, `Warehouse`, `Stage`, `PromotionTask`, `ClusterPromotionTask` |
| `argoproj.io/v1alpha1` | `AnalysisTemplate` |

`AnalysisTemplate` belongs to Argo Rollouts, so it must use the `argoproj.io/v1alpha1` API group. Using `kargo.akuity.io/v1alpha1` for an `AnalysisTemplate` is rejected by the Akuity gateway.

Inline `v1/Secret` entries are rejected. Use `kargoRepoCredentialSecretRefs` for repository credentials so plaintext stays out of the managed resource spec.

The provider supports a typed subset of Kargo resources. ConfigMaps, RBAC objects, and repository credential Secrets should be managed through dedicated fields or through your normal Kargo bootstrap workflow.

## Additive Semantics

The provider checks that every child resource listed in `spec.forProvider.resources` exists remotely with equivalent content. It does not prune remote resources that are omitted locally.

Removing an entry from the list stops managing that child resource from Crossplane. Delete unwanted child resources through the Akuity UI or API.

If a manifest omits `metadata.namespace`, the provider can match an existing observed child only when exactly one remote child has the same apiVersion, kind, and name. Include `metadata.namespace` for namespaced resources when possible.

## Example Shape

```yaml
resources:
  - apiVersion: kargo.akuity.io/v1alpha1
    kind: Project
    metadata:
      name: kargo-demo
  - apiVersion: kargo.akuity.io/v1alpha1
    kind: Warehouse
    metadata:
      name: images
      namespace: kargo-demo
    spec:
      subscriptions:
        - image:
            repoURL: public.ecr.aws/nginx/nginx
            semverConstraint: ^1.28.0
            discoveryLimit: 5
  - apiVersion: argoproj.io/v1alpha1
    kind: AnalysisTemplate
    metadata:
      name: smoke
      namespace: kargo-demo
    spec:
      metrics:
        - name: smoke
          provider:
            web:
              url: https://example.com/healthz
```
