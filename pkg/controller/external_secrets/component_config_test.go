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
	"reflect"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	operatorv1alpha1 "github.com/openshift/external-secrets-operator/api/v1alpha1"
	"github.com/openshift/external-secrets-operator/pkg/controller/commontest"
)

func TestApplyGlobalAnnotations(t *testing.T) {
	tests := []struct {
		name                string
		annotations         []operatorv1alpha1.Annotation
		existingAnnotations map[string]string
		expectedAnnotations map[string]string
	}{
		{
			name:                "no annotations configured",
			annotations:         nil,
			existingAnnotations: nil,
			expectedAnnotations: nil,
		},
		{
			name: "single annotation applied",
			annotations: []operatorv1alpha1.Annotation{
				{KVPair: operatorv1alpha1.KVPair{Key: "example.com/custom", Value: "value1"}},
			},
			existingAnnotations: nil,
			expectedAnnotations: map[string]string{
				"example.com/custom": "value1",
			},
		},
		{
			name: "multiple annotations applied",
			annotations: []operatorv1alpha1.Annotation{
				{KVPair: operatorv1alpha1.KVPair{Key: "example.com/ann-one", Value: "val1"}},
				{KVPair: operatorv1alpha1.KVPair{Key: "example.com/ann-two", Value: "val2"}},
			},
			existingAnnotations: nil,
			expectedAnnotations: map[string]string{
				"example.com/ann-one": "val1",
				"example.com/ann-two": "val2",
			},
		},
		{
			name: "user annotation takes precedence over existing",
			annotations: []operatorv1alpha1.Annotation{
				{KVPair: operatorv1alpha1.KVPair{Key: "example.com/existing", Value: "new-value"}},
			},
			existingAnnotations: map[string]string{
				"example.com/existing": "old-value",
			},
			expectedAnnotations: map[string]string{
				"example.com/existing": "new-value",
			},
		},
		{
			name: "reserved kubernetes.io/ prefix is skipped",
			annotations: []operatorv1alpha1.Annotation{
				{KVPair: operatorv1alpha1.KVPair{Key: "kubernetes.io/reserved", Value: "should-skip"}},
				{KVPair: operatorv1alpha1.KVPair{Key: "example.com/valid", Value: "should-apply"}},
			},
			existingAnnotations: nil,
			expectedAnnotations: map[string]string{
				"example.com/valid": "should-apply",
			},
		},
		{
			name: "reserved app.kubernetes.io/ prefix is skipped",
			annotations: []operatorv1alpha1.Annotation{
				{KVPair: operatorv1alpha1.KVPair{Key: "app.kubernetes.io/reserved", Value: "should-skip"}},
			},
			existingAnnotations: nil,
			expectedAnnotations: nil,
		},
		{
			name: "reserved openshift.io/ prefix is skipped",
			annotations: []operatorv1alpha1.Annotation{
				{KVPair: operatorv1alpha1.KVPair{Key: "openshift.io/reserved", Value: "should-skip"}},
			},
			existingAnnotations: nil,
			expectedAnnotations: nil,
		},
		{
			name: "reserved k8s.io/ prefix is skipped",
			annotations: []operatorv1alpha1.Annotation{
				{KVPair: operatorv1alpha1.KVPair{Key: "k8s.io/reserved", Value: "should-skip"}},
			},
			existingAnnotations: nil,
			expectedAnnotations: nil,
		},
		{
			name: "annotations merge with existing without overwriting unrelated",
			annotations: []operatorv1alpha1.Annotation{
				{KVPair: operatorv1alpha1.KVPair{Key: "example.com/new", Value: "new-val"}},
			},
			existingAnnotations: map[string]string{
				"example.com/existing": "existing-val",
			},
			expectedAnnotations: map[string]string{
				"example.com/existing": "existing-val",
				"example.com/new":      "new-val",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := testReconciler(t)
			esc := commontest.TestExternalSecretsConfig()
			esc.Spec.ControllerConfig.Annotations = tt.annotations

			deployment := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test-deployment",
					Annotations: tt.existingAnnotations,
				},
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Annotations: copyMap(tt.existingAnnotations),
						},
					},
				},
			}

			r.applyGlobalAnnotations(deployment, esc)

			// Check deployment annotations
			if !reflect.DeepEqual(deployment.GetAnnotations(), tt.expectedAnnotations) {
				t.Errorf("deployment annotations = %v, want %v", deployment.GetAnnotations(), tt.expectedAnnotations)
			}

			// Check pod template annotations
			if !reflect.DeepEqual(deployment.Spec.Template.GetAnnotations(), tt.expectedAnnotations) {
				t.Errorf("pod template annotations = %v, want %v", deployment.Spec.Template.GetAnnotations(), tt.expectedAnnotations)
			}
		})
	}
}

