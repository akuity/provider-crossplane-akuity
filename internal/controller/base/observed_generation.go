package base

import "github.com/crossplane/crossplane-runtime/v2/pkg/resource"

// PropagateObservedGeneration mirrors metadata.generation onto the
// top-level status.observedGeneration field.
func PropagateObservedGeneration[T interface {
	resource.ModernManaged
	resource.ReconciliationObserver
}](mg T) {
	mg.SetObservedGeneration(mg.GetGeneration())
}
