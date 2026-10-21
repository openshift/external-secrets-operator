package external_secrets

import (
	"bytes"
	"encoding/json"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/apimachinery/pkg/util/validation/field"

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
// applies it as a strategic merge patch to the Deployment. Returns a UserConfigurationError on disallowed
// paths or invalid patch data so the reconciler sets Degraded status.
func applyAdvancedOverrides(deployment *appsv1.Deployment, overrides *runtime.RawExtension, containerName string) error {
	if overrides == nil || overrides.Raw == nil {
		return nil
	}

	var raw map[string]any
	if err := json.Unmarshal(overrides.Raw, &raw); err != nil {
		return common.NewUserConfigurationError(err, "advancedOverrides contains invalid JSON")
	}

	if err := validateOverridePaths(overrides.Raw, raw, containerName); err != nil {
		return err
	}

	if err := strategicMergePatchDeployment(deployment, overrides.Raw); err != nil {
		return common.NewUserConfigurationError(err, "failed to apply advancedOverrides strategic merge patch")
	}

	return nil
}

// validateOverridePaths walks the unstructured object and rejects any path not on the allowlist.
// rawBytes is the original overrides.Raw payload, reused here for the strict-decode check below
// instead of re-marshaling raw, since both represent the same already-parsed JSON document.
func validateOverridePaths(rawBytes []byte, raw map[string]any, validContainerName string) error {
	decoder := json.NewDecoder(bytes.NewReader(rawBytes))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&appsv1.Deployment{}); err != nil {
		return common.NewUserConfigurationError(err, "advancedOverrides contains invalid deployment fields")
	}

	for key := range raw {
		if key != "spec" {
			return forbiddenFieldError(field.NewPath(key), "")
		}
	}

	specPath := field.NewPath("spec")
	specMap, err := validateAllowedFields(raw, "spec", specPath,
		func(key string) bool { return key == "template" }, firstClassHint)
	if err != nil {
		return err
	}

	tmplPath := specPath.Child("template")
	tmplMap, err := validateAllowedFields(specMap, "template", tmplPath,
		func(key string) bool { return key == "spec" },
		func(_ string) string { return "use the first-class annotations/labels fields instead" })
	if err != nil {
		return err
	}

	podSpecPath := tmplPath.Child("spec")
	podSpecMap, err := validateAllowedFields(tmplMap, "spec", podSpecPath,
		func(key string) bool { return allowedPodSpecFields[key] }, nil)
	if err != nil {
		return err
	}

	containersPath := podSpecPath.Child("containers")
	containers, exists, err := unstructured.NestedSlice(podSpecMap, "containers")
	if err != nil {
		return malformedFieldError(containersPath, err)
	}
	if !exists {
		return nil
	}

	for i, entry := range containers {
		cMap, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		containerPath := containersPath.Index(i)

		name, _, _ := unstructured.NestedString(cMap, "name")
		if name == "" {
			return common.NewUserConfigurationError(
				field.Required(containerPath.Child("name"), "container name is required in advancedOverrides"),
				"each container entry in advancedOverrides must specify a name",
			)
		}
		if name != validContainerName {
			return common.NewUserConfigurationError(
				field.Invalid(containerPath.Child("name"), name, fmt.Sprintf("must match the component container %q", validContainerName)),
				"advancedOverrides container name must match the component container %q", validContainerName,
			)
		}

		for key := range cMap {
			if allowedContainerFields[key] {
				continue
			}
			return forbiddenFieldError(containerPath.Child(key), firstClassHint(key))
		}
	}

	return nil
}

// validateAllowedFields extracts the nested map at fieldName from parent and rejects any of its
// keys not permitted by allowed, using hint (if non-nil) to generate a field-specific hint for
// the error message. If fieldName is genuinely absent from parent, it returns (nil, nil): not an
// error, since a patch may legitimately omit any level of this nesting, and the nil result
// propagates safely through subsequent unstructured.Nested* / validateAllowedFields calls.
//
// A present-but-malformed value at fieldName (e.g. an explicit JSON null, or a non-object value)
// is rejected rather than treated as absent: unstructured.NestedMap reports both cases as
// !exists, but only distinguishes them via a non-nil err, so err must be checked first. Left
// unchecked, a strategic merge patch treats an explicit null as "delete this field", so e.g.
// advancedOverrides: {"spec": null} would silently bypass every allowlist check below it and
// wipe out the entire Deployment spec.
func validateAllowedFields(parent map[string]any, fieldName string, fldPath *field.Path, allowed func(key string) bool, hint func(key string) string) (map[string]any, error) {
	m, exists, err := unstructured.NestedMap(parent, fieldName)
	if err != nil {
		return nil, malformedFieldError(fldPath, err)
	}
	if !exists {
		return nil, nil
	}

	for key := range m {
		if allowed(key) {
			continue
		}
		h := ""
		if hint != nil {
			h = hint(key)
		}
		return nil, forbiddenFieldError(fldPath.Child(key), h)
	}

	return m, nil
}

// malformedFieldError builds the UserConfigurationError for a present-but-malformed
// advancedOverrides value at fldPath (e.g. an explicit JSON null, or a value whose type doesn't
// match what a strategic merge patch expects at that path), wrapping the underlying accessor
// error from unstructured.Nested*.
func malformedFieldError(fldPath *field.Path, cause error) error {
	return common.NewUserConfigurationError(
		field.Invalid(fldPath, nil, cause.Error()),
		"advancedOverrides field %q is malformed", fldPath.String(),
	)
}

// forbiddenFieldError builds the UserConfigurationError for a disallowed advancedOverrides
// field at fldPath, appending hint (if non-empty) to point the user at the correct alternative.
func forbiddenFieldError(fldPath *field.Path, hint string) error {
	detail := "field is not allowed in advancedOverrides"
	if hint != "" {
		detail += "; " + hint
	}
	return common.NewUserConfigurationError(
		field.Forbidden(fldPath, detail),
		"advancedOverrides contains a disallowed field",
	)
}

// firstClassHint returns a user-friendly hint for advancedOverrides fields
// that are blocked specifically because overriding them would bypass validation (CEL rules,
// Minimum/Maximum bounds, immutability) enforced on their first-class CRD equivalent.
func firstClassHint(key string) string {
	switch key {
	case "replicas":
		return "use the first-class deploymentConfigs.replicas field instead"
	case "revisionHistoryLimit":
		return "use the first-class deploymentConfigs.revisionHistoryLimit field instead"
	case "selector":
		return "spec.selector is operator-managed and cannot be overridden"
	case "env":
		return "use the first-class spec.controllerConfig.componentConfigs.overrideEnv field instead"
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