func TestApplyComponentDeploymentConfig(t *testing.T) {
	tests := []struct {
		name                    string
		componentConfig         *operatorv1alpha1.ComponentConfig
		expectedRevHistoryLimit *int32
	}{
		{
			name:                    "nil component config",
			componentConfig:         nil,
			expectedRevHistoryLimit: nil,
		},
		{
			name: "nil deployment configs",
			componentConfig: &operatorv1alpha1.ComponentConfig{
				ComponentName:     operatorv1alpha1.CoreController,
				DeploymentConfigs: nil,
			},
			expectedRevHistoryLimit: nil,
		},
		{
			name: "revisionHistoryLimit set to 5",
			componentConfig: &operatorv1alpha1.ComponentConfig{
				ComponentName: operatorv1alpha1.CoreController,
				DeploymentConfigs: &operatorv1alpha1.DeploymentConfig{
					RevisionHistoryLimit: ptr.To[int32](5),
				},
			},
			expectedRevHistoryLimit: ptr.To[int32](5),
		},
		{
			name: "revisionHistoryLimit set to 10",
			componentConfig: &operatorv1alpha1.ComponentConfig{
				ComponentName: operatorv1alpha1.Webhook,
				DeploymentConfigs: &operatorv1alpha1.DeploymentConfig{
					RevisionHistoryLimit: ptr.To[int32](10),
				},
			},
			expectedRevHistoryLimit: ptr.To[int32](10),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := testReconciler(t)
			deployment := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{Name: "test"},
			}

			r.applyComponentDeploymentConfig(deployment, tt.componentConfig)

			if !reflect.DeepEqual(deployment.Spec.RevisionHistoryLimit, tt.expectedRevHistoryLimit) {
				t.Errorf("revisionHistoryLimit = %v, want %v", deployment.Spec.RevisionHistoryLimit, tt.expectedRevHistoryLimit)
			}
		})
	}
}

func TestApplyComponentOverrideEnv(t *testing.T) {
	tests := []struct {
		name            string
		componentConfig *operatorv1alpha1.ComponentConfig
		containerName   string
		initialEnv      []corev1.EnvVar
		expectedEnv     []corev1.EnvVar
	}{
		{
			name:            "nil component config",
			componentConfig: nil,
			containerName:   "external-secrets",
			initialEnv:      nil,
			expectedEnv:     nil,
		},
		{
			name: "empty override env",
			componentConfig: &operatorv1alpha1.ComponentConfig{
				ComponentName: operatorv1alpha1.CoreController,
				OverrideEnv:   []corev1.EnvVar{},
			},
			containerName: "external-secrets",
			initialEnv:    nil,
			expectedEnv:   nil,
		},
		{
			name: "add new env var",
			componentConfig: &operatorv1alpha1.ComponentConfig{
				ComponentName: operatorv1alpha1.CoreController,
				OverrideEnv: []corev1.EnvVar{
					{Name: "GOMAXPROCS", Value: "4"},
				},
			},
			containerName: "external-secrets",
			initialEnv:    nil,
			expectedEnv: []corev1.EnvVar{
				{Name: "GOMAXPROCS", Value: "4"},
			},
		},
		{
			name: "override existing env var",
			componentConfig: &operatorv1alpha1.ComponentConfig{
				ComponentName: operatorv1alpha1.CoreController,
				OverrideEnv: []corev1.EnvVar{
					{Name: "LOG_FORMAT", Value: "json"},
				},
			},
			containerName: "external-secrets",
			initialEnv: []corev1.EnvVar{
				{Name: "LOG_FORMAT", Value: "text"},
			},
			expectedEnv: []corev1.EnvVar{
				{Name: "LOG_FORMAT", Value: "json"},
			},
		},
		{
			name: "skip reserved HOSTNAME env var",
			componentConfig: &operatorv1alpha1.ComponentConfig{
				ComponentName: operatorv1alpha1.CoreController,
				OverrideEnv: []corev1.EnvVar{
					{Name: "HOSTNAME", Value: "custom"},
					{Name: "GOMAXPROCS", Value: "4"},
				},
			},
			containerName: "external-secrets",
			initialEnv:    nil,
			expectedEnv: []corev1.EnvVar{
				{Name: "GOMAXPROCS", Value: "4"},
			},
		},
		{
			name: "skip reserved KUBERNETES_ prefix env var",
			componentConfig: &operatorv1alpha1.ComponentConfig{
				ComponentName: operatorv1alpha1.CoreController,
				OverrideEnv: []corev1.EnvVar{
					{Name: "KUBERNETES_SERVICE_HOST", Value: "custom"},
				},
			},
			containerName: "external-secrets",
			initialEnv:    []corev1.EnvVar{},
			expectedEnv:   []corev1.EnvVar{},
		},
		{
			name: "skip reserved EXTERNAL_SECRETS_ prefix env var",
			componentConfig: &operatorv1alpha1.ComponentConfig{
				ComponentName: operatorv1alpha1.CoreController,
				OverrideEnv: []corev1.EnvVar{
					{Name: "EXTERNAL_SECRETS_CONFIG", Value: "custom"},
				},
			},
			containerName: "external-secrets",
			initialEnv:    []corev1.EnvVar{},
			expectedEnv:   []corev1.EnvVar{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := testReconciler(t)
			deployment := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{Name: "test"},
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name: tt.containerName,
									Env:  tt.initialEnv,
								},
							},
						},
					},
				},
			}

			r.applyComponentOverrideEnv(deployment, tt.componentConfig, tt.containerName)

			actualEnv := deployment.Spec.Template.Spec.Containers[0].Env
			if !reflect.DeepEqual(actualEnv, tt.expectedEnv) {
				t.Errorf("container env = %v, want %v", actualEnv, tt.expectedEnv)
			}
		})
	}
}

