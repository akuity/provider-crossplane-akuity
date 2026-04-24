//go:build envtest

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

// Package envtest hosts CRD admission-time integration tests that boot
// a real kube-apiserver (via sigs.k8s.io/controller-runtime/pkg/envtest)
// and install the provider's CRDs so CEL rules on the shipped schemas
// can be exercised end-to-end.
//
// Unit tests against fake kube clients cover controller logic. This
// harness covers the seams that only surface with a real apiserver:
// CEL validation on Create/Update, immutability rules, and regex
// constraints on named fields.
//
// The build tag `envtest` keeps this suite out of `go test ./...` so
// callers without setup-envtest provisioned binaries don't hit a
// hard-fail. `make test-envtest` installs the kube-apiserver+etcd
// binaries and sets KUBEBUILDER_ASSETS before invoking the tagged run.
package envtest_test

import (
	"os"
	"path/filepath"
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	corev1alpha1 "github.com/akuityio/provider-crossplane-akuity/apis/core/v1alpha1"
	apisv1alpha1 "github.com/akuityio/provider-crossplane-akuity/apis/v1alpha1"
)

var (
	testEnv *envtest.Environment
	cfg     *rest.Config
	scheme  *runtime.Scheme
	kube    client.Client
)

func TestMain(m *testing.M) {
	s := runtime.NewScheme()
	if err := corev1alpha1.SchemeBuilder.AddToScheme(s); err != nil {
		panic("add core/v1alpha1 to scheme: " + err.Error())
	}
	if err := apisv1alpha1.SchemeBuilder.AddToScheme(s); err != nil {
		panic("add v1alpha1 to scheme: " + err.Error())
	}
	scheme = s

	testEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "..", "package", "crds")},
		ErrorIfCRDPathMissing: true,
	}
	var err error
	cfg, err = testEnv.Start()
	if err != nil {
		panic("start envtest: " + err.Error())
	}

	kube, err = client.New(cfg, client.Options{Scheme: scheme})
	if err != nil {
		_ = testEnv.Stop()
		panic("build client: " + err.Error())
	}

	code := m.Run()

	if err := testEnv.Stop(); err != nil {
		println("envtest stop:", err.Error())
	}
	os.Exit(code)
}
