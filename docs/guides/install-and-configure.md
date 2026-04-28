# Install and Configure

This guide installs the provider, creates the Akuity API credentials Secret, and creates the `ProviderConfig` used by managed resources.

## Prerequisites

- A Kubernetes cluster with Crossplane installed.
- `kubectl` access to the cluster.
- An Akuity API key ID, API key secret, and organization ID.
- Access to the provider package image in `us-docker.pkg.dev/akuity/crossplane/provider`.

## Compatibility

Before upgrading from v0.3.x: the v2 runtime does not support
`publishConnectionDetailsTo` or `StoreConfig`. Migrate any manifests using
ESS-style connection publishing before installing v2.0.0, or the controller
will reject those resources at apply.

Crossplane 1.19.x and 2.x are supported by the same artifact. `safe-start` is
a Crossplane 2.x-only provider capability and no-ops on Crossplane 1.x.

The examples pin versions used during validation. Before applying them to a
production organization, choose Argo CD, Kargo, and agent versions supported by
your Akuity Platform environment.

All provider managed resources opt in to Crossplane `managementPolicies`. Use
`managementPolicies: ["Observe"]` for observe-only adoption and
`deletionPolicy: Orphan` when the Akuity resource should survive managed
resource deletion.

## Install The Provider

Review [examples/provider/provider.yaml](../../examples/provider/provider.yaml), replace the provider package tag, and apply it:

```shell
kubectl apply -f examples/provider/provider.yaml
kubectl get providers.pkg.crossplane.io
kubectl describe providerrevisions.pkg.crossplane.io
```

The `Provider` object installs the CRDs and starts the controller runtime. The example also includes a `DeploymentRuntimeConfig` that enables `--debug`; remove it or remove the `runtimeConfigRef` for production defaults.

## Create Credentials

Create a JSON credentials file:

```json
{"apiKeyId":"MY_AKUITY_API_KEY_ID","apiKeySecret":"MY_AKUITY_API_KEY_SECRET"}
```

Create the Kubernetes Secret in the Crossplane namespace. Either edit
[examples/provider/credentials-secret.yaml](../../examples/provider/credentials-secret.yaml)
and apply it, or create the Secret from a local credentials file:

```shell
kubectl -n crossplane-system create secret generic akuity-provider-secret \
  --from-file=credentials=./akuity-credentials.json
```

The Secret key must match `spec.credentialsSecretRef.key` in [examples/provider/config.yaml](../../examples/provider/config.yaml).

## Configure The Provider

Edit [examples/provider/config.yaml](../../examples/provider/config.yaml):

- Set `spec.organizationId` to the Akuity organization ID.
- Keep `spec.credentialsSecretRef` pointed at the credentials Secret.
- Set `spec.serverUrl` only for non-default Akuity API endpoints.
- Use `spec.skipTlsVerify` only for local or test environments.

Apply the ProviderConfig:

```shell
kubectl apply -f examples/provider/config.yaml
kubectl get providerconfig.akuity.crossplane.io akuity
```

## Create Managed Resources

Start with a basic Argo CD instance:

```shell
kubectl apply -f examples/instance/basic.yaml
kubectl get instances.core.akuity.crossplane.io
kubectl describe instance.core.akuity.crossplane.io my-instance
```

Then attach resources that depend on the instance status, such as `Cluster` or `InstanceIpAllowList`, after the parent reports a populated `status.atProvider.id`.

For resources that install agents into a target Kubernetes cluster, configure
only one kubeconfig source: `kubeconfigSecretRef` or
`enableInClusterKubeconfig`. Generated agent manifests are applied during
create, not during later updates. See
[Lifecycle and Reconciliation](lifecycle-and-reconciliation.md).

When using `enableInClusterKubeconfig: true`, the provider pod's own
ServiceAccount must be able to apply the generated agent manifests. Use a
customer-managed ServiceAccount bound out-of-band and point the provider
runtime at it with `deploymentTemplate.spec.template.spec.serviceAccountName`.
This avoids binding permissions to Crossplane's generated, revision-specific
provider ServiceAccount names during upgrades. See
[In-cluster agent install RBAC](../../examples/cluster/in-cluster-rbac.yaml)
for the required ServiceAccount, ClusterRole, ClusterRoleBinding, and
`DeploymentRuntimeConfig` pattern. Reference that runtime config from the
Provider with `spec.runtimeConfigRef.name: akuity-in-cluster`. Without this
RBAC, `Cluster` create fails with Kubernetes `Forbidden` errors and the
managed resource remains `Synced=False`.

This guidance applies only to `enableInClusterKubeconfig: true`; with
`kubeconfigSecretRef`, equivalent permissions are needed on the identity inside
the referenced kubeconfig.

## Upgrade Or Remove

Upgrade by changing `spec.package` on the `Provider` object to the target image tag. Crossplane creates a new `ProviderRevision` and activates it according to the package revision policy.

Remove managed resources before removing the provider. Deleting the provider first removes the controllers and can leave external Akuity resources unmanaged.
