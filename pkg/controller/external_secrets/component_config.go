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
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	operatorv1alpha1 "github.com/openshift/external-secrets-operator/api/v1alpha1"
)

var (
	// disallowedAnnotationMatcher restricts annotations with reserved prefixes from being
	// applied to operand resources. This provides server-side enforcement in addition to
	// the CEL validation in the CRD.
	disallowedAnnotationMatcher = regexp.MustCompile(`^kubernetes\.io/|^app\.kubernetes\.io/|^openshift\.io/|^k8s\.io/`)

	// reservedEnvVarPrefixes lists environment variable prefixes that cannot be overridden
	// by user-specified overrideEnv entries. This provides server-side enforcement in addition
	// to the CEL validation in the CRD.
	reservedEnvVarPrefixes = []string{"HOSTNAME", "KUBERNETES_", "EXTERNAL_SECRETS_"}

	// deploymentAssetToComponentName maps deployment asset names to their corresponding
	// ComponentName enum values.
	deploymentAssetToComponentName = map[string]operatorv1alpha1.ComponentName{
		controllerDeploymentAssetName:     operatorv1alpha1.CoreController,
		webhookDeploymentAssetName:        operatorv1alpha1.Webhook,
		certControllerDeploymentAssetName: operatorv1alpha1.CertController,
		bitwardenDeploymentAssetName:      operatorv1alpha1.BitwardenSDKServer,
	}
)

// applyAnnotations applies user-specified global annotations from ControllerConfig to the
// Deployment metadata and Pod template metadata. Annotations with reserved prefixes are
// skipped and logged. User-specified annotations take precedence over defaults in case of conflicts.
func (r *Reconciler) applyAnnotations(deployment *appsv1.Deployment, esc *operatorv1alpha1.ExternalSecretsConfig) {
	if len(esc.Spec.ControllerConfig.Annotations) == 0 {
		return
	}

	// Ensure annotation maps exist
	if deployment.Annotations == nil {
		deployment.Annotations = make(map[string]string)
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
		deployment.Annotations[annotation.Key] = annotation.Value
		podAnnotations[annotation.Key] = annotation.Value
	}

	deployment.Spec.Template.SetAnnotations(podAnnotations)
}

// getComponentConfig returns the ComponentConfig for the specified component name, if one exists
// in the ExternalSecretsConfig spec. Returns nil if no configuration is found for the component.
func getComponentConfig(esc *operatorv1alpha1.ExternalSecretsConfig, componentName operatorv1alpha1.ComponentName) *operatorv1alpha1.ComponentConfig {
	for i := range esc.Spec.ControllerConfig.ComponentConfigs {
		if esc.Spec.ControllerConfig.ComponentConfigs[i].ComponentName == componentName {
			return &esc.Spec.ControllerConfig.ComponentConfigs[i]
		}
	}
	return nil
}

// getComponentNameForAsset returns the ComponentName corresponding to the given deployment asset name.
func getComponentNameForAsset(assetName string) (operatorv1alpha1.ComponentName, bool) {
	name, ok := deploymentAssetToComponentName[assetName]
	return name, ok
}

// applyComponentConfig applies component-specific configuration overrides from the ComponentConfig
// to the deployment. This includes revisionHistoryLimit and overrideEnv.
func (r *Reconciler) applyComponentConfig(deployment *appsv1.Deployment, esc *operatorv1alpha1.ExternalSecretsConfig, assetName string) error {
	componentName, ok := getComponentNameForAsset(assetName)
	if !ok {
		return nil
	}

	cc := getComponentConfig(esc, componentName)
	if cc == nil {
		return nil
	}

	// Apply revisionHistoryLimit
	if cc.DeploymentConfigs.RevisionHistoryLimit != nil {
		deployment.Spec.RevisionHistoryLimit = cc.DeploymentConfigs.RevisionHistoryLimit
		r.log.V(1).Info("applied revisionHistoryLimit",
			"component", string(componentName),
			"revisionHistoryLimit", *cc.DeploymentConfigs.RevisionHistoryLimit)
	}

	// Apply overrideEnv
	if len(cc.OverrideEnv) > 0 {
		if err := applyOverrideEnv(deployment, cc.OverrideEnv); err != nil {
			return fmt.Errorf("failed to apply overrideEnv for component %s: %w", string(componentName), err)
		}
		r.log.V(1).Info("applied overrideEnv",
			"component", string(componentName),
			"envVarCount", len(cc.OverrideEnv))
	}

	return nil
}

// applyOverrideEnv merges user-specified environment variables into all containers in the deployment.
// User-specified variables take precedence in case of conflicts with existing variables.
// Reserved environment variable prefixes are skipped.
func applyOverrideEnv(deployment *appsv1.Deployment, overrideEnv []corev1.EnvVar) error {
	for i := range deployment.Spec.Template.Spec.Containers {
		container := &deployment.Spec.Template.Spec.Containers[i]
		mergeEnvVars(container, overrideEnv)
	}
	return nil
}

// mergeEnvVars merges override environment variables into a container's existing env vars.
// If an env var with the same name already exists, it is replaced. Otherwise, it is appended.
// Env vars with reserved prefixes are silently skipped.
func mergeEnvVars(container *corev1.Container, overrideEnv []corev1.EnvVar) {
	if container.Env == nil {
		container.Env = make([]corev1.EnvVar, 0, len(overrideEnv))
	}

	for _, override := range overrideEnv {
		if isReservedEnvVar(override.Name) {
			continue
		}

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
}

// isReservedEnvVar checks whether an environment variable name has a reserved prefix.
func isReservedEnvVar(name string) bool {
	for _, prefix := range reservedEnvVarPrefixes {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}
