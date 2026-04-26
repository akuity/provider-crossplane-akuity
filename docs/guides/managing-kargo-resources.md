# Managing Declarative Kargo Resources

`KargoInstance.spec.forProvider.resources` can send declarative Kargo child resources with the instance apply request. This is useful when the Crossplane resource should bootstrap Projects, Warehouses, Stages, and promotion tasks alongside the Kargo instance.

Use [examples/kargoinstance/resources.yaml](../../examples/kargoinstance/resources.yaml) as the focused example.

## Supported Resources

Entries must use `apiVersion: kargo.akuity.io/v1alpha1` and one of these kinds:

- `Project`
- `Warehouse`
- `Stage`
- `AnalysisTemplate`
- `PromotionTask`
- `ClusterPromotionTask`

Inline `v1/Secret` entries are rejected. Use `kargoRepoCredentialSecretRefs` for repository credentials so plaintext stays out of the managed resource spec.

## Additive Semantics

The provider checks that every child resource listed in `spec.forProvider.resources` exists remotely with equivalent content. It does not prune remote resources that are omitted locally.

Removing an entry from the list stops managing that child resource from Crossplane. Delete unwanted child resources through the Akuity UI or API.

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
```
