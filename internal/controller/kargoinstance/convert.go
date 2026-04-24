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

	"github.com/akuityio/provider-crossplane-akuity/apis/core/v1alpha1"
	"github.com/akuityio/provider-crossplane-akuity/internal/marshal"
	akuitytypes "github.com/akuityio/provider-crossplane-akuity/internal/types/generated/akuity/v1alpha1"
	crossplanetypes "github.com/akuityio/provider-crossplane-akuity/internal/types/generated/crossplane/v1alpha1"
)

// apiToSpec rebuilds the KargoInstance parameters from the
// Kargo-plane response. The nested KargoInstanceSpec sub-tree flows
// through the generated converter; wrapper-level fields
// (Version, Description, Fqdn, Subdomain, OidcConfig) go through the
// marshal bridge because the wire Kargo payload on the response side
// carries them inline on *kargov1.KargoInstance rather than as
// KargoSpec.
func apiToSpec(ki *kargov1.KargoInstance) (v1alpha1.KargoInstanceParameters, error) {
	wire, err := marshal.ProtoToWire[akuitytypes.KargoInstanceSpec](ki.GetSpec())
	if err != nil {
		return v1alpha1.KargoInstanceParameters{}, fmt.Errorf("decode KargoInstanceSpec: %w", err)
	}
	oidc, err := marshal.ProtoToWire[akuitytypes.KargoOidcConfig](ki.GetOidcConfig())
	if err != nil {
		return v1alpha1.KargoInstanceParameters{}, fmt.Errorf("decode KargoOidcConfig: %w", err)
	}

	params := v1alpha1.KargoInstanceParameters{
		Name: ki.GetName(),
		Kargo: crossplanetypes.KargoSpec{
			Description: ki.GetDescription(),
			Version:     ki.GetVersion(),
			Fqdn:        ki.GetFqdn(),
			Subdomain:   ki.GetSubdomain(),
		},
	}
	if s := crossplanetypes.KargoInstanceSpecAPIToSpec(&wire); s != nil {
		params.Kargo.KargoInstanceSpec = *s
	}
	if ki.GetOidcConfig() != nil {
		if converted := crossplanetypes.KargoOidcConfigAPIToSpec(&oidc); converted != nil {
			params.Kargo.OidcConfig = converted
		}
	}
	return params, nil
}
