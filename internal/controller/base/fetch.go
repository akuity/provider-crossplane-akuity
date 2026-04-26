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
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"

	"github.com/akuityio/provider-crossplane-akuity/internal/reason"
)

// GetOutcome categorizes a *Get* call's error into the branches every
// Observe repeats. GetOK means the caller's remote object is usable;
// any other outcome carries a pre-shaped ExternalObservation the caller
// can return directly.
type GetOutcome int

const (
	// GetOK signals that err == nil; the remote object is usable.
	GetOK GetOutcome = iota
	// GetAbsent signals that the resource does not exist on the Akuity
	// side (NotFound). Caller returns ResourceExists=false.
	GetAbsent
	// GetProvisioning signals a transient wait on upstream provisioning.
	// Caller reports up-to-date + Unavailable so reconciliation does not
	// thrash while the parent resource is still coming up.
	GetProvisioning
	// GetTerminal signals a non-transient gateway error. Caller returns
	// the original err; the managed reconciler surfaces it as
	// ReconcileError. Callers are expected to set
	// xpv1.ReconcileError(err) on mg before returning.
	GetTerminal
)

// ClassifyGetError maps err into one of the GetOutcome branches and
// returns a pre-populated ExternalObservation + the error to return.
// Callers are expected to set conditions on mg (Unavailable for
// Provisioning, ReconcileError for Terminal) before returning; this
// function intentionally does not touch mg so it can be used from
// shared helpers that don't hold a typed managed resource.
func ClassifyGetError(err error) (GetOutcome, managed.ExternalObservation, error) {
	if err == nil {
		return GetOK, managed.ExternalObservation{}, nil
	}
	if reason.IsNotFound(err) {
		return GetAbsent, managed.ExternalObservation{ResourceExists: false}, nil
	}
	if reason.IsProvisioningWait(err) {
		return GetProvisioning, managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: true}, nil
	}
	return GetTerminal, managed.ExternalObservation{}, err
}
