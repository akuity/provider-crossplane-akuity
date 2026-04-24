// Package reason defines sentinel error types that classify why a reconcile
// call failed. Controllers use the IsNotFound / IsPermissionDenied /
// IsRetryable / IsNotReconciled predicates to decide whether to mark a
// managed resource absent, surface a condition, or let the workqueue retry.
//
// # NotFound / PermissionDenied conflation
//
// The Akuity control plane uses organisation-scoped RBAC. Calls that target
// a resource the caller cannot see may return either codes.NotFound or
// codes.PermissionDenied depending on which gate fired first, and the two
// are not distinguishable from the client. Because the Crossplane provider
// only ever queries resources owned by the ProviderConfig it was configured
// with, any PermissionDenied at read time is treated as NotFound at the
// akuity-client boundary (see internal/clients/akuity). Controllers can
// therefore call reason.IsNotFound and trust it to cover both cases.
//
// If a future Akuity API change separates the two, loosen this conflation at
// the client boundary; controllers should not need to change.
package reason
