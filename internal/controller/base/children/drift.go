// Package children compares the user's declarative child-resource
// bundle on a managed resource spec (raw YAML via runtime.RawExtension)
// against the set of children the Akuity gateway reports back via its
// Export* responses (opaque structpb.Structs).
//
// The provider operates with additive semantics: removing an entry
// from spec.forProvider.{argocdResources, kargoResources} does NOT
// trigger server-side deletion. This mirrors the conservative pruning
// posture chosen for the provider (only CONFIG_MANAGEMENT_PLUGINS is
// server-pruned by default) and avoids clobbering resources that other
// tools or the Akuity platform UI are co-managing. The drift signal
// therefore flows only one way: every desired child must be present
// and its fields reflected on the server; extra server-side children
// and extra server-side fields are intentionally ignored.
//
// Subset matching. Declarative children are often heavily defaulted by
// the server (ArgoCD fills in `spec.project=default`, `syncPolicy`,
// and more when the user omits them). A straight digest comparison
// would report drift on every reconcile loop for these. Instead, for
// each desired path the comparator looks up the corresponding path in
// the observed tree and compares values; extra fields on the observed
// side are silently dropped. This matches the semantics of
// `kubectl apply`'s last-applied-configuration diff and is what
// operators expect from a declarative API.
package children

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"

	"google.golang.org/protobuf/types/known/structpb"
	"k8s.io/apimachinery/pkg/runtime"
)

// Identity is the (apiVersion, kind, namespace, name) tuple used to
// match desired children against observed ones. Keeping it a struct
// rather than a flattened string keeps error messages readable.
type Identity struct {
	APIVersion string
	Kind       string
	Namespace  string
	Name       string
}

func (k Identity) String() string {
	ns := k.Namespace
	if ns == "" {
		ns = "-"
	}
	return fmt.Sprintf("%s/%s/%s/%s", k.APIVersion, k.Kind, ns, k.Name)
}

// serverMetaFields are stripped from every child before comparison so
// kube-apiserver-assigned values (resourceVersion, uid, timestamps,
// generation, managedFields, selfLink) can't produce false drift.
// The matching entries on metadata are listed inline here rather than
// as a set literal to make the intent — "these belong to the server" —
// readable at the call site.
//
// Fields deliberately NOT stripped:
//   - ownerReferences and finalizers are user-owned lifecycle fields.
//     An ArgoCD Application's resources-finalizer.argocd.argoproj.io
//     changes whether workloads cascade on delete; an explicit
//     ownerReference declares parent-child ties the server must not
//     drop. Treating them as server-owned hid server-side removal
//     from drift detection.
var serverMetaFields = []string{
	"resourceVersion",
	"uid",
	"generation",
	"creationTimestamp",
	"managedFields",
	"selfLink",
	"deletionTimestamp",
	"deletionGracePeriodSeconds",
}

// Index parses each raw payload into its Identity and stores the
// decoded object (stripped of server-managed metadata + status). The
// decoded map is retained so Compare can walk the desired tree and
// look up each path in the observed tree — no digest comparison
// happens at this layer.
func Index(raw []runtime.RawExtension) (map[Identity]map[string]interface{}, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	out := make(map[Identity]map[string]interface{}, len(raw))
	for i, r := range raw {
		if len(r.Raw) == 0 {
			return nil, fmt.Errorf("entry %d: empty payload", i)
		}
		obj := map[string]interface{}{}
		if err := json.Unmarshal(r.Raw, &obj); err != nil {
			return nil, fmt.Errorf("entry %d: invalid JSON: %w", i, err)
		}
		id, err := identityOf(obj)
		if err != nil {
			return nil, fmt.Errorf("entry %d: %w", i, err)
		}
		if _, dup := out[id]; dup {
			return nil, fmt.Errorf("entry %d: duplicate child identity %s", i, id)
		}
		out[id] = strip(obj)
	}
	return out, nil
}

// IndexStructs does the same Identity + decode step for the
// []*structpb.Struct shape the Akuity gateway returns in its Export*
// responses. structpb Structs are deterministic on (de)serialization
// so converting to map[string]interface{} round-trips cleanly.
func IndexStructs(in []*structpb.Struct) (map[Identity]map[string]interface{}, error) {
	if len(in) == 0 {
		return nil, nil
	}
	out := make(map[Identity]map[string]interface{}, len(in))
	for i, s := range in {
		if s == nil {
			continue
		}
		obj := s.AsMap()
		id, err := identityOf(obj)
		if err != nil {
			return nil, fmt.Errorf("struct %d: %w", i, err)
		}
		if _, dup := out[id]; dup {
			// The gateway shouldn't return duplicates, but a defensive
			// check beats a silent "last one wins" during comparison.
			return nil, fmt.Errorf("struct %d: duplicate child identity %s", i, id)
		}
		out[id] = strip(obj)
	}
	return out, nil
}

