package metrics

import (
	"context"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"k8s.io/apimachinery/pkg/runtime"
	runtimeschema "k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/crossplane/crossplane-runtime/v2/pkg/event"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"

	"github.com/akuityio/provider-crossplane-akuity/apis/core/v1alpha1"
)

func newScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	s := runtime.NewScheme()
	if err := v1alpha1.SchemeBuilder.AddToScheme(s); err != nil {
		t.Fatalf("add v1alpha1 to scheme: %v", err)
	}
	return s
}

func TestLegacyCounterTick(t *testing.T) {
	scheme := newScheme(t)
	cluster1 := &v1alpha1.Cluster{}
	cluster1.SetName("c1")
	cluster2 := &v1alpha1.Cluster{}
	cluster2.SetName("c2")
	instance1 := &v1alpha1.Instance{}
	instance1.SetName("i1")

	kube := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(cluster1, cluster2, instance1).
		Build()

	LegacyCRCount.Reset()
	lc := &LegacyCounter{
		Client: kube,
		Log:    logging.NewNopLogger(),
	}
	lc.tick(context.Background())

	if got := testutil.ToFloat64(LegacyCRCount.WithLabelValues("Cluster")); got != 2 {
		t.Errorf("Cluster gauge = %v, want 2", got)
	}
	if got := testutil.ToFloat64(LegacyCRCount.WithLabelValues("Instance")); got != 1 {
		t.Errorf("Instance gauge = %v, want 1", got)
	}
}

func TestLegacyCounterTick_ListError_KeepsLastValue(t *testing.T) {
	// No scheme registered → List returns an error; tick must not panic and
	// must leave previously-set gauge values untouched.
	kube := fake.NewClientBuilder().WithScheme(runtime.NewScheme()).Build()

	LegacyCRCount.Reset()
	LegacyCRCount.WithLabelValues("Cluster").Set(7)

	lc := &LegacyCounter{
		Client: kube,
		Log:    logging.NewNopLogger(),
	}
	lc.tick(context.Background())

	if got := testutil.ToFloat64(LegacyCRCount.WithLabelValues("Cluster")); got != 7 {
		t.Errorf("Cluster gauge after list error = %v, want 7 (unchanged)", got)
	}
}

// capturingRecorder is a minimal event.Recorder that stores every Event
// passed to it, for assertions.
type capturingRecorder struct {
	events []capturedEvent
}

type capturedEvent struct {
	obj runtime.Object
	e   event.Event
}

func (c *capturingRecorder) Event(obj runtime.Object, e event.Event) {
	c.events = append(c.events, capturedEvent{obj: obj, e: e})
}

func (c *capturingRecorder) WithAnnotations(_ ...string) event.Recorder { return c }

func TestNewLegacyDeprecationInitializer_EmitsWarning(t *testing.T) {
	rec := &capturingRecorder{}
	init := NewLegacyDeprecationInitializer(rec)

	cluster := &v1alpha1.Cluster{}
	cluster.SetName("legacy-one")
	cluster.SetGroupVersionKind(runtimeschema.GroupVersionKind{
		Group:   v1alpha1.Group,
		Version: v1alpha1.Version,
		Kind:    "Cluster",
	})

	if err := init.Initialize(context.Background(), cluster); err != nil {
		t.Fatalf("Initialize returned err: %v", err)
	}
	if len(rec.events) != 1 {
		t.Fatalf("got %d events, want 1", len(rec.events))
	}
	got := rec.events[0].e
	if got.Type != event.TypeWarning {
		t.Errorf("event.Type = %q, want Warning", got.Type)
	}
	if got.Reason != LegacyDeprecationReason {
		t.Errorf("event.Reason = %q, want %q", got.Reason, LegacyDeprecationReason)
	}
	if !strings.Contains(got.Message, LegacyRemovalVersion) ||
		!strings.Contains(got.Message, LegacyRemovalTargetDate) ||
		!strings.Contains(got.Message, "core.m.akuity.crossplane.io/v1alpha2") {
		t.Errorf("event.Message missing removal/version/target-date/v1alpha2 pointer: %q", got.Message)
	}
}
