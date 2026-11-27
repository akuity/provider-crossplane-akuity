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

package cluster

import (
	"fmt"
	"strings"

	"sigs.k8s.io/yaml"

	generated "github.com/akuityio/provider-crossplane-akuity/internal/types/generated/crossplane/v1alpha1"
)

const customClusterSize = "custom"

func projectCustomClusterSize(data *generated.ClusterData) error {
	if data == nil {
		return nil
	}
	if !strings.EqualFold(string(data.Size), customClusterSize) {
		if data.CustomAgentSizeConfig != nil {
			return fmt.Errorf("customAgentSizeConfig requires size custom")
		}
		return nil
	}
	if data.CustomAgentSizeConfig == nil {
		return fmt.Errorf("size custom requires customAgentSizeConfig")
	}
	if data.AutoscalerConfig != nil {
		return fmt.Errorf("size custom cannot be combined with autoscalerConfig")
	}
	kustomization, err := generateClusterCustomSizeKustomization(data.CustomAgentSizeConfig, data.Kustomization)
	if err != nil {
		return err
	}
	data.Size = "large"
	data.Kustomization = kustomization
	data.CustomAgentSizeConfig = nil
	return nil
}

func generateClusterCustomSizeKustomization(cfg *generated.ClusterCustomAgentSizeConfig, userKustomization string) (string, error) { //nolint:gocyclo
	if cfg == nil {
		return userKustomization, nil
	}
	if cfg.ApplicationController == nil && cfg.RepoServer == nil {
		return "", fmt.Errorf("customAgentSizeConfig must configure at least one component")
	}
	top, err := parseKustomizationObject(userKustomization)
	if err != nil {
		return "", err
	}
	userPatches, err := kustomizationSlice(top, "patches")
	if err != nil {
		return "", err
	}
	for _, p := range userPatches {
		patch, ok := p.(map[string]any)
		if !ok {
			continue
		}
		name, ok := patchTargetDeploymentName(patch)
		if !ok {
			continue
		}
		if name != "argocd-application-controller" && name != "argocd-repo-server" {
			continue
		}
		if isResourcePatch(patch) {
			return "", fmt.Errorf("kustomization contains resource patches for %s, which conflicts with customAgentSizeConfig", name)
		}
	}

	customPatches := make([]any, 0, 2)
	if cfg.ApplicationController != nil {
		if err := validateCustomResources("applicationController", cfg.ApplicationController.Mem, cfg.ApplicationController.Cpu); err != nil {
			return "", err
		}
		customPatches = append(customPatches, deploymentResourcePatch(
			"argocd-application-controller",
			"argocd-application-controller",
			cfg.ApplicationController.Mem,
			cfg.ApplicationController.Cpu,
		))
	}
	if cfg.RepoServer != nil {
		if err := validateCustomResources("repoServer", cfg.RepoServer.Mem, cfg.RepoServer.Cpu); err != nil {
			return "", err
		}
		if cfg.RepoServer.Replicas <= 0 {
			return "", fmt.Errorf("customAgentSizeConfig.repoServer.replicas must be greater than zero")
		}
		customPatches = append(customPatches, deploymentResourcePatch(
			"argocd-repo-server",
			"argocd-repo-server",
			cfg.RepoServer.Mem,
			cfg.RepoServer.Cpu,
		))
	}
	top["patches"] = append(customPatches, userPatches...)

	userReplicas, err := kustomizationSlice(top, "replicas")
	if err != nil {
		return "", err
	}
	if cfg.RepoServer != nil {
		top["replicas"] = append([]any{map[string]any{
			"name":  "argocd-repo-server",
			"count": cfg.RepoServer.Replicas,
		}}, userReplicas...)
	}

	out, err := yaml.Marshal(top)
	if err != nil {
		return "", fmt.Errorf("failed to marshal custom size kustomization: %w", err)
	}
	return string(out), nil
}

func parseKustomizationObject(s string) (map[string]any, error) {
	if strings.TrimSpace(s) == "" {
		return map[string]any{
			apiVersionKey: "kustomize.config.k8s.io/v1beta1",
			kindKey:       "Kustomization",
		}, nil
	}
	var top map[string]any
	if err := yaml.Unmarshal([]byte(s), &top); err != nil {
		return nil, fmt.Errorf("failed to parse user kustomization: %w", err)
	}
	if top == nil {
		top = map[string]any{}
	}
	if _, ok := top[apiVersionKey]; !ok {
		top[apiVersionKey] = "kustomize.config.k8s.io/v1beta1"
	}
	if _, ok := top[kindKey]; !ok {
		top[kindKey] = "Kustomization"
	}
	return top, nil
}

func kustomizationSlice(top map[string]any, key string) ([]any, error) {
	raw, ok := top[key]
	if !ok || raw == nil {
		return nil, nil
	}
	items, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("kustomization.%s must be a list", key)
	}
	return items, nil
}

func patchTargetDeploymentName(patch map[string]any) (string, bool) {
	target, ok := patch["target"].(map[string]any)
	if !ok {
		return "", false
	}
	if target[kindKey] != "Deployment" {
		return "", false
	}
	name, ok := target["name"].(string)
	return name, ok
}

func isResourcePatch(patch map[string]any) bool {
	patchContent, ok := patch["patch"].(string)
	if !ok {
		return false
	}
	var obj map[string]any
	if err := yaml.Unmarshal([]byte(patchContent), &obj); err != nil {
		return false
	}
	containers, ok := nestedSlice(obj, "spec", "template", "spec", "containers")
	if !ok {
		return false
	}
	for _, c := range containers {
		container, ok := c.(map[string]any)
		if !ok {
			continue
		}
		if _, ok := container["resources"]; ok {
			return true
		}
	}
	return false
}

func nestedSlice(obj map[string]any, keys ...string) ([]any, bool) {
	var cur any = obj
	for _, key := range keys {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		cur, ok = m[key]
		if !ok {
			return nil, false
		}
	}
	out, ok := cur.([]any)
	return out, ok
}

func validateCustomResources(path, mem, cpu string) error {
	if mem == "" {
		return fmt.Errorf("customAgentSizeConfig.%s.mem is required", path)
	}
	if cpu == "" {
		return fmt.Errorf("customAgentSizeConfig.%s.cpu is required", path)
	}
	return nil
}

func deploymentResourcePatch(name, container, mem, cpu string) map[string]any {
	patch := map[string]any{
		apiVersionKey: "apps/v1",
		kindKey:       "Deployment",
		"metadata": map[string]any{
			"name": name,
		},
		"spec": map[string]any{
			"template": map[string]any{
				"spec": map[string]any{
					"containers": []any{
						map[string]any{
							"name": container,
							"resources": map[string]any{
								"limits": map[string]any{
									"memory": mem,
								},
								"requests": map[string]any{
									"cpu":    cpu,
									"memory": mem,
								},
							},
						},
					},
				},
			},
		},
	}
	patchYAML, _ := yaml.Marshal(patch)
	return map[string]any{
		"patch": string(patchYAML),
		"target": map[string]any{
			kindKey: "Deployment",
			"name":  name,
		},
	}
}
