// Package secrets resolves kube Secret references referenced by
// managed resource specs. Sensitive payloads destined for the Akuity
// gateway (argocd-secret, notifications-secret, repo-credential-secrets,
// etc.) are carried as xpv1.LocalSecretReference on the MR spec rather
// than inlined plaintext; this package fans those references out into
// the map[string]string payloads that the gateway Apply requests expect.
package secrets

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/akuityio/provider-crossplane-akuity/apis/core/v1alpha1"
)

// ErrMissingSecret is returned when a referenced Secret does not exist.
// Controllers should treat this as a terminal configuration error and
// surface it to the user without exponential retry.
var ErrMissingSecret = errors.New("referenced secret not found")

// ResolveAllKeys loads the Secret at ref (within ns) and returns a copy
// of its data as map[string]string. Returns nil with no error when ref
// is nil, so callers can compose it without pre-checks. If the Secret
// does not exist, ErrMissingSecret is returned wrapped with the secret
// name for diagnostics.
func ResolveAllKeys(ctx context.Context, c client.Client, ns string, ref *xpv1.LocalSecretReference) (map[string]string, error) {
	if ref == nil || ref.Name == "" {
		return nil, nil
	}
	sec := &corev1.Secret{}
	if err := c.Get(ctx, types.NamespacedName{Namespace: ns, Name: ref.Name}, sec); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, fmt.Errorf("%w: %s/%s", ErrMissingSecret, ns, ref.Name)
		}
		return nil, fmt.Errorf("get secret %s/%s: %w", ns, ref.Name, err)
	}
	return bytesToStringMap(sec.Data), nil
}

// ResolveNamed loads each referenced Secret and returns a map keyed by
// the caller-provided Name field to the resolved key/value data. Empty
// or nil refs yield a nil map to let callers skip setting the wire
// field. Duplicate Names error out — they would silently clobber each
// other in the proto map otherwise.
func ResolveNamed(ctx context.Context, c client.Client, ns string, refs []v1alpha1.NamedLocalSecretReference) (map[string]map[string]string, error) {
	if len(refs) == 0 {
		return nil, nil
	}
	out := make(map[string]map[string]string, len(refs))
	for i := range refs {
		name := refs[i].Name
		if _, dup := out[name]; dup {
			return nil, fmt.Errorf("duplicate secret reference name %q", name)
		}
		data, err := ResolveAllKeys(ctx, c, ns, &refs[i].SecretRef)
		if err != nil {
			return nil, fmt.Errorf("resolve %q: %w", name, err)
		}
		out[name] = data
	}
	return out, nil
}

// Hash returns a stable SHA256 of a resolved secret map. Keys are
// iterated in sorted order so two maps with the same content always
// produce the same digest. The output is suitable for drift detection
// via an annotation on the managed resource: when the user rotates
// the backing Secret, the digest changes and the reconciler observes
// ResourceUpToDate=false.
func Hash(data map[string]string) string {
	if len(data) == 0 {
		return ""
	}
	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	h := sha256.New()
	for _, k := range keys {
		h.Write([]byte(k))
		h.Write([]byte{0})
		h.Write([]byte(data[k]))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

// HashNamed returns a stable SHA256 spanning every entry in a
// ResolveNamed result. The outer keys and each nested Secret's contents
// contribute to the digest so either a rename of a credential slot or a
// rotation of an underlying Secret invalidates the cached hash.
func HashNamed(data map[string]map[string]string) string {
	if len(data) == 0 {
		return ""
	}
	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	h := sha256.New()
	for _, k := range keys {
		h.Write([]byte(k))
		h.Write([]byte{0})
		h.Write([]byte(Hash(data[k])))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

func bytesToStringMap(in map[string][]byte) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = string(v)
	}
	return out
}