// DriftReport compares a desired index (what the user asked for in
// spec.forProvider.*Resources) against the observed index (what the
// Akuity gateway reports back via Export). With additive semantics,
// drift fires when any desired child is either missing from the server
// or present but not subset-matched. Extra server-side children and
// extra server-side fields are intentionally ignored.
type DriftReport struct {
	// Missing lists desired children the gateway has not reported;
	// next Apply should create them.
	Missing []Identity
	// Changed lists desired children whose fields diverge from the
	// gateway's copy on at least one desired path; next Apply should
	// bring them back in line.
	Changed []Identity
}

// Empty reports whether the desired bundle is already fully reflected
// on the gateway.
func (d DriftReport) Empty() bool {
	return len(d.Missing) == 0 && len(d.Changed) == 0
}

// Compare produces a DriftReport for additive-semantics reconciliation
// with subset field matching. For each desired child, the comparator
// walks the desired tree and asserts that every path is present and
// equal in the observed tree. Paths absent from desired are ignored —
// that's what makes server-side defaults (e.g. ArgoCD filling in
// spec.project=default) non-drift.
//
// Namespace defaulting (H2): a desired manifest with metadata.namespace
// omitted ("") is matched against an observed resource of the same
// apiVersion+kind+name if (and only if) exactly one such observed
// resource exists. This handles the common case where the user writes
// no namespace and the gateway reports the resource with a concrete
// default (e.g. "default" or a project namespace) without introducing
// cross-namespace ambiguity — two observed candidates → no match →
// fall through to missing, which is the safe signal.
func Compare(desired, observed map[Identity]map[string]interface{}) DriftReport {
	if len(desired) == 0 {
		return DriftReport{}
	}
	// Iterating the desired set in sorted order keeps the report
	// deterministic, which makes downstream log messages and tests
	// reproducible.
	var report DriftReport
	for _, id := range sortedKeys(desired) {
		obs, ok := observed[id]
		if !ok && id.Namespace == "" {
			obs, ok = lookupAnyNamespace(observed, id)
		}
		if !ok {
			report.Missing = append(report.Missing, id)
			continue
		}
		if !matchesSubset(desired[id], obs) {
			report.Changed = append(report.Changed, id)
		}
	}
	return report
}

// lookupAnyNamespace returns the single observed entry matching the
// (apiVersion, kind, name) triple of id when id's Namespace is empty.
// Returns ok=false if zero or more than one candidate exists; the
// latter case is deliberately treated as no-match so a namespace-less
// desired manifest never silently maps to the "wrong" observed twin.
func lookupAnyNamespace(observed map[Identity]map[string]interface{}, id Identity) (map[string]interface{}, bool) {
	var match map[string]interface{}
	found := 0
	for k, v := range observed {
		if k.APIVersion == id.APIVersion && k.Kind == id.Kind && k.Name == id.Name {
			match = v
			found++
			if found > 1 {
				return nil, false
			}
		}
	}
	if found == 1 {
		return match, true
	}
	return nil, false
}

