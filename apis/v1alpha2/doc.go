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

// Package v1alpha2 defines the namespaced ProviderConfig and the
// cluster-scoped ClusterProviderConfig variants consumed by v1alpha2
// managed resources, served at akuity.m.crossplane.io. Both point at
// an Akuity Platform organization and a credential source; the split
// follows the Crossplane v2 pattern where namespaced MRs may reference
// either a namespace-local ProviderConfig or a global
// ClusterProviderConfig.
//
// The legacy akuity.crossplane.io/v1alpha1 ProviderConfig continues to
// serve v1alpha1 cluster-scoped MRs and is unaffected by the v1alpha2
// types defined here — the distinct group eliminates CRD-scope
// conflicts between the cluster-scoped legacy variant and the
// namespaced v1alpha2 variant.
package v1alpha2
