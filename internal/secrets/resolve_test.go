package secrets

import (
	"context"
	"errors"
	"testing"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/akuityio/provider-crossplane-akuity/apis/core/v1alpha1"
)

func fakeClient(t *testing.T, objs ...runtime.Object) *fake.ClientBuilder {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("add corev1 to scheme: %v", err)
	}
	return fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(objs...)
}

func newSecret(ns, name string, data map[string]string) *corev1.Secret {
	out := map[string][]byte{}
	for k, v := range data {
		out[k] = []byte(v)
	}
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Namespace: ns, Name: name},
		Data:       out,
	}
}

func TestResolveAllKeys_NilOrEmptyRef(t *testing.T) {
	c := fakeClient(t).Build()
	got, err := ResolveAllKeys(context.Background(), c, "default", nil)
	if err != nil || got != nil {
		t.Fatalf("nil ref: got=%v err=%v, want nil,nil", got, err)
	}
	got, err = ResolveAllKeys(context.Background(), c, "default", &xpv1.LocalSecretReference{})
	if err != nil || got != nil {
		t.Fatalf("empty ref: got=%v err=%v, want nil,nil", got, err)
	}
}

func TestResolveAllKeys_Found(t *testing.T) {
	sec := newSecret("akuity", "argo-secret", map[string]string{
		"admin.password": "hunter2",
		"server.secret":  "s3cr3t",
	})
	c := fakeClient(t, sec).Build()
	got, err := ResolveAllKeys(context.Background(), c, "akuity",
		&xpv1.LocalSecretReference{Name: "argo-secret"})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	want := map[string]string{
		"admin.password": "hunter2",
		"server.secret":  "s3cr3t",
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("mismatch (-want +got):\n%s", diff)
	}
}

func TestResolveAllKeys_NotFoundWrapsSentinel(t *testing.T) {
	c := fakeClient(t).Build()
	_, err := ResolveAllKeys(context.Background(), c, "akuity",
		&xpv1.LocalSecretReference{Name: "ghost"})
	if !errors.Is(err, ErrMissingSecret) {
		t.Fatalf("want ErrMissingSecret, got %v", err)
	}
}

func TestResolveNamed_HappyPath(t *testing.T) {
	a := newSecret("akuity", "creds-a", map[string]string{"url": "https://a", "password": "p1"})
	b := newSecret("akuity", "creds-b", map[string]string{"url": "https://b", "sshPrivateKey": "key"})
	c := fakeClient(t, a, b).Build()

	got, err := ResolveNamed(context.Background(), c, "akuity", []v1alpha1.NamedLocalSecretReference{
		{Name: "repo-a", SecretRef: xpv1.LocalSecretReference{Name: "creds-a"}},
		{Name: "repo-b", SecretRef: xpv1.LocalSecretReference{Name: "creds-b"}},
	})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	want := map[string]map[string]string{
		"repo-a": {"url": "https://a", "password": "p1"},
		"repo-b": {"url": "https://b", "sshPrivateKey": "key"},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("mismatch (-want +got):\n%s", diff)
	}
}

func TestResolveNamed_EmptyReturnsNil(t *testing.T) {
	c := fakeClient(t).Build()
	got, err := ResolveNamed(context.Background(), c, "akuity", nil)
	if err != nil || got != nil {
		t.Fatalf("got=%v err=%v, want nil,nil", got, err)
	}
}

func TestResolveNamed_DuplicateNames(t *testing.T) {
	a := newSecret("akuity", "creds-a", map[string]string{"url": "https://a"})
	c := fakeClient(t, a).Build()
	_, err := ResolveNamed(context.Background(), c, "akuity", []v1alpha1.NamedLocalSecretReference{
		{Name: "repo-dup", SecretRef: xpv1.LocalSecretReference{Name: "creds-a"}},
		{Name: "repo-dup", SecretRef: xpv1.LocalSecretReference{Name: "creds-a"}},
	})
	if err == nil {
		t.Fatalf("want duplicate-name error, got nil")
	}
}

func TestResolveNamed_PropagatesMissing(t *testing.T) {
	c := fakeClient(t).Build()
	_, err := ResolveNamed(context.Background(), c, "akuity", []v1alpha1.NamedLocalSecretReference{
		{Name: "repo-x", SecretRef: xpv1.LocalSecretReference{Name: "ghost"}},
	})
	if !errors.Is(err, ErrMissingSecret) {
		t.Fatalf("want ErrMissingSecret, got %v", err)
	}
}

func TestHash_StableAcrossKeyOrder(t *testing.T) {
	h1 := Hash(map[string]string{"a": "1", "b": "2"})
	h2 := Hash(map[string]string{"b": "2", "a": "1"})
	if h1 == "" {
		t.Fatalf("empty hash for non-empty input")
	}
	if h1 != h2 {
		t.Fatalf("hash not stable: %s vs %s", h1, h2)
	}
}

func TestHash_DifferentForMutations(t *testing.T) {
	base := map[string]string{"a": "1", "b": "2"}
	diff := map[string]string{"a": "1", "b": "3"}
	if Hash(base) == Hash(diff) {
		t.Fatalf("collision on value mutation")
	}
	extra := map[string]string{"a": "1", "b": "2", "c": "3"}
	if Hash(base) == Hash(extra) {
		t.Fatalf("collision on added key")
	}
}

func TestHash_EmptyIsEmptyString(t *testing.T) {
	if Hash(nil) != "" {
		t.Fatalf("want empty string for nil")
	}
	if Hash(map[string]string{}) != "" {
		t.Fatalf("want empty string for empty map")
	}
}

func TestHashNamed_Stable(t *testing.T) {
	a := map[string]map[string]string{
		"repo-a": {"url": "u1", "password": "p1"},
		"repo-b": {"url": "u2"},
	}
	b := map[string]map[string]string{
		"repo-b": {"url": "u2"},
		"repo-a": {"password": "p1", "url": "u1"},
	}
	if HashNamed(a) != HashNamed(b) {
		t.Fatalf("HashNamed unstable across key order")
	}
}