// matchesSubset reports whether every path in desired is present in
// observed with an equal value. Maps recurse; arrays are compared
// element-wise (length + position must match, since array order is
// meaningful for most Kubernetes fields like containers and env);
// scalars use reflect.DeepEqual so JSON number types (always float64
// after unmarshal) compare correctly.
//
// Asymmetric by design: observed may contain extra map keys that
// desired omits. Those extras are exactly the server-side defaults
// the controller must not treat as drift.
//
// Known sharp edge (H1 / Codex-medium): arrays are strict on length
// and position. If the Akuity gateway ever starts appending defaulted
// elements to user-supplied arrays inside a declarative child (server-
// side admission injecting a default toleration into a Kargo Stage,
// for example), every reconcile loop would report drift and re-Apply
// the same payload indefinitely. No supported child kind is currently
// known to behave that way — the regression trap at
// drift_test.go:TestMatchesSubset_ArrayAppendTrap documents the
// behaviour so we learn immediately if the gateway changes.
//
// Two deliberate non-fixes here:
//   - Subset-matching on arrays would hide legitimate user drift
//     (reordered containers, removed env entries). Kubernetes array
//     semantics are order-sensitive; a lenient matcher is a bug, not
//     a feature.
//   - Strategic-merge-patch-style keyed array matching (containers
//     by name, env by name, …) is per-kind work with no upstream
//     schema source for arbitrary Akuity wire shapes. Introduce it
//     only when a specific kind forces our hand.
//
//nolint:gocyclo // matchesSubset is a single recursive type switch over JSON shapes; flattening it would lose the comparison contract documented in the doc-comment above.
func matchesSubset(desired, observed interface{}) bool {
	switch d := desired.(type) {
	case map[string]interface{}:
		o, ok := observed.(map[string]interface{})
		if !ok {
			// A desired empty map matches an absent observed value:
			// "spec: {}" / "metadata: {}" is the same shape the
			// gateway returns when no fields under that key carry
			// content, but the gateway suppresses the empty container
			// from the export response. Treating `present-empty` and
			// `absent` as distinct produces drift on every reconcile
			// for any user-supplied manifest with an empty section
			// (Kargo Project's `spec: {}` is the canonical case —
			// the kind has no required spec fields and the server
			// echoes back metadata only).
			return len(d) == 0 && observed == nil
		}
		for k, dv := range d {
			ov, present := o[k]
			if !present {
				// Same logic as the absent-observed branch above:
				// a desired empty map / empty list / nil is
				// indistinguishable from absent on the gateway side.
				if isEmptyJSONValue(dv) {
					continue
				}
				return false
			}
			if !matchesSubset(dv, ov) {
				return false
			}
		}
		return true
	case []interface{}:
		o, ok := observed.([]interface{})
		if !ok {
			return len(d) == 0 && observed == nil
		}
		if len(d) != len(o) {
			return false
		}
		for i := range d {
			if !matchesSubset(d[i], o[i]) {
				return false
			}
		}
		return true
	default:
		// Scalars and untyped nils.
		return reflect.DeepEqual(desired, observed)
	}
}

// isEmptyJSONValue reports whether v is the JSON-decoded representation
// of "this key has no content" — empty map, empty array, or nil. Treated
// as equivalent to absent during desired ⊆ observed matching so that
// users can spell out a structural placeholder ("spec: {}") without
// every reconcile flagging drift against a gateway that suppresses
// empty containers from its export response.
func isEmptyJSONValue(v interface{}) bool {
	switch t := v.(type) {
	case nil:
		return true
	case map[string]interface{}:
		return len(t) == 0
	case []interface{}:
		return len(t) == 0
	default:
		return false
	}
}

// identityOf pulls the apiVersion / kind / metadata.{name, namespace}
// out of a decoded manifest. metadata.name is required; the other
// fields default cleanly when absent.
func identityOf(obj map[string]interface{}) (Identity, error) {
	apiVersion, _ := obj["apiVersion"].(string)
	kind, _ := obj["kind"].(string)
	md, _ := obj["metadata"].(map[string]interface{})
	if md == nil {
		return Identity{}, fmt.Errorf("missing metadata")
	}
	name, _ := md["name"].(string)
	if name == "" {
		return Identity{}, fmt.Errorf("missing metadata.name")
	}
	namespace, _ := md["namespace"].(string)
	return Identity{
		APIVersion: apiVersion,
		Kind:       kind,
		Namespace:  namespace,
		Name:       name,
	}, nil
}

// strip returns a copy of obj with server-managed metadata fields and
// any status sub-tree removed. Operates on a shallow clone so callers
// can keep using the original map.
func strip(obj map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{}, len(obj))
	for k, v := range obj {
		if k == "status" {
			// status is owned by the server; a mismatch there is
			// never evidence that the user's spec needs to be
			// re-applied.
			continue
		}
		if k == "metadata" {
			md, ok := v.(map[string]interface{})
			if ok {
				out[k] = stripMeta(md)
				continue
			}
		}
		out[k] = v
	}
	return out
}

func stripMeta(md map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{}, len(md))
	for k, v := range md {
		if isServerMetaField(k) {
			continue
		}
		out[k] = v
	}
	return out
}

func isServerMetaField(k string) bool {
	for _, f := range serverMetaFields {
		if f == k {
			return true
		}
	}
	return false
}

func sortedKeys(m map[Identity]map[string]interface{}) []Identity {
	out := make([]Identity, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].String() < out[j].String()
	})
	return out
}
