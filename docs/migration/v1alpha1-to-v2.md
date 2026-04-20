# Migrating from `provider-crossplane-akuity` v1.x to v2.0.0

This guide covers the upgrade from the v1.x provider (Crossplane v1 runtime,
`core.akuity.crossplane.io/v1alpha1` managed resources) to v2.0.0 (Crossplane
v2 runtime, `core.m.akuity.crossplane.io/v1alpha2` namespaced managed
resources). The v1alpha1 API surface is kept for a 6-month deprecation
window; both versions can be used side-by-side during the migration. The
legacy group is scheduled for removal in **v3.0.0**.

---

## 1. Compatibility matrix

| Component | v1.x | v2.0.0 |
|---|---|---|
| Crossplane runtime | v1.20 | v2.2+ |
| controller-runtime | v0.21 | v0.23+ |
| MR groups | `core.akuity.crossplane.io/v1alpha1` | `core.m.akuity.crossplane.io/v1alpha2` (active) + `core.akuity.crossplane.io/v1alpha1` (deprecated, kept for 6 months) |
| MR scope | cluster-wide | namespaced |
| ProviderConfig | `akuity.crossplane.io/v1alpha1` (cluster-wide) | `akuity.m.crossplane.io/v1alpha2` `ProviderConfig` (namespaced) + `ClusterProviderConfig` (cluster-wide) |
| ESS / `StoreConfig` / `publishConnectionDetailsTo` | supported | **removed** (Crossplane v2 physically deletes the ESS types) |
| Agent install | inline in the Cluster controller | opt-in via a Composition wiring `provider-kubernetes` |
| MR activation | always on | `safe-start` + `ManagedResourceActivationPolicy` (shipped in-package) |

**One-way changes.** The items below cannot be rolled back by downgrading the
provider package — plan accordingly.
- ESS is removed at the runtime layer.
- `checkClusterReconciled` is removed: Cluster `Create` returns immediately;
  install manifests become available on the connection secret once the
  Akuity API reports terminal reconciliation status. External scripts that
  used Create-blocking behaviour as an implicit readiness signal need to
  switch to `kubectl wait --for=condition=Ready cluster/<name>`.
- `ClusterAgent` is dropped. Agent install is now a Composition flow (see
  §5).

---

## 2. Upgrade order of operations

1. **Capture ESS dependencies** (if any). Before upgrading the provider,
   list every Akuity MR using `publishConnectionDetailsTo` or `StoreConfig`:
   ```bash
   kubectl get cluster,instance -A -o json |
     jq -r '.items[] | select(.spec.publishConnectionDetailsTo or .spec.storeConfigRef) | "\(.kind)/\(.metadata.name)"'
   ```
   For each hit, pick an ESS alternative (§4) and prepare the replacement
   manifests. **Do not upgrade the provider before the replacements are
   ready** — the v2 runtime removes the ESS types, so any remaining ESS
   references fail to reconcile with a compile-level error inside the
   runtime.

2. **Upgrade `crossplane` itself to v2.2+** following the upstream guide.
   v1.x providers still reconcile under the v2 runtime, so the Akuity
   provider can be upgraded on its own schedule.

3. **Upgrade the Akuity provider package to v2.0.0.** The package ships
   both API groups and two `ManagedResourceActivationPolicy` manifests
   under `package/mrap/` that activate every v1alpha1 + v1alpha2 MR out of
   the box. Existing v1alpha1 CRs keep reconciling.

4. **Migrate each v1alpha1 CR to its v1alpha2 equivalent** (§3). Migrate at
   your own pace inside the 6-month window.

5. **(Optional, recommended)** Remove the legacy MRAP at
   `package/mrap/legacy-v1alpha1.yaml` once all v1alpha1 CRs are gone. The
   legacy MRDs go Inactive, reconcile stops, and the upcoming v3.0.0
   cleanup has nothing left to evict.

---

## 3. Per-resource migration

### 3.1 `Cluster`

v1alpha1:
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

v1alpha2 (namespaced; same-namespace InstanceRef only):
```yaml
apiVersion: core.m.akuity.crossplane.io/v1alpha2
kind: Cluster
metadata:
  name: my-cluster
  namespace: default
spec:
  forProvider:
    name: my-cluster
    instanceRef:
      name: my-instance
  providerConfigRef:
    kind: ProviderConfig
    name: akuity
  writeConnectionSecretToRef:
    name: my-cluster-manifests
```

Removed fields — migration path for each:
| v1alpha1 field | v1alpha2 replacement |
|---|---|
| `kubeConfigSecretRef` | N/A — Cluster no longer installs the agent. Use `provider-kubernetes` in a Composition (§5). |
| `enableInClusterKubeConfig` | N/A — same as above. |
| `removeAgentResourcesOnDestroy` | Same — `provider-kubernetes` Object deletion removes the agent resources it applied. |
| `spec.reapplyManifestsOnUpdate` (implicit) | `provider-kubernetes` honours this natively. |

### 3.2 `Instance`

The spec shape is largely identical. Three changes:

- `apiVersion` → `core.m.akuity.crossplane.io/v1alpha2`
- Add `metadata.namespace`.
- `providerConfigRef` adds a `kind` field (`ProviderConfig` or
  `ClusterProviderConfig`).

### 3.3 New v1alpha2-only resources

- `KargoInstance` — manages a Kargo instance.
- `KargoAgent` — manages a Kargo agent and publishes agent-install manifests
  on its connection secret, same pattern as Cluster.
