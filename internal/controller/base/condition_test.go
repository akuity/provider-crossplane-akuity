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

package base_test

import (
	"testing"

	xpv2 "github.com/crossplane/crossplane/apis/v2/core/v2"
	"github.com/stretchr/testify/assert"

	"github.com/akuityio/provider-crossplane-akuity/apis/core/v1alpha1"
	"github.com/akuityio/provider-crossplane-akuity/internal/controller/base"
)

func TestSetHealthCondition_Healthy(t *testing.T) {
	mg := &v1alpha1.Instance{}
	base.SetHealthCondition(mg, true)
	got := mg.Status.GetCondition(xpv2.TypeReady)
	assert.Equal(t, xpv2.Available().Reason, got.Reason)
	assert.Equal(t, xpv2.Available().Status, got.Status)
}

func TestSetHealthCondition_Unhealthy(t *testing.T) {
	mg := &v1alpha1.Instance{}
	base.SetHealthCondition(mg, false)
	got := mg.Status.GetCondition(xpv2.TypeReady)
	assert.Equal(t, xpv2.Unavailable().Reason, got.Reason)
	assert.Equal(t, xpv2.Unavailable().Status, got.Status)
}
