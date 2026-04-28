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
	"github.com/crossplane/crossplane-runtime/v2/pkg/event"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/akuityio/provider-crossplane-akuity/internal/clients/akuity"
	"github.com/akuityio/provider-crossplane-akuity/internal/reason"
)

// ExternalClient carries the dependencies every external client
// needs. Per-resource clients embed it.
type ExternalClient struct {
	Client   akuity.Client
	Kube     client.Client
	Logger   logging.Logger
	Recorder event.Recorder

	TerminalWrites *TerminalWriteGuard
}

// NewExternalClient is a small constructor used by the per-resource
// ExternalClientBuilder glue.
func NewExternalClient(c akuity.Client, kube client.Client, logger logging.Logger, recorder event.Recorder) ExternalClient {
	return ExternalClient{
		Client:         c,
		Kube:           kube,
		Logger:         logger,
		Recorder:       recorder,
		TerminalWrites: DefaultTerminalWriteGuard,
	}
}

func (e ExternalClient) SuppressTerminalWrite(mg resource.Managed, key TerminalWriteKey) (managed.ExternalObservation, error, bool) {
	if e.TerminalWrites == nil {
		return managed.ExternalObservation{}, nil, false
	}
	obs, err, ok := e.TerminalWrites.Suppress(mg, key)
	if e.Logger != nil {
		action := "miss"
		if ok {
			action = "suppress"
		}
		e.Logger.Debug("terminal write guard", "action", action, "gvk", key.GVK, "namespace", key.Namespace, "name", key.Name, "uidPresent", key.UID != "", "fingerprint", key.Fingerprint)
	}
	return obs, err, ok
}

func (e ExternalClient) RecordTerminalWrite(key TerminalWriteKey, err error) error {
	if e.TerminalWrites != nil {
		e.TerminalWrites.Record(key, err)
	}
	if e.Logger != nil && err != nil {
		action := "ignore"
		if reason.IsTerminal(err) {
			action = "record"
		}
		e.Logger.Debug("terminal write guard", "action", action, "gvk", key.GVK, "namespace", key.Namespace, "name", key.Name, "uidPresent", key.UID != "", "fingerprint", key.Fingerprint)
	}
	return err
}

func (e ExternalClient) ClearTerminalWrite(key TerminalWriteKey) {
	if e.TerminalWrites != nil {
		e.TerminalWrites.Clear(key)
	}
}

func (e ExternalClient) ClearTerminalWriteResource(mg resource.Managed, gvk schema.GroupVersionKind) {
	if e.TerminalWrites != nil {
		e.TerminalWrites.Clear(NewTerminalWriteResourceKey(mg, gvk))
	}
}

func (e ExternalClient) HasTerminalWriteResource(mg resource.Managed, gvk schema.GroupVersionKind) bool {
	if e.TerminalWrites == nil {
		return false
	}
	return e.TerminalWrites.HasResource(NewTerminalWriteResourceKey(mg, gvk))
}

func (e ExternalClient) SkipTerminalWriteGuard(err error) (managed.ExternalObservation, error, bool) {
	if e.Logger != nil {
		e.Logger.Debug("terminal write guard skipped", "err", err)
	}
	return managed.ExternalObservation{}, nil, false
}

func (e ExternalClient) LogTerminalWriteGuardSkipped(err error) {
	if e.Logger != nil {
		e.Logger.Debug("terminal write guard skipped", "err", err)
	}
}
