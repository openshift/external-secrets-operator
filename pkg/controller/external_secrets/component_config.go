/*
Copyright 2025.

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

package external_secrets

import (
	"fmt"
	"regexp"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	operatorv1alpha1 "github.com/openshift/external-secrets-operator/api/v1alpha1"
)

var (
	// disallowedAnnotationMatcher restricts annotations with reserved prefixes from being
	// applied to operator-managed resources. Reserved prefixes match platform-managed annotations
	// that should not be overridden by users.
	disallowedAnnotationMatcher = regexp.MustCompile(`^kubernetes\.io/|^app\.kubernetes\.io/|^openshift\.io/|^k8s\.io/`)

	// componentNameToDeploymentAsset maps ComponentName enum values to their corresponding
	// deployment asset file paths used by the controller.
	componentNameToDeploymentAsset = map[operatorv1alpha1.ComponentName]string{
		operatorv1alpha1.CoreController:     controllerDeploymentAssetName,
		operatorv1alpha1.Webhook:            webhookDeploymentAssetName,
		operatorv1alpha1.CertController:     certControllerDeploymentAssetName,
		operatorv1alpha1.BitwardenSDKServer: bitwardenDeploymentAssetName,
	}

	// componentNameToContainerName maps ComponentName enum values to their primary
	// container names within each deployment.
	componentNameToContainerName = map[operatorv1alpha1.ComponentName]string{
		operatorv1alpha1.CoreController:     "external-secrets",
		operatorv1alpha1.Webhook:            "webhook",
		operatorv1alpha1.CertController:     "cert-controller",
		operatorv1alpha1.BitwardenSDKServer: "bitwarden-sdk-server",
	}
)

// applyAnnotationsToDeployment applies custom annotations from controllerConfig.annotations
// to the deployment's metadata and pod template metadata.
// Annotations with reserved prefixes are skipped with a log warning.
func (r *Reconciler) applyAnnotationsToDeployment(deployment *appsv1.Deployment, esc *operatorv1alpha1.ExternalSecretsConfig) {
	if len(esc.Spec.ControllerConfig.Annotations) == 0 {
		return
	}

	// Build the annotation map, skipping reserved prefixes
	customAnnotations := make(map[string]string, len(esc.Spec.ControllerConfig.Annotations))
	for _, annotation := range esc.Spec.ControllerConfig.Annotations {
		if disallowedAnnotationMatcher.MatchString(annotation.Key) {
			r.log.V(1).Info("skip adding annotation with reserved prefix", "key", annotation.Key)
			continue
		}
		customAnnotations[annotation.Key] = annotation.Value
	}

	if len(customAnnotations) == 0 {
		return
	}

	// Apply to deployment metadata
	deploymentAnnotations := deployment.GetAnnotations()
	if deploymentAnnotations == nil {
		deploymentAnnotations = make(map[string]string, len(customAnnotations))
	}
	for k, v := range customAnnotations {
		deploymentAnnotations[k] = v
	}
	deployment.SetAnnotations(deploymentAnnotations)

	// Apply to pod template metadata
	podAnnotations := deployment.Spec.Template.GetAnnotations()
	if podAnnotations == nil {
		podAnnotations = make(map[string]string, len(customAnnotations))
	}
	for k, v := range customAnnotations {
		podAnnotations[k] = v
	}
	deployment.Spec.Template.SetAnnotations(podAnnotations)
}

// applyComponentConfigToDeployment applies component-specific configuration overrides
// (revisionHistoryLimit and overrideEnv) to a deployment based on its asset name.
// It finds the matching ComponentConfig entry by mapping the asset name to a ComponentName.
func (r *Reconciler) applyComponentConfigToDeployment(deployment *appsv1.Deployment, esc *operatorv1alpha1.ExternalSecretsConfig, assetName string) error {
	if len(esc.Spec.ControllerConfig.ComponentConfigs) == 0 {
		return nil
	}

	// Determine which ComponentName this asset corresponds to
	componentName, found := getComponentNameForAsset(assetName)
	if !found {
		return nil
	}

	// Find the matching ComponentConfig entry
	var componentConfig *operatorv1alpha1.ComponentConfig
	for i := range esc.Spec.ControllerConfig.ComponentConfigs {
		if esc.Spec.ControllerConfig.ComponentConfigs[i].ComponentName == componentName {
			componentConfig = &esc.Spec.ControllerConfig.ComponentConfigs[i]
			break
		}
	}
	if componentConfig == nil {
		return nil
	}

	// Apply revisionHistoryLimit
	if componentConfig.DeploymentConfigs.RevisionHistoryLimit != nil {
		deployment.Spec.RevisionHistoryLimit = componentConfig.DeploymentConfigs.RevisionHistoryLimit
		r.log.V(1).Info("applied revisionHistoryLimit",
			"component", componentName,
			"revisionHistoryLimit", *componentConfig.DeploymentConfigs.RevisionHistoryLimit)
	}

	// Apply overrideEnv to the primary container
	if len(componentConfig.OverrideEnv) > 0 {
		containerName, ok := componentNameToContainerName[componentName]
		if !ok {
			return fmt.Errorf("no container name mapping for component %q", componentName)
		}

		if err := applyOverrideEnvToContainer(deployment, containerName, componentConfig.OverrideEnv); err != nil {
			return fmt.Errorf("failed to apply overrideEnv for component %q: %w", componentName, err)
		}
		r.log.V(1).Info("applied overrideEnv",
			"component", componentName,
			"envCount", len(componentConfig.OverrideEnv))
	}

	return nil
}

// getComponentNameForAsset returns the ComponentName that corresponds to the given
// deployment asset name. Returns false if the asset is not recognized.
func getComponentNameForAsset(assetName string) (operatorv1alpha1.ComponentName, bool) {
	for componentName, asset := range componentNameToDeploymentAsset {
		if asset == assetName {
			return componentName, true
		}
	}
	return "", false
}

// applyOverrideEnvToContainer merges user-specified environment variables into the
// primary container of a deployment. User-specified variables take precedence over
// existing variables with the same name.
func applyOverrideEnvToContainer(deployment *appsv1.Deployment, containerName string, overrideEnv []corev1.EnvVar) error {
	for i := range deployment.Spec.Template.Spec.Containers {
		if deployment.Spec.Template.Spec.Containers[i].Name != containerName {
			continue
		}
		container := &deployment.Spec.Template.Spec.Containers[i]

		// Build a map of existing env vars for fast lookup
		existingEnvMap := make(map[string]int, len(container.Env))
		for idx, env := range container.Env {
			existingEnvMap[env.Name] = idx
		}

		// Merge overrideEnv — update existing or append new
		for _, override := range overrideEnv {
			if idx, exists := existingEnvMap[override.Name]; exists {
				container.Env[idx] = override
			} else {
				container.Env = append(container.Env, override)
				existingEnvMap[override.Name] = len(container.Env) - 1
			}
		}

		return nil
	}

	return fmt.Errorf("container %q not found in deployment %s/%s", containerName, deployment.GetNamespace(), deployment.GetName())
}
