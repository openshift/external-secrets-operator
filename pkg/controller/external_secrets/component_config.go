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
	// reservedEnvVarPrefixes defines environment variable name prefixes that cannot be overridden
	// by users through the overrideEnv configuration.
	reservedEnvVarPrefixes = []string{"HOSTNAME", "KUBERNETES_", "EXTERNAL_SECRETS_"}

	// disallowedAnnotationMatcher is for restricting the annotations defined to apply on all resources
	// created for `external-secrets` operand deployment.
	disallowedAnnotationMatcher = regexp.MustCompile(`^kubernetes\.io/|^app\.kubernetes\.io/|^openshift\.io/|^k8s\.io/`)
)

// deploymentAssetToComponentName maps asset names to their corresponding ComponentName values.
var deploymentAssetToComponentName = map[string]operatorv1alpha1.ComponentName{
	controllerDeploymentAssetName:     operatorv1alpha1.CoreController,
	webhookDeploymentAssetName:        operatorv1alpha1.Webhook,
	certControllerDeploymentAssetName: operatorv1alpha1.CertController,
	bitwardenDeploymentAssetName:      operatorv1alpha1.BitwardenSDKServer,
}

// applyAnnotations applies custom annotations from controllerConfig.annotations to the
// Deployment metadata and Pod template metadata. Annotations with reserved prefixes are
// skipped with a log warning.
func (r *Reconciler) applyAnnotations(deployment *appsv1.Deployment, esc *operatorv1alpha1.ExternalSecretsConfig) {
	if len(esc.Spec.ControllerConfig.Annotations) == 0 {
		return
	}

	deployAnnotations := deployment.GetAnnotations()
	if deployAnnotations == nil {
		deployAnnotations = make(map[string]string)
	}

	podAnnotations := deployment.Spec.Template.GetAnnotations()
	if podAnnotations == nil {
		podAnnotations = make(map[string]string)
	}

	for _, annotation := range esc.Spec.ControllerConfig.Annotations {
		if disallowedAnnotationMatcher.MatchString(annotation.Key) {
			r.log.V(1).Info("skip adding annotation with reserved prefix", "key", annotation.Key, "value", annotation.Value)
			continue
		}
		deployAnnotations[annotation.Key] = annotation.Value
		podAnnotations[annotation.Key] = annotation.Value
	}

	deployment.SetAnnotations(deployAnnotations)
	deployment.Spec.Template.SetAnnotations(podAnnotations)
}

// applyComponentConfig applies per-component configuration overrides from
// controllerConfig.componentConfigs to the given deployment. It looks up the
// component config matching the deployment's asset name and applies:
// - revisionHistoryLimit from deploymentConfig
// - overrideEnv environment variables merged into the container spec
func (r *Reconciler) applyComponentConfig(deployment *appsv1.Deployment, esc *operatorv1alpha1.ExternalSecretsConfig, assetName string) error {
	componentName, ok := deploymentAssetToComponentName[assetName]
	if !ok {
		return nil
	}

	config := getComponentConfigForComponent(esc, componentName)
	if config == nil {
		return nil
	}

	// Apply revisionHistoryLimit
	if config.DeploymentConfig.RevisionHistoryLimit != nil {
		deployment.Spec.RevisionHistoryLimit = config.DeploymentConfig.RevisionHistoryLimit
	}

	// Apply overrideEnv
	if len(config.OverrideEnv) > 0 {
		if err := r.applyOverrideEnv(deployment, config.OverrideEnv); err != nil {
			return fmt.Errorf("failed to apply override env for component %s: %w", componentName, err)
		}
	}

	return nil
}

// getComponentConfigForComponent returns the ComponentConfig for the given component name,
// or nil if no configuration is set.
func getComponentConfigForComponent(esc *operatorv1alpha1.ExternalSecretsConfig, componentName operatorv1alpha1.ComponentName) *operatorv1alpha1.ComponentConfig {
	for i := range esc.Spec.ControllerConfig.ComponentConfigs {
		if esc.Spec.ControllerConfig.ComponentConfigs[i].ComponentName == componentName {
			return &esc.Spec.ControllerConfig.ComponentConfigs[i]
		}
	}
	return nil
}

// applyOverrideEnv merges user-specified environment variables into all containers
// in the deployment. User-specified variables take precedence over existing defaults.
// Environment variables with reserved prefixes are skipped with a log warning.
func (r *Reconciler) applyOverrideEnv(deployment *appsv1.Deployment, overrideEnv []corev1.EnvVar) error {
	for i := range deployment.Spec.Template.Spec.Containers {
		container := &deployment.Spec.Template.Spec.Containers[i]
		mergeEnvVars(container, overrideEnv, r)
	}
	return nil
}

// mergeEnvVars merges override environment variables into a container's existing env vars.
// If an env var with the same name already exists, it is overridden with the user-provided value.
// Env vars with reserved prefixes are skipped.
func mergeEnvVars(container *corev1.Container, overrideEnv []corev1.EnvVar, r *Reconciler) {
	for _, envVar := range overrideEnv {
		if isReservedEnvVar(envVar.Name) {
			r.log.V(1).Info("skip overriding environment variable with reserved prefix",
				"name", envVar.Name, "container", container.Name)
			continue
		}

		found := false
		for j := range container.Env {
			if container.Env[j].Name == envVar.Name {
				container.Env[j] = envVar
				found = true
				break
			}
		}
		if !found {
			container.Env = append(container.Env, envVar)
		}
	}
}

// isReservedEnvVar checks if the given environment variable name starts with a reserved prefix.
func isReservedEnvVar(name string) bool {
	for _, prefix := range reservedEnvVarPrefixes {
		if len(name) >= len(prefix) && name[:len(prefix)] == prefix {
			return true
		}
	}
	return false
}
