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
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/akuityio/provider-crossplane-akuity/internal/clients/akuity"
)

// ExternalClient carries the dependencies every external client
// needs. Per-resource clients embed it.
type ExternalClient struct {
	Client   akuity.Client
	Kube     client.Client
	Logger   logging.Logger
	Recorder event.Recorder
}

// NewExternalClient is a small constructor used by the per-resource
// ExternalClientBuilder glue.
func NewExternalClient(c akuity.Client, kube client.Client, logger logging.Logger, recorder event.Recorder) ExternalClient {
	return ExternalClient{Client: c, Kube: kube, Logger: logger, Recorder: recorder}
}
