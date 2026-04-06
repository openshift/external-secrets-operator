package external_secrets

import (
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	operatorv1alpha1 "github.com/openshift/external-secrets-operator/api/v1alpha1"
)

// componentNameToDeploymentAsset maps a ComponentName to its corresponding deployment asset name.
var componentNameToDeploymentAsset = map[operatorv1alpha1.ComponentName]string{
	operatorv1alpha1.CoreController:     controllerDeploymentAssetName,
	operatorv1alpha1.Webhook:            webhookDeploymentAssetName,
	operatorv1alpha1.CertController:     certControllerDeploymentAssetName,
	operatorv1alpha1.BitwardenSDKServer: bitwardenDeploymentAssetName,
}

// componentNameToContainerName maps a ComponentName to the primary container name within its deployment.
var componentNameToContainerName = map[operatorv1alpha1.ComponentName]string{
	operatorv1alpha1.CoreController:     "external-secrets",
	operatorv1alpha1.Webhook:            "webhook",
	operatorv1alpha1.CertController:     "cert-controller",
	operatorv1alpha1.BitwardenSDKServer: "bitwarden-sdk-server",
}

// applyAnnotations merges user-specified annotations from ExternalSecretsConfig onto
// both the Deployment ObjectMeta and the Pod template ObjectMeta. Annotations with
// reserved prefixes are skipped (they are validated at the CRD level).
func applyAnnotations(deployment *appsv1.Deployment, annotations []operatorv1alpha1.Annotation) {
	if len(annotations) == 0 {
		return
	}

	// Apply to Deployment ObjectMeta
	deployAnnotations := deployment.GetAnnotations()
	if deployAnnotations == nil {
		deployAnnotations = make(map[string]string, len(annotations))
	}
	for _, a := range annotations {
		deployAnnotations[a.Key] = a.Value
	}
	deployment.SetAnnotations(deployAnnotations)

	// Apply to Pod template ObjectMeta
	podAnnotations := deployment.Spec.Template.GetAnnotations()
	if podAnnotations == nil {
		podAnnotations = make(map[string]string, len(annotations))
	}
	for _, a := range annotations {
		podAnnotations[a.Key] = a.Value
	}
	deployment.Spec.Template.SetAnnotations(podAnnotations)
}

// applyComponentConfig applies component-specific configuration overrides to a deployment.
// It looks up the matching ComponentConfig entry for the given component name and applies:
// - revisionHistoryLimit from deploymentConfigs
// - overrideEnv from overrideEnv
func applyComponentConfig(deployment *appsv1.Deployment, componentConfigs []operatorv1alpha1.ComponentConfig, componentName operatorv1alpha1.ComponentName) {
	if len(componentConfigs) == 0 {
		return
	}

	// Find the matching ComponentConfig for this component
	var config *operatorv1alpha1.ComponentConfig
	for i := range componentConfigs {
		if componentConfigs[i].ComponentName == componentName {
			config = &componentConfigs[i]
			break
		}
	}
	if config == nil {
		return
	}

	// Apply revisionHistoryLimit if specified
	if config.DeploymentConfigs.RevisionHistoryLimit != nil {
		deployment.Spec.RevisionHistoryLimit = config.DeploymentConfigs.RevisionHistoryLimit
	}

	// Apply override environment variables if specified
	if len(config.OverrideEnv) > 0 {
		containerName, ok := componentNameToContainerName[componentName]
		if !ok {
			return
		}
		applyOverrideEnv(deployment, containerName, config.OverrideEnv)
	}
}

// applyOverrideEnv merges override environment variables into the specified container.
// User-specified variables take precedence over existing ones. Variables with reserved
// prefixes are rejected at the CRD validation level.
func applyOverrideEnv(deployment *appsv1.Deployment, containerName string, overrideEnv []corev1.EnvVar) {
	for i := range deployment.Spec.Template.Spec.Containers {
		if deployment.Spec.Template.Spec.Containers[i].Name != containerName {
			continue
		}

		container := &deployment.Spec.Template.Spec.Containers[i]
		if container.Env == nil {
			container.Env = make([]corev1.EnvVar, 0, len(overrideEnv))
		}

		for _, override := range overrideEnv {
			found := false
			for j := range container.Env {
				if container.Env[j].Name == override.Name {
					container.Env[j] = override
					found = true
					break
				}
			}
			if !found {
				container.Env = append(container.Env, override)
			}
		}
		break
	}
}

// getComponentNameForAsset returns the ComponentName associated with a given deployment asset name.
func getComponentNameForAsset(assetName string) (operatorv1alpha1.ComponentName, error) {
	for componentName, asset := range componentNameToDeploymentAsset {
		if asset == assetName {
			return componentName, nil
		}
	}
	return "", fmt.Errorf("no component mapping found for asset: %s", assetName)
}
