#!/usr/bin/env bash
# Rewrites in-repo relative links inside the marketplace-extensions tree
# to absolute https://github.com/akuity/provider-crossplane-akuity URLs
# pinned at the release tag. Source markdown stays single-rooted in the
# repo; only the assembled extensions copy gets rewritten.
#
# Args:
#   $1  extensions root (e.g. _output/extensions)
#   $2  git ref to pin against (release tag, branch, or sha)

set -euo pipefail

ext_root=${1:?usage: $0 <extensions-root> <ref>}
ref=${2:?usage: $0 <extensions-root> <ref>}

base="https://github.com/akuity/provider-crossplane-akuity"

# In-place sed wrapper that works on both BSD and GNU sed.
sed_inplace() {
  local script=$1
  shift
  if sed --version >/dev/null 2>&1; then
    sed -i -E "$script" "$@"
  else
    sed -i '' -E "$script" "$@"
  fi
}

readme="$ext_root/readme/readme.md"
if [[ -f "$readme" ]]; then
  # Bare directory refs (./examples/<dir>) → tree/.
  sed_inplace "s#\\]\\(\\./examples/([a-zA-Z0-9_-]+)\\)#](${base}/tree/${ref}/examples/\\1)#g" "$readme"
  sed_inplace "s#\\]\\(\\./apis/([a-zA-Z0-9/_.-]+)\\)#](${base}/tree/${ref}/apis/\\1)#g" "$readme"
  # File refs → blob/.
  sed_inplace "s#\\]\\(\\./examples/([a-zA-Z0-9/_.-]+)\\)#](${base}/blob/${ref}/examples/\\1)#g" "$readme"
  sed_inplace "s#\\]\\(\\./docs/([a-zA-Z0-9/_.-]+)\\)#](${base}/blob/${ref}/docs/\\1)#g" "$readme"
  sed_inplace "s#\\]\\(\\./package/([a-zA-Z0-9/_.-]+)\\)#](${base}/blob/${ref}/package/\\1)#g" "$readme"
fi

# docs/index.md: links cross out of docs/ to ../examples/<dir> (bare dir).
# docs/resources/*.md and docs/guides/*.md: ../../examples/<dir>/file.yaml.
# Intra-docs links (../guides/foo.md, guides/foo.md, resources/foo.md) keep
# the relative form so the docs/ subtree renders coherently when marketplace
# serves it as a unit.
while IFS= read -r -d '' f; do
  sed_inplace "s#\\]\\(\\.\\./\\.\\./examples/([a-zA-Z0-9/_.-]+)\\)#](${base}/blob/${ref}/examples/\\1)#g" "$f"
  sed_inplace "s#\\]\\(\\.\\./examples/([a-zA-Z0-9_-]+)\\)#](${base}/tree/${ref}/examples/\\1)#g" "$f"
done < <(find "$ext_root/docs" -name '*.md' -print0)
