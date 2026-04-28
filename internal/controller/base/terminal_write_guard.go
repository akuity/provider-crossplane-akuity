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
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash"
	"sync"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	"google.golang.org/protobuf/proto"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/akuityio/provider-crossplane-akuity/internal/reason"
)

// TerminalWriteKey identifies one failed write attempt. The fingerprint
// should include the effective outbound payload, including resolved
// IDs and Secret hashes where those affect the write body.
type TerminalWriteKey struct {
	GVK         string
	UID         string
	Namespace   string
	Name        string
	Fingerprint string
}

// NewTerminalWriteKey creates a stable key for suppressing repeated
// terminal writes until the effective payload changes.
func NewTerminalWriteKey(mg resource.Managed, gvk schema.GroupVersionKind, parts ...any) (TerminalWriteKey, error) {
	h := sha256.New()
	for _, part := range parts {
		if err := writeTerminalFingerprintPart(h, part); err != nil {
			return TerminalWriteKey{}, err
		}
	}
	return TerminalWriteKey{
		GVK:         gvk.String(),
		UID:         string(mg.GetUID()),
		Namespace:   mg.GetNamespace(),
		Name:        mg.GetName(),
		Fingerprint: hex.EncodeToString(h.Sum(nil)),
	}, nil
}

func writeTerminalFingerprintPart(h hash.Hash, part any) error {
	if _, err := fmt.Fprintf(h, "%T\n", part); err != nil {
		return err
	}
	if msg, ok := part.(proto.Message); ok {
		b, err := proto.MarshalOptions{Deterministic: true}.Marshal(msg)
		if err != nil {
			return err
		}
		_, _ = h.Write(b)
		_, _ = h.Write([]byte{'\n'})
		return nil
	}
	enc := json.NewEncoder(h)
	if err := enc.Encode(part); err != nil {
		return err
	}
	return nil
}

// NewTerminalWriteResourceKey identifies a resource without caring
// about generation or payload. Clear uses only resource identity, so
// controllers can use this cheap key to drop cached errors on delete.
func NewTerminalWriteResourceKey(mg resource.Managed, gvk schema.GroupVersionKind) TerminalWriteKey {
	return TerminalWriteKey{
		GVK:       gvk.String(),
		UID:       string(mg.GetUID()),
		Namespace: mg.GetNamespace(),
		Name:      mg.GetName(),
	}
}

// TerminalWriteGuard remembers terminal write failures so Observe can
// stop re-scheduling the same Create/Update/Patch while preserving the
// terminal error users need to fix the input.
type TerminalWriteGuard struct {
	mu      sync.Mutex
	entries map[TerminalWriteKey]error
}

// DefaultTerminalWriteGuard is shared by controller instances in this process.
var DefaultTerminalWriteGuard = NewTerminalWriteGuard()

func NewTerminalWriteGuard() *TerminalWriteGuard {
	return &TerminalWriteGuard{entries: map[TerminalWriteKey]error{}}
}

// Record stores terminal write errors and clears stale records for the
// same resource. Non-terminal errors are intentionally ignored.
func (g *TerminalWriteGuard) Record(key TerminalWriteKey, err error) {
	if g == nil || err == nil || !reason.IsTerminal(err) {
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	g.deleteResourceLocked(key)
	g.entries[key] = err
}

// Clear removes any terminal write record for the same resource.
func (g *TerminalWriteGuard) Clear(key TerminalWriteKey) {
	if g == nil {
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	g.deleteResourceLocked(key)
}

// HasResource reports whether the guard has any terminal write entry
// for the same resource identity. Controllers use this as a cheap
// green-path fast check before resolving Secrets solely to clear a key.
func (g *TerminalWriteGuard) HasResource(key TerminalWriteKey) bool {
	if g == nil {
		return false
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	for existing := range g.entries {
		if sameTerminalWriteResource(existing, key) {
			return true
		}
	}
	return false
}

// Suppress returns the cached terminal error for an identical write that
// already failed. Returning the error from Observe prevents another write
// while keeping Synced=False/ReconcileError visible to the user.
// A changed payload fingerprint clears stale records and allows
// reconciliation to try again.
func (g *TerminalWriteGuard) Suppress(mg resource.Managed, key TerminalWriteKey) (managed.ExternalObservation, error, bool) {
	if g == nil {
		return managed.ExternalObservation{}, nil, false
	}
	if mg.GetDeletionTimestamp() != nil {
		g.mu.Lock()
		g.deleteResourceLocked(key)
		g.mu.Unlock()
		return managed.ExternalObservation{}, nil, false
	}
	g.mu.Lock()
	var err error
	ok := false
	for existing, existingErr := range g.entries {
		if sameTerminalWritePayload(existing, key) {
			err = existingErr
			ok = true
			break
		}
	}
	if !ok {
		g.deleteResourceLocked(key)
		g.mu.Unlock()
		return managed.ExternalObservation{}, nil, false
	}
	g.mu.Unlock()
	mg.SetConditions(xpv1.ReconcileError(err))
	return managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: true}, err, true
}

func (g *TerminalWriteGuard) deleteResourceLocked(key TerminalWriteKey) {
	for existing := range g.entries {
		if sameTerminalWriteResource(existing, key) {
			delete(g.entries, existing)
		}
	}
}

func sameTerminalWriteResource(a, b TerminalWriteKey) bool {
	if a.GVK != b.GVK {
		return false
	}
	if a.UID != "" && b.UID != "" {
		return a.UID == b.UID
	}
	return a.Namespace == b.Namespace && a.Name == b.Name
}

func sameTerminalWritePayload(a, b TerminalWriteKey) bool {
	return sameTerminalWriteResource(a, b) && a.Fingerprint == b.Fingerprint
}
