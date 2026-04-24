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
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/events"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"

	xpevent "github.com/crossplane/crossplane-runtime/v2/pkg/event"
)

// Recorder re-exports the crossplane-runtime event.Recorder type so
// callers can import this package alone (for both the NewRecorder
// constructor and the Recorder interface) instead of pulling the
// upstream event package alongside.
type Recorder = xpevent.Recorder

// NewRecorder returns a crossplane-runtime event.Recorder backed by
// controller-runtime's non-deprecated manager.GetEventRecorder. It
// bridges between the two event APIs so controller Setup functions can
// avoid the SA1019 deprecation on manager.GetEventRecorderFor:
//
//   - mgr.GetEventRecorder(name) returns events.EventRecorder, the new
//     k8s.io/client-go events API.
//   - xpevent.NewAPIRecorder upstream in crossplane-runtime still takes
//     the legacy k8s.io/client-go/tools/record.EventRecorder.
//
// The eventRecorderAdapter below satisfies the legacy interface by
// re-dispatching every call to the new Eventf signature. Drop this
// shim once crossplane-runtime migrates its event package to the new
// events API.
func NewRecorder(mgr ctrl.Manager, name string) xpevent.Recorder {
	return xpevent.NewAPIRecorder(&eventRecorderAdapter{inner: mgr.GetEventRecorder(name)})
}

// eventRecorderAdapter lets an events.EventRecorder (new API) stand in
// for a record.EventRecorder (legacy API). Annotations supplied to
// AnnotatedEventf are dropped because the new API does not carry them;
// crossplane-runtime's APIRecorder uses annotations only for optional
// WithAnnotations enrichment, which this codebase never calls.
type eventRecorderAdapter struct {
	inner events.EventRecorder
}

func (a *eventRecorderAdapter) Event(obj runtime.Object, eventtype, reason, message string) {
	a.inner.Eventf(obj, nil, eventtype, reason, "", "%s", message)
}

func (a *eventRecorderAdapter) Eventf(obj runtime.Object, eventtype, reason, messageFmt string, args ...interface{}) {
	a.inner.Eventf(obj, nil, eventtype, reason, "", messageFmt, args...)
}

func (a *eventRecorderAdapter) AnnotatedEventf(obj runtime.Object, _ map[string]string, eventtype, reason, messageFmt string, args ...interface{}) {
	a.inner.Eventf(obj, nil, eventtype, reason, "", messageFmt, args...)
}

// Compile-time assertion that the adapter fully implements the legacy
// interface the crossplane-runtime event package depends on.
var _ record.EventRecorder = (*eventRecorderAdapter)(nil)
