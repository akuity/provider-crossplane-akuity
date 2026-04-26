// Package secrets resolves kube Secret references referenced by
// managed resource specs. Sensitive payloads destined for the Akuity
// gateway (argocd-secret, notifications-secret, repo-credential-secrets,
// etc.) are carried as namespaced Secret references on the MR spec
// rather than inlined plaintext; this package fans those references out
// into the map[string]string payloads that the gateway Apply requests
// expect.
package secrets

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"sort"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/akuityio/provider-crossplane-akuity/apis/core/v1alpha1"
	"github.com/akuityio/provider-crossplane-akuity/internal/reason"
)

// ErrMissingSecret is returned when a referenced Secret does not exist.
// Controllers should treat this as a terminal configuration error and
// surface it to the user without exponential retry.
var ErrMissingSecret = errors.New("referenced secret not found")

// ErrInvalidSecretReference is returned when a non-nil Secret reference
// omits name or namespace.
var ErrInvalidSecretReference = errors.New("invalid secret reference")

// ErrEmptySecret is returned when a referenced Secret exists but carries
// no data. Empty source Secrets are rejected because the platform apply
// endpoints treat omitted/empty secret payloads as "no opinion", not as
// a remote clear/delete.
var ErrEmptySecret = errors.New("referenced secret has no data")

var repoCredentialNameRE = regexp.MustCompile(`^repo-[a-z0-9][a-z0-9-]*$`)

// IsConfigError reports whether err is caused by invalid user-supplied
// Secret reference configuration rather than a transient kube API
// failure.
func IsConfigError(err error) bool {
	return errors.Is(err, ErrMissingSecret) ||
		errors.Is(err, ErrInvalidSecretReference) ||
		errors.Is(err, ErrEmptySecret)
}

// AsTerminalIfConfig wraps user-supplied Secret configuration errors as
// terminal reconciliation errors. Transient kube API failures are
// returned unchanged.
func AsTerminalIfConfig(err error) error {
	if IsConfigError(err) {
		return reason.AsTerminal(err)
	}
	return err
}

// ResolvedSecret is a Secret reference plus the Secret's copied data.
type ResolvedSecret struct {
	Namespace string
	Name      string
	Labels    map[string]string
	Data      map[string]string
}

// Hash returns a stable digest spanning both the source Secret identity
// and its resolved data. A namespace/name change with identical data
// still rotates the digest.
func (r ResolvedSecret) Hash() string {
	if r.Namespace == "" && r.Name == "" && len(r.Data) == 0 {
		return ""
	}
	return Hash(map[string]string{
		"__namespace__": r.Namespace,
		"__name__":      r.Name,
		"__data__":      Hash(r.Data),
	})
}

// Resolve loads the Secret at ref and returns a copy of its data plus
// the source identity. Returns the zero value with no error when ref is
// nil, so callers can compose it without pre-checks. If the Secret does
// not exist, ErrMissingSecret is returned wrapped with the secret name
// for diagnostics.
func Resolve(ctx context.Context, c client.Client, ref *xpv1.SecretReference) (ResolvedSecret, error) {
	if ref == nil {
		return ResolvedSecret{}, nil
	}
	return resolve(ctx, c, ref.Namespace, ref.Name)
}

func resolve(ctx context.Context, c client.Client, namespace, name string) (ResolvedSecret, error) {
	if name == "" || namespace == "" {
		return ResolvedSecret{}, fmt.Errorf("%w: both name and namespace are required", ErrInvalidSecretReference)
	}
	sec := &corev1.Secret{}
	if err := c.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, sec); err != nil {
		if apierrors.IsNotFound(err) {
			return ResolvedSecret{}, fmt.Errorf("%w: %s/%s", ErrMissingSecret, namespace, name)
		}
		return ResolvedSecret{}, fmt.Errorf("get secret %s/%s: %w", namespace, name, err)
	}
	data := bytesToStringMap(sec.Data)
	if len(data) == 0 {
		return ResolvedSecret{}, fmt.Errorf("%w: %s/%s", ErrEmptySecret, namespace, name)
	}
	return ResolvedSecret{Namespace: namespace, Name: name, Labels: copyStringMap(sec.Labels), Data: data}, nil
}

// ResolveAllKeys loads the Secret at ref and returns a copy of its data
// as map[string]string.
func ResolveAllKeys(ctx context.Context, c client.Client, ref *xpv1.SecretReference) (map[string]string, error) {
	resolved, err := Resolve(ctx, c, ref)
	if err != nil {
		return nil, err
	}
	return resolved.Data, nil
}

// ResolveNamed loads each referenced Secret and returns a map keyed by
// the caller-provided Name field to the resolved key/value data. Empty
// or nil refs yield a nil map to let callers skip setting the wire
// field. Duplicate Names error out because they would silently clobber
// each other in the proto map otherwise.
func ResolveNamed(ctx context.Context, c client.Client, refs []v1alpha1.NamedSecretReference) (map[string]ResolvedSecret, error) {
	if len(refs) == 0 {
		return nil, nil
	}
	out := make(map[string]ResolvedSecret, len(refs))
	for i := range refs {
		name := refs[i].CredentialName()
		if !repoCredentialNameRE.MatchString(name) {
			return nil, fmt.Errorf("%w: effective name %q must match ^repo-[a-z0-9][a-z0-9-]*$", ErrInvalidSecretReference, name)
		}
		if _, dup := out[name]; dup {
			return nil, fmt.Errorf("%w: duplicate secret reference name %q", ErrInvalidSecretReference, name)
		}
		resolved, err := Resolve(ctx, c, &refs[i].SecretRef)
		if err != nil {
			return nil, fmt.Errorf("resolve %q: %w", name, err)
		}
		out[name] = resolved
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

// HashNamedResolved returns a stable SHA256 spanning every named
// resolved Secret. The outer slot names and each source Secret
// namespace/name/data digest contribute to the result.
func HashNamedResolved(data map[string]ResolvedSecret) string {
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
		h.Write([]byte(data[k].Hash()))
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

func copyStringMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
