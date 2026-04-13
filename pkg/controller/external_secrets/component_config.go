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
	// disallowedAnnotationMatcher restricts annotations that cannot be overridden by users.
	// These annotation prefixes are reserved for Kubernetes, OpenShift, and related tooling.
	disallowedAnnotationMatcher = regexp.MustCompile(`^kubernetes\.io/|^app\.kubernetes\.io/|^openshift\.io/|^k8s\.io/`)

	// reservedEnvVarPrefixes lists environment variable name prefixes that cannot be overridden.
	reservedEnvVarPrefixes = []string{"HOSTNAME", "KUBERNETES_", "EXTERNAL_SECRETS_"}

	// assetNameToComponentName maps deployment asset names to their corresponding ComponentName.
	assetNameToComponentName = map[string]operatorv1alpha1.ComponentName{
		controllerDeploymentAssetName:     operatorv1alpha1.CoreController,
		webhookDeploymentAssetName:        operatorv1alpha1.Webhook,
		certControllerDeploymentAssetName: operatorv1alpha1.CertController,
		bitwardenDeploymentAssetName:      operatorv1alpha1.BitwardenSDKServer,
	}

	// componentNameToContainerName maps ComponentName to the container name in the deployment spec.
	componentNameToContainerName = map[operatorv1alpha1.ComponentName]string{
		operatorv1alpha1.CoreController:     "external-secrets",
		operatorv1alpha1.Webhook:            "webhook",
		operatorv1alpha1.CertController:     "cert-controller",
		operatorv1alpha1.BitwardenSDKServer: "bitwarden-sdk-server",
	}
)

// applyGlobalAnnotations applies user-specified global annotations from ControllerConfig
// to both the Deployment metadata and its Pod template metadata.
// User-specified annotations take precedence over defaults in case of conflicts.
// Annotations matching disallowed prefixes are silently skipped.
func (r *Reconciler) applyGlobalAnnotations(deployment *appsv1.Deployment, esc *operatorv1alpha1.ExternalSecretsConfig) {
	if len(esc.Spec.ControllerConfig.Annotations) == 0 {
		return
	}

	// Filter allowed annotations first
	allowedAnnotations := make(map[string]string)
	for _, annotation := range esc.Spec.ControllerConfig.Annotations {
		if disallowedAnnotationMatcher.MatchString(annotation.Key) {
			r.log.V(1).Info("skip adding disallowed annotation configured in externalsecretsconfigs.operator.openshift.io",
				"annotation", annotation.Key, "value", annotation.Value)
			continue
		}
		allowedAnnotations[annotation.Key] = annotation.Value
	}

	if len(allowedAnnotations) == 0 {
		return
	}

	// Apply annotations to Deployment metadata
	deployAnnotations := deployment.GetAnnotations()
	if deployAnnotations == nil {
		deployAnnotations = make(map[string]string, len(allowedAnnotations))
	}
	for k, v := range allowedAnnotations {
		deployAnnotations[k] = v
	}
	deployment.SetAnnotations(deployAnnotations)

	// Apply annotations to Pod template metadata
	podAnnotations := deployment.Spec.Template.GetAnnotations()
	if podAnnotations == nil {
		podAnnotations = make(map[string]string, len(allowedAnnotations))
	}
	for k, v := range allowedAnnotations {
		podAnnotations[k] = v
	}
	deployment.Spec.Template.SetAnnotations(podAnnotations)
}

// getComponentConfig returns the ComponentConfig for the given component name, or nil if not configured.
func getComponentConfig(esc *operatorv1alpha1.ExternalSecretsConfig, componentName operatorv1alpha1.ComponentName) *operatorv1alpha1.ComponentConfig {
	for i := range esc.Spec.ControllerConfig.ComponentConfigs {
		if esc.Spec.ControllerConfig.ComponentConfigs[i].ComponentName == componentName {
			return &esc.Spec.ControllerConfig.ComponentConfigs[i]
		}
	}
	return nil
}

