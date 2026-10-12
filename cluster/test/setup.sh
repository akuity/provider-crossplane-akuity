#!/usr/bin/env bash
set -aeuo pipefail

echo "Running setup.sh"

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
PROVIDER=provider-crossplane-akuity
IN_CLUSTER_SA=provider-crossplane-akuity-in-cluster

# examples/argocd/cluster/in-cluster.yaml installs the agent with the provider
# pod's own credentials, so run the provider as the ServiceAccount that
# examples/provider/in-cluster-rbac.yaml grants the agent install permissions.
echo "Running the provider as the in-cluster agent install ServiceAccount..."
${KUBECTL} apply -f "${REPO_ROOT}/examples/provider/in-cluster-rbac.yaml"
runtime_config=$(${KUBECTL} get provider.pkg "${PROVIDER}" -o jsonpath='{.spec.runtimeConfigRef.name}')
${KUBECTL} patch deploymentruntimeconfig "${runtime_config}" --type merge \
  -p "{\"spec\":{\"deploymentTemplate\":{\"spec\":{\"template\":{\"spec\":{\"serviceAccountName\":\"${IN_CLUSTER_SA}\"}}}}}}"
# Crossplane names the runtime Deployment after the active ProviderRevision.
deployment=$(${KUBECTL} get provider.pkg "${PROVIDER}" -o jsonpath='{.status.currentRevision}')
${KUBECTL} -n crossplane-system wait "deployment/${deployment}" \
  --for=jsonpath='{.spec.template.spec.serviceAccountName}'="${IN_CLUSTER_SA}" --timeout=2m
${KUBECTL} -n crossplane-system rollout status "deployment/${deployment}" --timeout=5m
${KUBECTL} wait provider.pkg "${PROVIDER}" --for condition=Healthy --timeout 5m

if [[ -n "${UPTEST_CLOUD_CREDENTIALS:-}" ]]; then
  : "${UPTEST_AKUITY_ORGANIZATION_ID:?UPTEST_AKUITY_ORGANIZATION_ID must be set together with UPTEST_CLOUD_CREDENTIALS}"

  echo "Creating cloud credential secret..."
  ${KUBECTL} -n crossplane-system create secret generic provider-secret --from-literal=credentials="${UPTEST_CLOUD_CREDENTIALS}" --dry-run=client -o yaml | ${KUBECTL} apply -f -

  # Examples either reference the "akuity" provider config or fall back to the
  # CRD default "default", so both point at the same credentials.
  for name in default akuity; do
    echo "Creating the ${name} provider config..."
    cat <<EOF | ${KUBECTL} apply -f -
apiVersion: akuity.crossplane.io/v1alpha1
kind: ProviderConfig
metadata:
  name: ${name}
spec:
  organizationId: ${UPTEST_AKUITY_ORGANIZATION_ID}
  ${UPTEST_AKUITY_SERVER_URL:+serverUrl: ${UPTEST_AKUITY_SERVER_URL}}
  credentialsSecretRef:
    namespace: crossplane-system
    name: provider-secret
    key: credentials
EOF
  done
fi
