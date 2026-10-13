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
	"context"

	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	"github.com/pkg/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// SpecNameAsExternalName returns an initializer that sets the external-name
// to the Akuity name the controller applies, replacing Crossplane's default
// NameAsExternalName. When metadata.name differs from that name, the default
// makes Observe look up a different row than Create writes, so Create can
// run against a row that already exists. A differing external-name is
// overwritten, as Create does after a successful apply.
func SpecNameAsExternalName[T resource.Managed](kube client.Client, specName func(T) string) managed.InitializerFn {
	return func(ctx context.Context, mg resource.Managed) error {
		cr, ok := mg.(T)
		if !ok {
			return errors.Errorf("cannot set external name: unexpected managed resource type %T", mg)
		}
		name := specName(cr)
		if name == "" || meta.GetExternalName(mg) == name {
			return nil
		}
		meta.SetExternalName(mg, name)
		return errors.Wrap(kube.Update(ctx, mg), "cannot update external name")
	}
}
