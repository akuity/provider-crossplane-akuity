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

// Package base holds generic scaffolding shared by all v1alpha2
// controllers: a BaseConnector[T] that handles PC/ClusterPC resolution
// and usage tracking, and a BaseExternalClient that embeds the Akuity
// client plus logger/recorder. Each per-resource controller embeds
// BaseExternalClient and implements the four managed.TypedExternalClient
// lifecycle methods.
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

// ClientFactory constructs the per-reconcile Akuity API client from a
// managed resource's ProviderConfigReference. It is extracted so tests
// can plug in a fake without reaching into the credentials path.
type ClientFactory func(ctx context.Context, kube client.Client, mg resource.ModernManaged) (akuity.Client, error)

// DefaultClientFactory resolves either a namespaced ProviderConfig
// (same namespace as mg) or a cluster-scoped ClusterProviderConfig,
// depending on the Kind on the managed resource's
// ProviderConfigReference.
func DefaultClientFactory(ctx context.Context, kube client.Client, mg resource.ModernManaged) (akuity.Client, error) {
	return config.GetAkuityClientFromV2Ref(ctx, kube, mg.GetProviderConfigReference(), mg.GetNamespace())
}

// ExternalClientBuilder turns a typed managed resource plus a resolved
// Akuity client into a concrete TypedExternalClient. Per-resource
// controllers supply this.
type ExternalClientBuilder[T resource.ModernManaged] func(client akuity.Client, kube client.Client, logger logging.Logger, recorder event.Recorder) managed.TypedExternalClient[T]

// Connector is a generic TypedExternalConnector[T]. It tracks
// ProviderConfig usage via the modern (typed) tracker, resolves the
// Akuity client through the supplied ClientFactory, and hands both
// client and kube client off to the per-resource builder.
type Connector[T resource.ModernManaged] struct {
	Kube      client.Client
	Usage     *resource.ProviderConfigUsageTracker
	Logger    logging.Logger
	Recorder  event.Recorder
	NewClient ClientFactory
	Build     ExternalClientBuilder[T]
}

// Connect implements managed.TypedExternalConnector.
func (c *Connector[T]) Connect(ctx context.Context, mg T) (managed.TypedExternalClient[T], error) {
	if err := c.Usage.Track(ctx, mg); err != nil {
		return nil, errors.Wrap(err, "cannot track ProviderConfig usage")
	}

	ac, err := c.NewClient(ctx, c.Kube, mg)
	if err != nil {
		return nil, err
	}

	return c.Build(ac, c.Kube, c.Logger, c.Recorder), nil
}
