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
	"github.com/crossplane/crossplane-runtime/v2/pkg/controller"
	"github.com/crossplane/crossplane-runtime/v2/pkg/event"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	"github.com/crossplane/crossplane-runtime/v2/pkg/statemetrics"
	"github.com/pkg/errors"
	ctrl "sigs.k8s.io/controller-runtime"
)

// ReconcilerOptions returns the managed.Reconciler options every Akuity
// controller shares (logging, polling, events, management policies and,
// when the provider exposes metrics, the managed resource metric recorder)
// followed by the supplied controller-specific options.
func ReconcilerOptions(o controller.Options, logger logging.Logger, recorder event.Recorder, extra ...managed.ReconcilerOption) []managed.ReconcilerOption {
	opts := []managed.ReconcilerOption{
		managed.WithLogger(logger),
		managed.WithPollInterval(o.PollInterval),
		managed.WithRecorder(recorder),
		managed.WithManagementPolicies(),
	}
	if o.MetricOptions != nil && o.MetricOptions.MRMetrics != nil {
		opts = append(opts, managed.WithMetricRecorder(o.MetricOptions.MRMetrics))
	}
	return append(opts, extra...)
}

// AddStateMetrics registers a runnable with the manager that periodically
// records the state (ready, synced, deleting) of every managed resource in
// list. It is a no-op when the provider exposes no state metrics.
func AddStateMetrics(mgr ctrl.Manager, o controller.Options, list resource.ManagedList) error {
	if o.MetricOptions == nil || o.MetricOptions.MRStateMetrics == nil {
		return nil
	}
	rec := statemetrics.NewMRStateRecorder(mgr.GetClient(), o.Logger, o.MetricOptions.MRStateMetrics, list, o.MetricOptions.PollStateMetricInterval)
	return errors.Wrapf(mgr.Add(rec), "cannot register state metrics recorder for %T", list)
}
