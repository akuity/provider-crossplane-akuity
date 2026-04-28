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

// Package event provides a small adapter that lets controller Setup
// functions obtain a crossplane-runtime event.Recorder without calling
// the deprecated controller-runtime Manager.GetEventRecorderFor.
package event

import (
	ctrl "sigs.k8s.io/controller-runtime"

	xpevent "github.com/crossplane/crossplane-runtime/v2/pkg/event"
)

// Recorder re-exports the crossplane-runtime event.Recorder type so
// callers can import this package alone (for both the NewRecorder
// constructor and the Recorder interface) instead of pulling the
// upstream event package alongside.
type Recorder = xpevent.Recorder

// NewRecorder returns a crossplane-runtime event.Recorder backed by the
// legacy core/v1 Event sink. controller-runtime's non-deprecated
// GetEventRecorder uses events.k8s.io/v1, but meta.pkg.crossplane.io/v1
// Provider packages cannot request those RBAC verbs. Crossplane's
// generated provider RBAC does include core/v1 Events, so keep this path
// until the package metadata format can grant events.k8s.io permissions.
func NewRecorder(mgr ctrl.Manager, name string) xpevent.Recorder {
	recorder := mgr.GetEventRecorderFor(name) //nolint:staticcheck
	return xpevent.NewAPIRecorder(recorder)
}
