package base

import "github.com/crossplane/crossplane-runtime/v2/pkg/resource"

// PropagateObservedGeneration mirrors metadata.generation onto the
// top-level status.observedGeneration field. Works for both
// modern (namespaced) and legacy (cluster-scoped) managed resources —
// the only requirement is resource.Managed + ReconciliationObserver.
func PropagateObservedGeneration[T interface {
	resource.Managed
	resource.ReconciliationObserver
}](mg T) {
	mg.SetObservedGeneration(mg.GetGeneration())
}