func TestApplyComponentOverrideEnvWrongContainer(t *testing.T) {
	r := testReconciler(t)
	componentConfig := &operatorv1alpha1.ComponentConfig{
		ComponentName: operatorv1alpha1.CoreController,
		OverrideEnv: []corev1.EnvVar{
			{Name: "GOMAXPROCS", Value: "4"},
		},
	}
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "test"},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: "some-other-container",
						},
					},
				},
			},
		},
	}

	// Target "external-secrets" but the deployment has "some-other-container"
	r.applyComponentOverrideEnv(deployment, componentConfig, "external-secrets")

	actualEnv := deployment.Spec.Template.Spec.Containers[0].Env
	if len(actualEnv) != 0 {
		t.Errorf("expected no env vars on non-matching container, got %v", actualEnv)
	}
}

func TestGetComponentConfig(t *testing.T) {
	tests := []struct {
		name          string
		configs       []operatorv1alpha1.ComponentConfig
		componentName operatorv1alpha1.ComponentName
		expectNil     bool
	}{
		{
			name:          "empty configs returns nil",
			configs:       nil,
			componentName: operatorv1alpha1.CoreController,
			expectNil:     true,
		},
		{
			name: "finds matching config",
			configs: []operatorv1alpha1.ComponentConfig{
				{ComponentName: operatorv1alpha1.CoreController},
				{ComponentName: operatorv1alpha1.Webhook},
			},
			componentName: operatorv1alpha1.Webhook,
			expectNil:     false,
		},
		{
			name: "returns nil for non-matching component",
			configs: []operatorv1alpha1.ComponentConfig{
				{ComponentName: operatorv1alpha1.CoreController},
			},
			componentName: operatorv1alpha1.BitwardenSDKServer,
			expectNil:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			esc := commontest.TestExternalSecretsConfig()
			esc.Spec.ControllerConfig.ComponentConfigs = tt.configs

			result := getComponentConfig(esc, tt.componentName)
			if tt.expectNil && result != nil {
				t.Error("expected nil, got non-nil")
			}
			if !tt.expectNil && result == nil {
				t.Error("expected non-nil, got nil")
			}
			if !tt.expectNil && result != nil && result.ComponentName != tt.componentName {
				t.Errorf("componentName = %s, want %s", result.ComponentName, tt.componentName)
			}
		})
	}
}

func TestIsReservedEnvVar(t *testing.T) {
	tests := []struct {
		name     string
		envName  string
		expected bool
	}{
		{"HOSTNAME is reserved", "HOSTNAME", true},
		{"KUBERNETES_ prefix is reserved", "KUBERNETES_SERVICE_HOST", true},
		{"EXTERNAL_SECRETS_ prefix is reserved", "EXTERNAL_SECRETS_CONFIG", true},
		{"GOMAXPROCS is not reserved", "GOMAXPROCS", false},
		{"LOG_FORMAT is not reserved", "LOG_FORMAT", false},
		{"empty string is not reserved", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isReservedEnvVar(tt.envName)
			if result != tt.expected {
				t.Errorf("isReservedEnvVar(%q) = %v, want %v", tt.envName, result, tt.expected)
			}
		})
	}
}

// copyMap creates a shallow copy of a string map. Returns nil if input is nil.
func copyMap(m map[string]string) map[string]string {
	if m == nil {
		return nil
	}
	result := make(map[string]string, len(m))
	for k, v := range m {
		result[k] = v
	}
	return result
}
