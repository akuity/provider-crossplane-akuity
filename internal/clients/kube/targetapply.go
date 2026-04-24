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

package kube

import (
	"context"
	"errors"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
)

// TargetKubeConfig describes how a controller should reach the
// managed cluster to install / uninstall agent manifests. Shared by
// Cluster and KargoAgent controllers so the in-cluster-vs-SecretRef
// branching + manifest apply pipeline lives in one place.
type TargetKubeConfig struct {
	EnableInCluster bool
	SecretName      string
	SecretNamespace string
}

// HasKubeConfig reports whether either kubeconfig source is configured.
// Callers use this to decide whether to perform target-cluster apply.
func (t TargetKubeConfig) HasKubeConfig() bool {
	return t.EnableInCluster || t.SecretName != ""
}

// RestConfig resolves the TargetKubeConfig into a client-go rest.Config,
// either by reading the pod's in-cluster config or by loading the
// named Secret's "kubeconfig" key.
func RestConfig(ctx context.Context, c client.Client, t TargetKubeConfig) (*rest.Config, error) {
	if t.EnableInCluster {
		cfg, err := rest.InClusterConfig()
		if err != nil {
			return nil, fmt.Errorf("could not build in-cluster kube config: %w", err)
		}
		return cfg, nil
	}
	if t.SecretName == "" {
		return nil, errors.New("kubeconfig secret reference is missing a Name")
	}
	secret := &corev1.Secret{}
	if err := c.Get(ctx, k8stypes.NamespacedName{Name: t.SecretName, Namespace: t.SecretNamespace}, secret); err != nil {
		return nil, fmt.Errorf("could not get secret %s/%s containing kubeconfig: %w", t.SecretNamespace, t.SecretName, err)
	}
	data, ok := secret.Data["kubeconfig"]
	if !ok {
		return nil, fmt.Errorf("secret %s/%s has no \"kubeconfig\" key", t.SecretNamespace, t.SecretName)
	}
	cfg, err := clientcmd.RESTConfigFromKubeConfig(data)
	if err != nil {
		return nil, fmt.Errorf("could not parse kubeconfig from secret %s/%s: %w", t.SecretNamespace, t.SecretName, err)
	}
	return cfg, nil
}

// ApplyManifestsToTarget builds a REST config from t, constructs an
// ApplyClient against it, and applies (or deletes when del is true)
// the supplied manifests string. All three steps carry their own
// error-wrap so the caller can distinguish config, client, and apply
// failures in logs.
func ApplyManifestsToTarget(ctx context.Context, c client.Client, logger logging.Logger, t TargetKubeConfig, manifests string, del bool) error {
	cfg, err := RestConfig(ctx, c, t)
	if err != nil {
		return err
	}
	dyn, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return fmt.Errorf("error creating dynamic client: %w", err)
	}
	cs, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return fmt.Errorf("error creating typed client: %w", err)
	}
	ac, err := NewApplyClient(dyn, cs, logger)
	if err != nil {
		return fmt.Errorf("error creating apply client: %w", err)
	}
	return ac.ApplyManifests(ctx, manifests, del)
}
