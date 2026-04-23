/*
Copyright 2026 The Crossplane Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package base

import (
	"context"

	"github.com/crossplane/crossplane-runtime/v2/pkg/event"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	"github.com/pkg/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/akuityio/provider-crossplane-akuity/internal/clients/akuity"
	"github.com/akuityio/provider-crossplane-akuity/internal/controller/config"
)

// ObservedManaged is the subset of cluster-scoped managed resources
// handled by the shared connector. In addition to the managed-resource
// surface, they expose top-level observedGeneration accessors.
//
// resource.LegacyManaged is marked deprecated upstream in favour of
// ModernManaged (namespaced). The Akuity provider deliberately targets
// cluster-scoped MRs for backward compatibility with existing customer
// manifests; the deprecation warning does not apply.
type ObservedManaged interface {
	resource.LegacyManaged //nolint:staticcheck // cluster-scoped MRs are intentional
	resource.ReconciliationObserver
}

// ClientFactory constructs the per-reconcile Akuity API client from a
// managed resource's ProviderConfigReference.
type ClientFactory func(ctx context.Context, kube client.Client, mg resource.LegacyManaged) (akuity.Client, error) //nolint:staticcheck // cluster-scoped MRs are intentional

// DefaultClientFactory resolves the cluster-scoped ProviderConfig named
// on mg.
func DefaultClientFactory(ctx context.Context, kube client.Client, mg resource.LegacyManaged) (akuity.Client, error) { //nolint:staticcheck // cluster-scoped MRs are intentional
	ref := mg.GetProviderConfigReference()
	if ref == nil {
		return nil, errors.New("managed resource has no providerConfigRef")
	}
	return config.GetAkuityClientFromProviderConfig(ctx, kube, ref.Name)
}

// ExternalClientBuilder turns a typed cluster-scoped managed resource
// plus a resolved Akuity client into a concrete TypedExternalClient.
type ExternalClientBuilder[T ObservedManaged] func(client akuity.Client, kube client.Client, logger logging.Logger, recorder event.Recorder) managed.TypedExternalClient[T]

// Connector is a generic TypedExternalConnector[T] for cluster-scoped
// MRs. Wires the ProviderConfigUsage tracker and the ProviderConfig
// lookup.
type Connector[T ObservedManaged] struct {
	Kube      client.Client
	Usage     *resource.LegacyProviderConfigUsageTracker
	Logger    logging.Logger
	Recorder  event.Recorder
	NewClient ClientFactory
	Build     ExternalClientBuilder[T]
}

// Connect implements managed.TypedExternalConnector.
func (c *Connector[T]) Connect(ctx context.Context, mg T) (managed.TypedExternalClient[T], error) {
	PropagateObservedGeneration(mg)

	if err := c.Usage.Track(ctx, mg); err != nil {
		return nil, errors.Wrap(err, "cannot track ProviderConfig usage")
	}

	ac, err := c.NewClient(ctx, c.Kube, mg)
	if err != nil {
		return nil, err
	}

	return c.Build(ac, c.Kube, c.Logger, c.Recorder), nil
}
