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

func TestResolveAllKeys_NilRef(t *testing.T) {
	c := fakeClient(t).Build()
	got, err := ResolveAllKeys(context.Background(), c, nil)
	if err != nil || got != nil {
		t.Fatalf("nil ref: got=%v err=%v, want nil,nil", got, err)
	}
}

func TestResolveAllKeys_EmptyRefIsInvalid(t *testing.T) {
	c := fakeClient(t).Build()
	_, err := ResolveAllKeys(context.Background(), c, &xpv1.SecretReference{})
	if !errors.Is(err, ErrInvalidSecretReference) {
		t.Fatalf("want ErrInvalidSecretReference, got %v", err)
	}
}

func TestResolveAllKeys_Found(t *testing.T) {
	sec := newSecret("akuity", "argo-secret", map[string]string{
		"admin.password": "hunter2",
		"server.secret":  "s3cr3t",
	})
	c := fakeClient(t, sec).Build()
	got, err := ResolveAllKeys(context.Background(), c,
		&xpv1.SecretReference{Namespace: "akuity", Name: "argo-secret"})
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
	_, err := ResolveAllKeys(context.Background(), c,
		&xpv1.SecretReference{Namespace: "akuity", Name: "ghost"})
	if !errors.Is(err, ErrMissingSecret) {
		t.Fatalf("want ErrMissingSecret, got %v", err)
	}
}

func TestResolveAllKeys_EmptySecretWrapsSentinel(t *testing.T) {
	sec := newSecret("akuity", "empty", nil)
	c := fakeClient(t, sec).Build()
	_, err := ResolveAllKeys(context.Background(), c,
		&xpv1.SecretReference{Namespace: "akuity", Name: "empty"})
	if !errors.Is(err, ErrEmptySecret) {
		t.Fatalf("want ErrEmptySecret, got %v", err)
	}
}

func TestResolveNamed_HappyPath(t *testing.T) {
	a := newSecret("akuity", "creds-a", map[string]string{"url": "https://a", "password": "p1"})
	b := newSecret("akuity", "repo-creds-b", map[string]string{"url": "https://b", "sshPrivateKey": "key"})
	c := fakeClient(t, a, b).Build()

	got, err := ResolveNamed(context.Background(), c, []v1alpha1.NamedSecretReference{
		{Name: "repo-a", SecretRef: xpv1.SecretReference{Namespace: "akuity", Name: "creds-a"}},
		{SecretRef: xpv1.SecretReference{Namespace: "akuity", Name: "repo-creds-b"}},
	})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	want := map[string]ResolvedSecret{
		"repo-a":       {Namespace: "akuity", Name: "creds-a", Data: map[string]string{"url": "https://a", "password": "p1"}},
		"repo-creds-b": {Namespace: "akuity", Name: "repo-creds-b", Data: map[string]string{"url": "https://b", "sshPrivateKey": "key"}},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("mismatch (-want +got):\n%s", diff)
	}
}

func TestResolveNamed_EmptyReturnsNil(t *testing.T) {
	c := fakeClient(t).Build()
	got, err := ResolveNamed(context.Background(), c, nil)
	if err != nil || got != nil {
		t.Fatalf("got=%v err=%v, want nil,nil", got, err)
	}
}

func TestResolveNamed_DuplicateNames(t *testing.T) {
	a := newSecret("akuity", "creds-a", map[string]string{"url": "https://a"})
	c := fakeClient(t, a).Build()
	_, err := ResolveNamed(context.Background(), c, []v1alpha1.NamedSecretReference{
		{Name: "repo-dup", SecretRef: xpv1.SecretReference{Namespace: "akuity", Name: "creds-a"}},
		{Name: "repo-dup", SecretRef: xpv1.SecretReference{Namespace: "akuity", Name: "creds-b"}},
	})
	if err == nil {
		t.Fatalf("want duplicate-name error, got nil")
	}
	if !errors.Is(err, ErrInvalidSecretReference) {
		t.Fatalf("want ErrInvalidSecretReference, got %v", err)
	}
}

func TestResolveNamed_InvalidEffectiveName(t *testing.T) {
	a := newSecret("akuity", "creds-a", map[string]string{"url": "https://a"})
	c := fakeClient(t, a).Build()
	_, err := ResolveNamed(context.Background(), c, []v1alpha1.NamedSecretReference{
		{SecretRef: xpv1.SecretReference{Namespace: "akuity", Name: "creds-a"}},
	})
	if !errors.Is(err, ErrInvalidSecretReference) {
		t.Fatalf("want ErrInvalidSecretReference, got %v", err)
	}
}

func TestResolveNamed_PropagatesMissing(t *testing.T) {
	c := fakeClient(t).Build()
	_, err := ResolveNamed(context.Background(), c, []v1alpha1.NamedSecretReference{
		{Name: "repo-x", SecretRef: xpv1.SecretReference{Namespace: "akuity", Name: "ghost"}},
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
