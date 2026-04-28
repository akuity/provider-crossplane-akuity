# ProviderConfig

`ProviderConfig` configures Akuity API access for every managed resource that references it.

```yaml
apiVersion: akuity.crossplane.io/v1alpha1
kind: ProviderConfig
metadata:
  name: akuity
spec:
  organizationId: REPLACE_ME
  credentialsSecretRef:
    namespace: crossplane-system
    name: akuity-provider-secret
    key: credentials
```

## Fields

| Field | Required | Description |
| --- | --- | --- |
| `spec.organizationId` | Yes | Akuity organization ID used for all API calls. |
| `spec.credentialsSecretRef` | Yes | Secret key containing JSON with `apiKeyId` and `apiKeySecret`. |
| `spec.serverUrl` | No | Akuity Platform API URL. Defaults to `https://akuity.cloud`. |
| `spec.skipTlsVerify` | No | Skips TLS verification. Use only for local or test endpoints. |

## Examples

- [Provider install](../../examples/provider/provider.yaml)
- [Credentials Secret](../../examples/provider/credentials-secret.yaml)
- [ProviderConfig](../../examples/provider/config.yaml)
