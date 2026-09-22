package external_secrets

import (
	"encoding/json"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/openshift/external-secrets-operator/pkg/controller/common"
)

// allowedPodSpecFields are the pod spec fields that advancedOverrides may set.
var allowedPodSpecFields = map[string]bool{
	"affinity":                  true,
	"tolerations":               true,
	"nodeSelector":              true,
	"topologySpreadConstraints": true,
	"containers":                true,
}

// allowedContainerFields are the container-level fields that advancedOverrides may set.
var allowedContainerFields = map[string]bool{
	"name":      true,
	"args":      true,
	"resources": true,
}

// partialDeployment is a minimal Deployment shape for unmarshaling advancedOverrides.
type partialDeployment struct {
	Spec *partialDeploymentSpec `json:"spec,omitempty"`
}

type partialDeploymentSpec struct {
	Template *partialPodTemplateSpec `json:"template,omitempty"`
}

type partialPodTemplateSpec struct {
	Spec *partialPodSpec `json:"spec,omitempty"`
}

type partialPodSpec struct {
	Affinity                  *corev1.Affinity                  `json:"affinity,omitempty"`
	Tolerations               []corev1.Toleration               `json:"tolerations,omitempty"`
	NodeSelector              map[string]string                 `json:"nodeSelector,omitempty"`
	TopologySpreadConstraints []corev1.TopologySpreadConstraint `json:"topologySpreadConstraints,omitempty"`
	Containers                []partialContainer                `json:"containers,omitempty"`
}

type partialContainer struct {
	Name      string                       `json:"name,omitempty"`
	Args      []string                     `json:"args,omitempty"`
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`
}

// validateAndApplyAdvancedOverrides validates that the advancedOverrides only contains
// allowlisted paths and applies the allowed fields to the deployment. Returns a
// UserConfigurationError if any disallowed path is found or the data is malformed.
func validateAndApplyAdvancedOverrides(deployment *appsv1.Deployment, overrides *runtime.RawExtension, containerName string) error {
	if overrides == nil || len(overrides.Raw) == 0 {
		return nil
	}

	// Validate paths using generic map traversal.
	if err := validateOverridePaths(overrides.Raw); err != nil {
		return err
	}

	// Parse into typed struct for application.
	var partial partialDeployment
	if err := json.Unmarshal(overrides.Raw, &partial); err != nil {
		return common.NewUserConfigurationError(err, "advancedOverrides contains invalid JSON")
	}

	applyParsedOverrides(deployment, &partial, containerName)
	return nil
}

// validateOverridePaths traverses the raw JSON to ensure only allowlisted paths are present.
func validateOverridePaths(raw []byte) error {
	var generic map[string]any
	if err := json.Unmarshal(raw, &generic); err != nil {
		return common.NewUserConfigurationError(err, "advancedOverrides contains invalid JSON")
	}

	// Top-level: only "spec" is allowed.
	for key := range generic {
		if key != "spec" {
			return common.NewUserConfigurationError(
				fmt.Errorf("disallowed top-level key %q in advancedOverrides", key),
				"advancedOverrides path %q is not on the allowlist", key,
			)
		}
	}

	specMap, ok := generic["spec"].(map[string]any)
	if !ok {
		return nil
	}

	// spec level: only "template" is allowed.
	for key := range specMap {
		if key != "template" {
			return common.NewUserConfigurationError(
				fmt.Errorf("disallowed key %q under spec in advancedOverrides", key),
				"advancedOverrides path spec.%s is not on the allowlist; use the first-class API field instead", key,
			)
		}
	}

	templateMap, ok := specMap["template"].(map[string]any)
	if !ok {
		return nil
	}

	// template level: only "spec" is allowed (not "metadata").
	for key := range templateMap {
		if key != "spec" {
			return common.NewUserConfigurationError(
				fmt.Errorf("disallowed key %q under spec.template in advancedOverrides", key),
				"advancedOverrides path spec.template.%s is not on the allowlist", key,
			)
		}
	}

	podSpecMap, ok := templateMap["spec"].(map[string]any)
	if !ok {
		return nil
	}

	// pod spec level: only allowlisted fields.
	for key := range podSpecMap {
		if !allowedPodSpecFields[key] {
			return common.NewUserConfigurationError(
				fmt.Errorf("disallowed key %q under spec.template.spec in advancedOverrides", key),
				"advancedOverrides path spec.template.spec.%s is not on the allowlist", key,
			)
		}
	}

	// Validate container-level fields.
	if containers, ok := podSpecMap["containers"]; ok {
		containersList, ok := containers.([]any)
		if !ok {
			return common.NewUserConfigurationError(
				fmt.Errorf("containers in advancedOverrides must be an array"),
				"advancedOverrides spec.template.spec.containers must be an array",
			)
		}
		for i, c := range containersList {
			containerMap, ok := c.(map[string]any)
			if !ok {
				continue
			}
			for key := range containerMap {
				if !allowedContainerFields[key] {
					return common.NewUserConfigurationError(
						fmt.Errorf("disallowed container field %q in advancedOverrides containers[%d]", key, i),
						"advancedOverrides path spec.template.spec.containers[%d].%s is not on the allowlist", i, key,
					)
				}
			}
		}
	}

	return nil
}

// applyParsedOverrides applies the validated partial deployment overrides to the target deployment.
func applyParsedOverrides(deployment *appsv1.Deployment, partial *partialDeployment, containerName string) {
	if partial.Spec == nil || partial.Spec.Template == nil || partial.Spec.Template.Spec == nil {
		return
	}

	ps := partial.Spec.Template.Spec

	if ps.Affinity != nil {
		deployment.Spec.Template.Spec.Affinity = ps.Affinity
	}
	if ps.Tolerations != nil {
		deployment.Spec.Template.Spec.Tolerations = ps.Tolerations
	}
	if ps.NodeSelector != nil {
		deployment.Spec.Template.Spec.NodeSelector = ps.NodeSelector
	}
	if ps.TopologySpreadConstraints != nil {
		deployment.Spec.Template.Spec.TopologySpreadConstraints = ps.TopologySpreadConstraints
	}

	// Apply container-level overrides only to the target container.
	for _, pc := range ps.Containers {
		if pc.Name != "" && pc.Name != containerName {
			continue
		}
		for i := range deployment.Spec.Template.Spec.Containers {
			if deployment.Spec.Template.Spec.Containers[i].Name != containerName {
				continue
			}
			if pc.Args != nil {
				deployment.Spec.Template.Spec.Containers[i].Args = pc.Args
			}
			if pc.Resources != nil {
				deployment.Spec.Template.Spec.Containers[i].Resources = *pc.Resources
			}
			break
		}
	}
}
