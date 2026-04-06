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
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	"github.com/go-logr/logr"

	operatorv1alpha1 "github.com/openshift/external-secrets-operator/api/v1alpha1"
)

func newTestReconciler() *Reconciler {
	return &Reconciler{
		log: logr.Discard(),
	}
}

func newTestDeployment(containerName string) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-deployment",
			Namespace: "external-secrets",
		},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  containerName,
							Image: "test-image:latest",
							Env: []corev1.EnvVar{
								{Name: "EXISTING_VAR", Value: "existing-value"},
							},
						},
					},
				},
			},
		},
	}
}

func newTestESC() *operatorv1alpha1.ExternalSecretsConfig {
	return &operatorv1alpha1.ExternalSecretsConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cluster",
		},
		Spec: operatorv1alpha1.ExternalSecretsConfigSpec{},
	}
}

func TestApplyAnnotationsToDeployment(t *testing.T) {
	tests := []struct {
		name                string
		annotations         []operatorv1alpha1.Annotation
		expectedDeployAnnot map[string]string
		expectedPodAnnot    map[string]string
	}{
		{
			name:                "no annotations",
			annotations:         nil,
			expectedDeployAnnot: nil,
			expectedPodAnnot:    nil,
		},
		{
			name: "single custom annotation",
			annotations: []operatorv1alpha1.Annotation{
				{Key: "example.com/my-key", Value: "my-value"},
			},
			expectedDeployAnnot: map[string]string{"example.com/my-key": "my-value"},
			expectedPodAnnot:    map[string]string{"example.com/my-key": "my-value"},
		},
		{
			name: "multiple annotations",
			annotations: []operatorv1alpha1.Annotation{
				{Key: "example.com/key-1", Value: "value-1"},
				{Key: "example.com/key-2", Value: "value-2"},
			},
			expectedDeployAnnot: map[string]string{
				"example.com/key-1": "value-1",
				"example.com/key-2": "value-2",
			},
			expectedPodAnnot: map[string]string{
				"example.com/key-1": "value-1",
				"example.com/key-2": "value-2",
			},
		},
		{
			name: "reserved annotations are skipped",
			annotations: []operatorv1alpha1.Annotation{
				{Key: "kubernetes.io/reserved", Value: "skip"},
				{Key: "app.kubernetes.io/name", Value: "skip"},
				{Key: "openshift.io/some-key", Value: "skip"},
				{Key: "k8s.io/some-key", Value: "skip"},
				{Key: "example.com/allowed", Value: "keep"},
			},
			expectedDeployAnnot: map[string]string{"example.com/allowed": "keep"},
			expectedPodAnnot:    map[string]string{"example.com/allowed": "keep"},
		},
		{
			name: "annotation with empty value",
			annotations: []operatorv1alpha1.Annotation{
				{Key: "example.com/empty-value", Value: ""},
			},
			expectedDeployAnnot: map[string]string{"example.com/empty-value": ""},
			expectedPodAnnot:    map[string]string{"example.com/empty-value": ""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newTestReconciler()
			deployment := newTestDeployment("external-secrets")
			esc := newTestESC()
			esc.Spec.ControllerConfig.Annotations = tt.annotations

			r.applyAnnotationsToDeployment(deployment, esc)

			// Check deployment annotations
			deployAnnot := deployment.GetAnnotations()
			if tt.expectedDeployAnnot == nil {
				if deployAnnot != nil && len(deployAnnot) > 0 {
					t.Errorf("expected no deployment annotations, got %v", deployAnnot)
				}
			} else {
				for k, v := range tt.expectedDeployAnnot {
					if got, ok := deployAnnot[k]; !ok || got != v {
						t.Errorf("expected deployment annotation %q=%q, got %q", k, v, got)
					}
				}
			}

			// Check pod template annotations
			podAnnot := deployment.Spec.Template.GetAnnotations()
			if tt.expectedPodAnnot == nil {
				if podAnnot != nil && len(podAnnot) > 0 {
					t.Errorf("expected no pod annotations, got %v", podAnnot)
				}
			} else {
				for k, v := range tt.expectedPodAnnot {
					if got, ok := podAnnot[k]; !ok || got != v {
						t.Errorf("expected pod annotation %q=%q, got %q", k, v, got)
					}
				}
			}
		})
	}
}

