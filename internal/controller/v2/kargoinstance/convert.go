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

package kargoinstance

import (
	"fmt"

	kargov1 "github.com/akuity/api-client-go/pkg/api/gen/kargo/v1"

	"github.com/akuityio/provider-crossplane-akuity/apis/core/v1alpha2"
	"github.com/akuityio/provider-crossplane-akuity/internal/convert"
	"github.com/akuityio/provider-crossplane-akuity/internal/marshal"
	akuitytypes "github.com/akuityio/provider-crossplane-akuity/internal/types/generated/akuity/v1alpha1"
)

// apiToSpec rebuilds the v1alpha2 KargoInstance parameters from the
// Kargo-plane response. The nested KargoInstanceSpec sub-tree flows
// through the generated converter; wrapper-level fields
// (Version, Description, Fqdn, Subdomain, OidcConfig) go through the
// marshal bridge because the wire Kargo payload on the response side
// carries them inline on *kargov1.KargoInstance rather than as
// KargoSpec.
func apiToSpec(ki *kargov1.KargoInstance) (v1alpha2.KargoInstanceParameters, error) {
	wire, err := kargoInstanceToWireSpec(ki)
	if err != nil {
		return v1alpha2.KargoInstanceParameters{}, err
	}

	params := v1alpha2.KargoInstanceParameters{
		Name: ki.GetName(),
		Spec: v1alpha2.KargoSpec{
			Description: ki.GetDescription(),
			Version:     ki.GetVersion(),
			Fqdn:        ki.GetFqdn(),
			Subdomain:   ki.GetSubdomain(),
		},
	}
	if s := convert.KargoInstanceSpecAPIToSpec(&wire); s != nil {
		params.Spec.KargoInstanceSpec = *s
	}
	if oidc := convert.KargoOidcConfigAPIToSpec(wireOIDC(ki)); oidc != nil {
		params.Spec.OidcConfig = oidc
	}
	return params, nil
}

// apiToObservation builds the AtProvider status block.
func apiToObservation(ki *kargov1.KargoInstance) v1alpha2.KargoInstanceObservation {
	obs := v1alpha2.KargoInstanceObservation{
		ID:                    ki.GetId(),
		Name:                  ki.GetName(),
		Hostname:              ki.GetHostname(),
		OwnerOrganizationName: ki.GetOwnerOrganizationName(),
	}
	if h := ki.GetHealthStatus(); h != nil {
		obs.HealthStatus = v1alpha2.ResourceStatusCode{Code: int32(h.GetCode()), Message: h.GetMessage()}
	}
	if r := ki.GetReconciliationStatus(); r != nil {
		obs.ReconciliationStatus = v1alpha2.ResourceStatusCode{Code: int32(r.GetCode()), Message: r.GetMessage()}
	}
	return obs
}

// kargoInstanceToWireSpec extracts the KargoInstanceSpec inner type
// (IpAllowList, AgentCustomizationDefaults, etc.) from the protobuf
// KargoInstance. Only the KargoInstanceSpec-typed fields are
// forwarded; wrapper-level fields on KargoInstance are plucked
// separately in apiToSpec.
func kargoInstanceToWireSpec(ki *kargov1.KargoInstance) (akuitytypes.KargoInstanceSpec, error) {
	spec := ki.GetSpec()
	if spec == nil {
		return akuitytypes.KargoInstanceSpec{}, nil
	}
	m, err := marshal.ProtoToMap(spec)
	if err != nil {
		return akuitytypes.KargoInstanceSpec{}, fmt.Errorf("encode KargoInstanceSpec: %w", err)
	}
	wire := akuitytypes.KargoInstanceSpec{}
	if err := marshal.RemarshalTo(m, &wire); err != nil {
		return akuitytypes.KargoInstanceSpec{}, fmt.Errorf("decode KargoInstanceSpec: %w", err)
	}
	return wire, nil
}

// wireOIDC pulls the OidcConfig from the protobuf KargoInstance into
// the hand-authored akuity wire type so the generated converter can produce
// a v1alpha2 KargoOidcConfig.
func wireOIDC(ki *kargov1.KargoInstance) *akuitytypes.KargoOidcConfig {
	oidc := ki.GetOidcConfig()
	if oidc == nil {
		return nil
	}
	m, err := marshal.ProtoToMap(oidc)
	if err != nil {
		return nil
	}
	out := akuitytypes.KargoOidcConfig{}
	if err := marshal.RemarshalTo(m, &out); err != nil {
		return nil
	}
	return &out
}
