# Secrets and ConfigMaps

This provider keeps plaintext secrets out of managed resource specs by using Kubernetes Secret references. The controller resolves the referenced Secrets at reconcile time and sends their key/value payloads to Akuity.

## Provider Credentials

`ProviderConfig.spec.credentialsSecretRef` points to a Secret key containing JSON:

```json
{"apiKeyId":"MY_AKUITY_API_KEY_ID","apiKeySecret":"MY_AKUITY_API_KEY_SECRET"}
```

See [Install and Configure](install-and-configure.md) for the full setup.

## Argo CD Instance Secrets

`Instance` supports these Secret reference fields:

- `argocdSecretRef` for `argocd-secret`.
- `argocdNotificationsSecretRef` for notification secrets.
- `argocdImageUpdaterSecretRef` for registry credentials.
- `applicationSetSecretRef` for ApplicationSet plugin credentials.
- `repoCredentialSecretRefs` for scoped repository credentials.
- `repoTemplateCredentialSecretRefs` for repository template credentials.

Secret payloads are write-only from the Akuity export API. The provider stores a hash in `status.atProvider.secretHash` so local Secret changes trigger an update. Removing a Secret reference stops applying that platform-side Secret, but does not delete it from Akuity.

Missing or empty referenced Secrets fail before the provider writes to Akuity. A later Secret edit rotates the hash and triggers one apply on the next reconcile.

## Kargo Instance Secrets

`KargoInstance` supports:

- `kargoSecretRef` for the `kargo-secret` payload.
- `kargo.oidcConfig.dexConfigSecretRef` for Dex/OIDC secret values.
- `kargoRepoCredentialSecretRefs` for Kargo repository credentials.

`kargoSecretRef` accepts only these keys:

- `adminAccountPasswordHash`
- `admin_account_password_hash`

The controller normalizes the snake_case spelling to `adminAccountPasswordHash` before sending it to Akuity. Unknown keys and conflicting aliases are rejected. If `kargoConfigMap.adminAccountEnabled` is set to `"true"`, provide the admin password hash through `kargoSecretRef` or ensure it already exists in Akuity.

Repository credential entries can set `name`, `projectNamespace`, and `credType`. When omitted, `name` defaults to `secretRef.name`, and `credType` is read from the source Secret label `kargo.akuity.io/cred-type`.

`projectNamespace` is required. It is the destination Kargo project namespace where the synthesized credential Secret is landed on the Akuity control plane; it does not default to the Kubernetes namespace of the source Secret.

Valid Kargo credential types are `git`, `helm`, `generic`, and `image`.

## ConfigMap Semantics

`Instance` ConfigMap fields and `KargoInstance.spec.forProvider.kargoConfigMap` use additive, key-owned semantics. The provider compares only keys present in the managed resource spec:

- If a managed key differs in Akuity, the provider reapplies it.
- Extra keys added by Akuity or another tool are ignored.
- Removing a key from the spec stops managing that key.
- Removing a key from the spec does not delete or clear it in Akuity.

`KargoInstance.kargoConfigMap` is forwarded to the platform without a provider-side key allow-list. Current platform-documented keys include:

- `adminAccountEnabled`
- `adminAccountTokenTtl`

Use the lowerCamel spellings above for currently documented fields; they match the platform's canonical Kargo API config map names. If the platform adds more fields, the provider CRD does not need a release to pass them through.

Known Kargo API aliases such as `admin_account_token_ttl` are canonicalized to the platform's lowerCamel JSON names before apply. The provider also clears the alternate known spelling in the same apply to avoid duplicate-field platform merge state. Removing `kargoConfigMap` from the managed resource stops managing those keys, but does not clear platform-side values.

For `Instance.argocdSshKnownHostsConfigMap.ssh_known_hosts`, keep the Akuity Platform default known_hosts entries and append custom hosts. The full value observed from the platform is exposed at `status.atProvider.argocdSshKnownHostsConfigMap.ssh_known_hosts`, which can be used as the source when refreshing the spec after platform defaults change.