// applyComponentDeploymentConfig applies component-specific deployment configuration overrides
// such as revisionHistoryLimit to the deployment spec.
func (r *Reconciler) applyComponentDeploymentConfig(deployment *appsv1.Deployment, componentConfig *operatorv1alpha1.ComponentConfig) {
	if componentConfig == nil || componentConfig.DeploymentConfigs == nil {
		return
	}

	if componentConfig.DeploymentConfigs.RevisionHistoryLimit != nil {
		deployment.Spec.RevisionHistoryLimit = componentConfig.DeploymentConfigs.RevisionHistoryLimit
		r.log.V(1).Info("applied revisionHistoryLimit override",
			"component", componentConfig.ComponentName,
			"revisionHistoryLimit", *componentConfig.DeploymentConfigs.RevisionHistoryLimit)
	}
}

// applyComponentOverrideEnv applies component-specific override environment variables
// to the component's primary container. User-specified variables take precedence
// over defaults in case of conflicts. Variables with reserved prefixes are skipped.
func (r *Reconciler) applyComponentOverrideEnv(deployment *appsv1.Deployment, componentConfig *operatorv1alpha1.ComponentConfig, containerName string) {
	if componentConfig == nil || len(componentConfig.OverrideEnv) == 0 {
		return
	}

	for i := range deployment.Spec.Template.Spec.Containers {
		if deployment.Spec.Template.Spec.Containers[i].Name != containerName {
			continue
		}

		container := &deployment.Spec.Template.Spec.Containers[i]
		if container.Env == nil {
			container.Env = make([]corev1.EnvVar, 0, len(componentConfig.OverrideEnv))
		}

		for _, envVar := range componentConfig.OverrideEnv {
			if isReservedEnvVar(envVar.Name) {
				r.log.V(1).Info("skip overriding reserved environment variable",
					"component", componentConfig.ComponentName,
					"envVar", envVar.Name)
				continue
			}

			// Check if the env var already exists and update it
			found := false
			for j := range container.Env {
				if container.Env[j].Name == envVar.Name {
					container.Env[j] = envVar
					found = true
					r.log.V(1).Info("overriding existing environment variable",
						"component", componentConfig.ComponentName,
						"envVar", envVar.Name)
					break
				}
			}

			// Add new env var if it doesn't exist
			if !found {
				container.Env = append(container.Env, envVar)
				r.log.V(1).Info("adding custom environment variable",
					"component", componentConfig.ComponentName,
					"envVar", envVar.Name)
			}
		}

		break
	}
}

// isReservedEnvVar checks if an environment variable name has a reserved prefix.
func isReservedEnvVar(name string) bool {
	for _, prefix := range reservedEnvVarPrefixes {
		if len(name) >= len(prefix) && name[:len(prefix)] == prefix {
			return true
		}
	}
	return false
}

// applyComponentConfig applies all component-specific configuration to a deployment,
// including deployment configs (revisionHistoryLimit) and override env vars.
func (r *Reconciler) applyComponentConfig(deployment *appsv1.Deployment, esc *operatorv1alpha1.ExternalSecretsConfig, assetName string) error {
	componentName, ok := assetNameToComponentName[assetName]
	if !ok {
		return fmt.Errorf("unknown deployment asset name: %s", assetName)
	}

	// Apply global annotations
	r.applyGlobalAnnotations(deployment, esc)

	// Apply component-specific configuration if present
	componentConfig := getComponentConfig(esc, componentName)
	if componentConfig != nil {
		// Apply deployment config overrides (e.g., revisionHistoryLimit)
		r.applyComponentDeploymentConfig(deployment, componentConfig)

		// Apply override environment variables
		containerName, ok := componentNameToContainerName[componentName]
		if !ok {
			return fmt.Errorf("unknown container name for component: %s", componentName)
		}
		r.applyComponentOverrideEnv(deployment, componentConfig, containerName)
	}

	return nil
}