- `KargoDefaultShardAgent` — narrow-patch resource that sets
  `kargoInstanceSpec.defaultShardAgent` on a target Kargo instance via the
  Akuity `PatchKargoInstance` endpoint. Specify the target with either
  `kargoInstanceId` (opaque Akuity ID, primary path) or `kargoInstanceRef`
  (name of a sibling `KargoInstance` MR in the same namespace, resolved
  through its `Status.AtProvider.ID`). Mutually exclusive.
- `InstanceIpAllowList` — narrow-patch resource for `spec.ipAllowList` on a
  target ArgoCD Instance via the Akuity `PatchInstance` endpoint. Same
  ID-or-Ref shape as `KargoDefaultShardAgent`; pair with an Instance that
  deliberately omits `ipAllowList`.

Ready-to-use manifests live under `examples/v1alpha2/`.

### 3.4 Why narrow-patch resources key by ID

Both `InstanceIpAllowList` and `KargoDefaultShardAgent` call
server-side-merging `Patch*` endpoints that require the opaque Akuity ID
of the target. Two call paths are supported:

- **Direct**: set `instanceId` / `kargoInstanceId` on the MR. No
  cross-resource resolution; no kube client reads beyond the MR itself.
- **Indirect** (recommended for GitOps flows): set `instanceRef` /
  `kargoInstanceRef` pointing at a sibling `Instance` / `KargoInstance`
  MR in the same namespace. The controller reads the sibling's
  `Status.AtProvider.ID` at reconcile time. If the sibling has not yet
  reported an ID, the narrow-patch reconcile errors out with a
  "waiting for its controller to observe" message and requeues.

The indirect path matches the Akuity Terraform provider's
`akp_instance_ip_allow_list` / `akp_kargo_default_shard_agent` resources
one-for-one, modulo Crossplane's ref-based composition idiom.

---

## 4. ESS → external-secrets alternatives

Crossplane v2 physically removes the ESS types (`xpv1.SecretStoreConfig`,
`xpv1.PublishConnectionDetailsTo`, `pkg/connection`). The provider ships no
migration shim — once the upgrade is applied, any MR manifest that still
carries `publishConnectionDetailsTo` fails admission. Pick one of the
following replacements and stage it **before** the upgrade:

- **[external-secrets-operator](https://external-secrets.io)** (recommended
  for most users). Install ESO, provision a `SecretStore` against the same
  backend the old `StoreConfig` pointed at, and create an `ExternalSecret`
  that targets the Crossplane-generated connection secret. Data replication
  to the original destination is then ESO's job.
- **[crossplane-contrib/provider-vault](https://github.com/crossplane-contrib/provider-vault)**
  when the backing store is Vault and you prefer a Crossplane-native flow.
  Compose a `VaultSecret` alongside the Akuity MR.
- **[crossplane-contrib/provider-sops](https://github.com/crossplane-contrib/provider-sops)**
  for GitOps flows with encrypted-at-rest secrets. Works unchanged across
  the v2 upgrade because it doesn't rely on the removed ESS APIs.

Removal timing: ESS support disappears the moment the provider is upgraded
to v2.0.0. Migration must happen in the same window as the upgrade, not at
the v3.0.0 calendar gate.

---

## 5. Cluster/Kargo agent install via `provider-kubernetes`

v1.x bundled a cluster-scoped kube-apply client inside the Cluster
controller; it dynamically applied the Akuity agent install YAML onto a
target cluster. v2 drops that path entirely in favour of
`provider-kubernetes`:

1. Cluster's connection secret carries the install YAML on a single key
   named `manifests`.
2. A Composition downstream fetches those manifests and feeds them to a
   `kubernetes.crossplane.io/v1alpha1.Object` pointing at the managed
   cluster's `ProviderConfig`.

A ready-to-adapt Composition lives at
[`examples/v1alpha2/cluster-with-agent-composition.yaml`](../../examples/v1alpha2/cluster-with-agent-composition.yaml).
The KargoAgent controller publishes the same `manifests` key and slots into
the same pattern.

Why the split: the old design hard-coded a per-kind whitelist inside the
provider; any new agent-install kind required a provider release. The
Composition flow hands that decision to operators.

---

## 6. Upgrade-day checklist

- [ ] Enumerated ESS-dependent MRs and staged replacement manifests.
- [ ] Communicated a freeze on creating new v1alpha1 CRs during the
      migration window (optional but avoids double-bookkeeping).
- [ ] Upgraded Crossplane core to v2.2+ and confirmed MRDs / MRAP CRDs are
      installed on the control cluster.
- [ ] Upgraded this provider to v2.0.0; `kubectl get managedresourcedefinition`
      shows every Akuity MRD with `ACTIVATED=true` after the shipped MRAPs
      apply.
- [ ] Existing v1alpha1 CRs continue to reconcile and emit the deprecation
      Warning event and the `akuity_legacy_v1alpha1_cr_count` gauge is
      visible on the provider's metrics endpoint.

---

## 7. Timeline

- **v2.0.0 (day 0)**: both API groups active; deprecation warnings begin;
  legacy-CR gauge starts reporting.
- **v2.x (months 1–6)**: bug/security fixes only for the legacy path; all
  feature work targets v1alpha2.
- **v3.0.0 (day +180)**: legacy group removed. Delete
  `package/mrap/legacy-v1alpha1.yaml` at this point if it is still present.

Calendar deadline is the only gate. The `akuity_legacy_v1alpha1_cr_count`
gauge informs release-communications timing, not the removal itself.
