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

package v1alpha2

// LocalReference is a same-namespace reference by name.
//
// Cross-namespace references are not supported in v1alpha2; the
// namespace of the target is always the namespace of the managed
// resource holding the reference.
type LocalReference struct {
	// Name of the referenced object. Required.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`
}

// LocalSecretKeySelector selects a specific key from a Secret in the
// same namespace as the referring managed resource.
type LocalSecretKeySelector struct {
	// Name of the Secret. Required.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`

	// Key within the Secret. Required.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Key string `json:"key"`
}

// ResourceStatusCode captures the Akuity API status code + message pair
// exposed on most observable resources.
type ResourceStatusCode struct {
	// Code reported by the Akuity API.
	Code int32 `json:"code,omitempty"`
	// Message reported by the Akuity API.
	Message string `json:"message,omitempty"`
}
