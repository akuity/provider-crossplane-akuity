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

package kargoagent

import (
	"fmt"
	"strings"

	"sigs.k8s.io/yaml"

	generated "github.com/akuityio/provider-crossplane-akuity/internal/types/generated/crossplane/v1alpha1"
)

const customKargoAgentSize = "custom"

func projectCustomKargoAgentSize(name string, data *generated.KargoAgentData) error {
	if data == nil {
		return nil
	}
	if !strings.EqualFold(string(data.Size), customKargoAgentSize) {
		if data.CustomAgentSizeConfig != nil {
			return fmt.Errorf("customAgentSizeConfig requires size custom")
		}
		return nil
	}
	if data.AkuityManaged != nil && *data.AkuityManaged {
		return fmt.Errorf("size custom is not allowed for akuityManaged Kargo agents")
	}
	if data.CustomAgentSizeConfig == nil {
		return fmt.Errorf("size custom requires customAgentSizeConfig")
	}
	if data.AutoscalerConfig != nil {
		return fmt.Errorf("size custom cannot be combined with autoscalerConfig")
	}
	kustomization, err := generateKargoAgentCustomSizeKustomization(name, data.CustomAgentSizeConfig, data.Kustomization)
	if err != nil {
		return err
	}
	data.Size = "large"
	data.Kustomization = kustomization
	data.CustomAgentSizeConfig = nil
	return nil
}

func generateKargoAgentCustomSizeKustomization(name string, cfg *generated.KargoAgentCustomAgentSizeConfig, userKustomization string) (string, error) { //nolint:gocyclo
	if cfg == nil {
		return userKustomization, nil
	}
	if cfg.KargoController == nil {
		return "", fmt.Errorf("customAgentSizeConfig.kargoController is required")
	}
	if cfg.KargoController.Mem == "" {
		return "", fmt.Errorf("customAgentSizeConfig.kargoController.mem is required")
	}
	if cfg.KargoController.Cpu == "" {
		return "", fmt.Errorf("customAgentSizeConfig.kargoController.cpu is required")
	}
	top, err := parseKargoAgentKustomizationObject(userKustomization)
	if err != nil {
		return "", err
	}
	userPatches, err := kargoAgentKustomizationSlice(top, "patches")
	if err != nil {
		return "", err
	}
	targetName := fmt.Sprintf("kargo-controller-%s", name)
	for _, p := range userPatches {
		patch, ok := p.(map[string]interface{})
		if !ok {
			continue
		}
		if patchTargetDeploymentName(patch) != targetName {
			continue
		}
		if kargoAgentResourcePatch(patch) {
			return "", fmt.Errorf("kustomization contains resource patches for %s, which conflicts with customAgentSizeConfig", targetName)
		}
	}
	top["patches"] = append([]interface{}{
		kargoAgentDeploymentResourcePatch(targetName, cfg.KargoController.Mem, cfg.KargoController.Cpu),
	}, userPatches...)

	out, err := yaml.Marshal(top)
	if err != nil {
		return "", fmt.Errorf("failed to marshal custom size kustomization: %w", err)
	}
	return string(out), nil
}

func parseKargoAgentKustomizationObject(s string) (map[string]interface{}, error) {
	if strings.TrimSpace(s) == "" {
		return map[string]interface{}{
			"apiVersion": "kustomize.config.k8s.io/v1beta1",
			"kind":       "Kustomization",
		}, nil
	}
	var top map[string]interface{}
	if err := yaml.Unmarshal([]byte(s), &top); err != nil {
		return nil, fmt.Errorf("failed to parse user kustomization: %w", err)
	}
	if top == nil {
		top = map[string]interface{}{}
	}
	if _, ok := top["apiVersion"]; !ok {
		top["apiVersion"] = "kustomize.config.k8s.io/v1beta1"
	}
	if _, ok := top["kind"]; !ok {
		top["kind"] = "Kustomization"
	}
	return top, nil
}

func kargoAgentKustomizationSlice(top map[string]interface{}, key string) ([]interface{}, error) {
	raw, ok := top[key]
	if !ok || raw == nil {
		return nil, nil
	}
	items, ok := raw.([]interface{})
	if !ok {
		return nil, fmt.Errorf("kustomization.%s must be a list", key)
	}
	return items, nil
}

func patchTargetDeploymentName(patch map[string]interface{}) string {
	target, ok := patch["target"].(map[string]interface{})
	if !ok || target["kind"] != "Deployment" {
		return ""
	}
	name, _ := target["name"].(string)
	return name
}

func kargoAgentResourcePatch(patch map[string]interface{}) bool {
	patchContent, ok := patch["patch"].(string)
	if !ok {
		return false
	}
	var obj map[string]interface{}
	if err := yaml.Unmarshal([]byte(patchContent), &obj); err != nil {
		return false
	}
	containers, ok := nestedKargoAgentSlice(obj, "spec", "template", "spec", "containers")
	if !ok {
		return false
	}
	for _, c := range containers {
		container, ok := c.(map[string]interface{})
		if !ok {
			continue
		}
		if _, ok := container["resources"]; ok {
			return true
		}
	}
	return false
}

func nestedKargoAgentSlice(obj map[string]interface{}, keys ...string) ([]interface{}, bool) {
	var cur interface{} = obj
	for _, key := range keys {
		m, ok := cur.(map[string]interface{})
		if !ok {
			return nil, false
		}
		cur, ok = m[key]
		if !ok {
			return nil, false
		}
	}
	out, ok := cur.([]interface{})
	return out, ok
}

func kargoAgentDeploymentResourcePatch(name, mem, cpu string) map[string]interface{} {
	patch := map[string]interface{}{
		"apiVersion": "apps/v1",
		"kind":       "Deployment",
		"metadata": map[string]interface{}{
			"name": name,
		},
		"spec": map[string]interface{}{
			"template": map[string]interface{}{
				"spec": map[string]interface{}{
					"containers": []interface{}{
						map[string]interface{}{
							"name": "controller",
							"resources": map[string]interface{}{
								"limits": map[string]interface{}{
									"memory": mem,
								},
								"requests": map[string]interface{}{
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
	return map[string]interface{}{
		"patch": string(patchYAML),
		"target": map[string]interface{}{
			"kind": "Deployment",
			"name": name,
		},
	}
}
