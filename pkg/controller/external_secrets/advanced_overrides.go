package external_secrets

import (
	"bytes"
	"encoding/json"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/strategicpatch"

	operatorv1alpha1 "github.com/openshift/external-secrets-operator/api/v1alpha1"
	"github.com/openshift/external-secrets-operator/pkg/controller/common"
)

// allowedPodSpecFields are the pod-level fields permitted in advancedOverrides.
var allowedPodSpecFields = map[string]bool{
	"affinity":                  true,
	"tolerations":               true,
	"nodeSelector":              true,
	"topologySpreadConstraints": true,
	"containers":                true,
}

// allowedContainerFields are the container-level fields permitted in advancedOverrides.
// "name" is required for strategic merge matching but is not an override target.
var allowedContainerFields = map[string]bool{
	"name":      true,
	"args":      true,
	"resources": true,
}

// applyAdvancedOverrides validates the advancedOverrides patch against the allowlist,
// applies it as a strategic merge patch to the Deployment, and re-asserts leader election
// on the core controller when replicas > 1. Returns a UserConfigurationError on disallowed
// paths or invalid patch data so the reconciler sets Degraded status.
func applyAdvancedOverrides(deployment *appsv1.Deployment, overrides *runtime.RawExtension, componentName operatorv1alpha1.ComponentName, containerName string) error {
	if overrides == nil || overrides.Raw == nil {
		return nil
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(overrides.Raw, &raw); err != nil {
		return common.NewUserConfigurationError(err, "advancedOverrides contains invalid JSON")
	}

	if err := validateOverridePaths(raw, containerName); err != nil {
		return err
	}

	if err := strategicMergePatchDeployment(deployment, overrides.Raw); err != nil {
		return common.NewUserConfigurationError(err, "failed to apply advancedOverrides strategic merge patch")
	}

	if componentName == operatorv1alpha1.CoreController {
		for i := range deployment.Spec.Template.Spec.Containers {
			if deployment.Spec.Template.Spec.Containers[i].Name == containerName {
				applyLeaderElection(&deployment.Spec.Template.Spec.Containers[i], deployment.Spec.Replicas)
				break
			}
		}
	}

	return nil
}

// validateOverridePaths walks the unstructured object and rejects any path not on the allowlist.
func validateOverridePaths(raw map[string]interface{}, validContainerName string) error {
	overrideBytes, err := json.Marshal(raw)
	if err != nil {
		return common.NewUserConfigurationError(err, "failed to marshal override for validation")
	}
	decoder := json.NewDecoder(bytes.NewReader(overrideBytes))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&appsv1.Deployment{}); err != nil {
		return common.NewUserConfigurationError(err, "advancedOverrides contains invalid deployment fields")
	}

	specMap, exists, err := unstructured.NestedMap(raw, "spec")
	if err != nil || !exists {
		return nil
	}

	for key := range specMap {
		if key != "template" {
			return disallowedPathError("spec."+key, firstClassHint(key))
		}
	}

	tmplMap, exists, err := unstructured.NestedMap(specMap, "template")
	if err != nil || !exists {
		return nil
	}

	for key := range tmplMap {
		if key != "spec" {
			return disallowedPathError("spec.template."+key, "use the first-class annotations/labels fields instead")
		}
	}

	podSpecMap, exists, err := unstructured.NestedMap(tmplMap, "spec")
	if err != nil || !exists {
		return nil
	}

	for key := range podSpecMap {
		if !allowedPodSpecFields[key] {
			return disallowedPathError("spec.template.spec."+key, "")
		}
	}

	containers, exists, err := unstructured.NestedSlice(podSpecMap, "containers")
	if err != nil || !exists {
		return nil
	}

	for _, entry := range containers {
		cMap, ok := entry.(map[string]interface{})
		if !ok {
			continue
		}

		name, _, _ := unstructured.NestedString(cMap, "name")
		if name == "" {
			return common.NewUserConfigurationError(
				fmt.Errorf("container name is required in advancedOverrides"),
				"each container entry in advancedOverrides must specify a name",
			)
		}
		if name != validContainerName {
			return common.NewUserConfigurationError(
				fmt.Errorf("container %q is not a valid target for this component", name),
				"advancedOverrides container name must match the component container %q", validContainerName,
			)
		}

		for key := range cMap {
			if !allowedContainerFields[key] {
				return disallowedPathError("spec.template.spec.containers[]."+key, "")
			}
		}
	}

	return nil
}

// disallowedPathError builds a UserConfigurationError for a rejected path.
func disallowedPathError(path, hint string) *common.ReconcileError {
	msg := fmt.Sprintf("%s is not allowed in advancedOverrides", path)
	if hint != "" {
		msg += "; " + hint
	}
	return common.NewUserConfigurationError(fmt.Errorf("%s", msg), "%s is not allowed in advancedOverrides", path)
}

// firstClassHint returns a user-friendly hint for deployment-level fields
// that have first-class API equivalents.
func firstClassHint(key string) string {
	switch key {
	case "replicas":
		return "use the first-class deploymentConfigs.replicas field instead"
	case "revisionHistoryLimit":
		return "use the first-class deploymentConfigs.revisionHistoryLimit field instead"
	case "selector":
		return "spec.selector is operator-managed and cannot be overridden"
	default:
		return ""
	}
}

// strategicMergePatchDeployment applies the raw patch bytes as a strategic merge patch
// to the Deployment object.
func strategicMergePatchDeployment(deployment *appsv1.Deployment, patchBytes []byte) error {
	original, err := json.Marshal(deployment)
	if err != nil {
		return fmt.Errorf("failed to marshal deployment: %w", err)
	}

	patched, err := strategicpatch.StrategicMergePatch(original, patchBytes, &appsv1.Deployment{})
	if err != nil {
		return fmt.Errorf("strategic merge patch failed: %w", err)
	}

	if err := json.Unmarshal(patched, deployment); err != nil {
		return fmt.Errorf("failed to unmarshal patched deployment: %w", err)
	}

	return nil
}