func TestApplyComponentConfigToDeployment_RevisionHistoryLimit(t *testing.T) {
	tests := []struct {
		name                    string
		componentConfigs        []operatorv1alpha1.ComponentConfig
		assetName               string
		expectedRevisionHistory *int32
	}{
		{
			name:                    "no component configs",
			componentConfigs:        nil,
			assetName:               controllerDeploymentAssetName,
			expectedRevisionHistory: nil,
		},
		{
			name: "matching component config sets revisionHistoryLimit",
			componentConfigs: []operatorv1alpha1.ComponentConfig{
				{
					ComponentName: operatorv1alpha1.CoreController,
					DeploymentConfigs: operatorv1alpha1.DeploymentConfig{
						RevisionHistoryLimit: ptr.To(int32(5)),
					},
				},
			},
			assetName:               controllerDeploymentAssetName,
			expectedRevisionHistory: ptr.To(int32(5)),
		},
		{
			name: "non-matching component config",
			componentConfigs: []operatorv1alpha1.ComponentConfig{
				{
					ComponentName: operatorv1alpha1.Webhook,
					DeploymentConfigs: operatorv1alpha1.DeploymentConfig{
						RevisionHistoryLimit: ptr.To(int32(3)),
					},
				},
			},
			assetName:               controllerDeploymentAssetName,
			expectedRevisionHistory: nil,
		},
		{
			name: "webhook component config",
			componentConfigs: []operatorv1alpha1.ComponentConfig{
				{
					ComponentName: operatorv1alpha1.Webhook,
					DeploymentConfigs: operatorv1alpha1.DeploymentConfig{
						RevisionHistoryLimit: ptr.To(int32(10)),
					},
				},
			},
			assetName:               webhookDeploymentAssetName,
			expectedRevisionHistory: ptr.To(int32(10)),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newTestReconciler()
			deployment := newTestDeployment("external-secrets")
			esc := newTestESC()
			esc.Spec.ControllerConfig.ComponentConfigs = tt.componentConfigs

			err := r.applyComponentConfigToDeployment(deployment, esc, tt.assetName)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.expectedRevisionHistory == nil {
				if deployment.Spec.RevisionHistoryLimit != nil {
					t.Errorf("expected nil revisionHistoryLimit, got %d", *deployment.Spec.RevisionHistoryLimit)
				}
			} else {
				if deployment.Spec.RevisionHistoryLimit == nil {
					t.Errorf("expected revisionHistoryLimit=%d, got nil", *tt.expectedRevisionHistory)
				} else if *deployment.Spec.RevisionHistoryLimit != *tt.expectedRevisionHistory {
					t.Errorf("expected revisionHistoryLimit=%d, got %d", *tt.expectedRevisionHistory, *deployment.Spec.RevisionHistoryLimit)
				}
			}
		})
	}
}

func TestApplyOverrideEnvToContainer(t *testing.T) {
	tests := []struct {
		name          string
		containerName string
		overrideEnv   []corev1.EnvVar
		expectedEnv   []corev1.EnvVar
		expectError   bool
	}{
		{
			name:          "new env var is appended",
			containerName: "external-secrets",
			overrideEnv: []corev1.EnvVar{
				{Name: "NEW_VAR", Value: "new-value"},
			},
			expectedEnv: []corev1.EnvVar{
				{Name: "EXISTING_VAR", Value: "existing-value"},
				{Name: "NEW_VAR", Value: "new-value"},
			},
		},
		{
			name:          "existing env var is overridden",
			containerName: "external-secrets",
			overrideEnv: []corev1.EnvVar{
				{Name: "EXISTING_VAR", Value: "overridden-value"},
			},
			expectedEnv: []corev1.EnvVar{
				{Name: "EXISTING_VAR", Value: "overridden-value"},
			},
		},
		{
			name:          "container not found",
			containerName: "nonexistent-container",
			overrideEnv: []corev1.EnvVar{
				{Name: "NEW_VAR", Value: "new-value"},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deployment := newTestDeployment("external-secrets")

			err := applyOverrideEnvToContainer(deployment, tt.containerName, tt.overrideEnv)

			if tt.expectError {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			container := deployment.Spec.Template.Spec.Containers[0]
			if len(container.Env) != len(tt.expectedEnv) {
				t.Errorf("expected %d env vars, got %d", len(tt.expectedEnv), len(container.Env))
			}

			for _, expected := range tt.expectedEnv {
				found := false
				for _, actual := range container.Env {
					if actual.Name == expected.Name && actual.Value == expected.Value {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected env var %s=%s not found", expected.Name, expected.Value)
				}
			}
		})
	}
}

func TestGetComponentNameForAsset(t *testing.T) {
	tests := []struct {
		assetName         string
		expectedComponent operatorv1alpha1.ComponentName
		expectedFound     bool
	}{
		{controllerDeploymentAssetName, operatorv1alpha1.CoreController, true},
		{webhookDeploymentAssetName, operatorv1alpha1.Webhook, true},
		{certControllerDeploymentAssetName, operatorv1alpha1.CertController, true},
		{bitwardenDeploymentAssetName, operatorv1alpha1.BitwardenSDKServer, true},
		{"unknown-asset", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.assetName, func(t *testing.T) {
			component, found := getComponentNameForAsset(tt.assetName)
			if found != tt.expectedFound {
				t.Errorf("expected found=%v, got %v", tt.expectedFound, found)
			}
			if component != tt.expectedComponent {
				t.Errorf("expected component=%q, got %q", tt.expectedComponent, component)
			}
		})
	}
}
