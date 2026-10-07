/*
Copyright 2020 The Crossplane Authors.

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

package controller

import (
	"github.com/crossplane/crossplane-runtime/v2/pkg/controller"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/akuityio/provider-crossplane-akuity/apis/core/v1alpha1"
	apisv1alpha1 "github.com/akuityio/provider-crossplane-akuity/apis/v1alpha1"
	"github.com/akuityio/provider-crossplane-akuity/internal/controller/cluster"
	"github.com/akuityio/provider-crossplane-akuity/internal/controller/config"
	"github.com/akuityio/provider-crossplane-akuity/internal/controller/instance"
	"github.com/akuityio/provider-crossplane-akuity/internal/controller/instanceipallowlist"
	"github.com/akuityio/provider-crossplane-akuity/internal/controller/kargoagent"
	"github.com/akuityio/provider-crossplane-akuity/internal/controller/kargodefaultshardagent"
	"github.com/akuityio/provider-crossplane-akuity/internal/controller/kargoinstance"
)

// Groups lists the API groups served by this provider. The CRD gate only
// watches CustomResourceDefinitions of these groups.
var Groups = []string{apisv1alpha1.Group, v1alpha1.Group}

// Setup creates all akuity controllers with the supplied logger and adds them to
// the supplied manager.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	for _, setup := range []func(ctrl.Manager, controller.Options) error{
		config.Setup,
		instance.Setup,
		cluster.Setup,
		instanceipallowlist.Setup,
		kargoinstance.Setup,
		kargoagent.Setup,
		kargodefaultshardagent.Setup,
	} {
		if err := setup(mgr, o); err != nil {
			return err
		}
	}

	return nil
}

// SetupGated starts each controller once the CRDs of its kinds are established.
func SetupGated(mgr ctrl.Manager, o controller.Options) error {
	for _, c := range []struct {
		setup func(ctrl.Manager, controller.Options) error
		gvks  []schema.GroupVersionKind
	}{
		{config.Setup, []schema.GroupVersionKind{apisv1alpha1.ProviderConfigGroupVersionKind, apisv1alpha1.ProviderConfigUsageGroupVersionKind}},
		{instance.Setup, []schema.GroupVersionKind{v1alpha1.InstanceGroupVersionKind}},
		{cluster.Setup, []schema.GroupVersionKind{v1alpha1.ClusterGroupVersionKind}},
		{instanceipallowlist.Setup, []schema.GroupVersionKind{v1alpha1.InstanceIpAllowListGroupVersionKind}},
		{kargoinstance.Setup, []schema.GroupVersionKind{v1alpha1.KargoInstanceGroupVersionKind}},
		{kargoagent.Setup, []schema.GroupVersionKind{v1alpha1.KargoAgentGroupVersionKind}},
		{kargodefaultshardagent.Setup, []schema.GroupVersionKind{v1alpha1.KargoDefaultShardAgentGroupVersionKind}},
	} {
		o.Gate.Register(func() {
			if err := c.setup(mgr, o); err != nil {
				mgr.GetLogger().Error(err, "unable to setup reconciler", "gvks", c.gvks)
			}
		}, c.gvks...)
	}

	return nil
}
