package external_secrets

import (
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	operatorv1alpha1 "github.com/openshift/external-secrets-operator/api/v1alpha1"
)

func TestApplyAnnotations(t *testing.T) {
	tests := []struct {
		name                   string
		existingAnnotations    map[string]string
		existingPodAnnotations map[string]string
		annotations            []operatorv1alpha1.Annotation
		expectedAnnotations    map[string]string
		expectedPodAnnotations map[string]string
	}{
		{
			name:                   "no annotations to apply",
			existingAnnotations:    nil,
			existingPodAnnotations: nil,
			annotations:            nil,
			expectedAnnotations:    nil,
			expectedPodAnnotations: nil,
		},
		{
			name:                   "apply single annotation",
			existingAnnotations:    nil,
			existingPodAnnotations: nil,
			annotations: []operatorv1alpha1.Annotation{
				{KVPair: operatorv1alpha1.KVPair{Key: "example.com/custom", Value: "test-value"}},
			},
			expectedAnnotations:    map[string]string{"example.com/custom": "test-value"},
			expectedPodAnnotations: map[string]string{"example.com/custom": "test-value"},
		},
		{
			name:                "apply multiple annotations",
			existingAnnotations: map[string]string{"existing": "annotation"},
			annotations: []operatorv1alpha1.Annotation{
				{KVPair: operatorv1alpha1.KVPair{Key: "example.com/first", Value: "value-1"}},
				{KVPair: operatorv1alpha1.KVPair{Key: "example.com/second", Value: "value-2"}},
			},
			expectedAnnotations: map[string]string{
				"existing":           "annotation",
				"example.com/first":  "value-1",
				"example.com/second": "value-2",
			},
			expectedPodAnnotations: map[string]string{
				"example.com/first":  "value-1",
				"example.com/second": "value-2",
			},
		},
		{
			name:                "override existing annotation",
			existingAnnotations: map[string]string{"example.com/key": "old-value"},
			annotations: []operatorv1alpha1.Annotation{
				{KVPair: operatorv1alpha1.KVPair{Key: "example.com/key", Value: "new-value"}},
			},
			expectedAnnotations:    map[string]string{"example.com/key": "new-value"},
			expectedPodAnnotations: map[string]string{"example.com/key": "new-value"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deployment := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test-deployment",
					Annotations: tt.existingAnnotations,
				},
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Annotations: tt.existingPodAnnotations,
						},
					},
				},
			}

			applyAnnotations(deployment, tt.annotations)

			// Check deployment annotations
			if tt.expectedAnnotations == nil {
				if len(deployment.GetAnnotations()) != 0 && tt.existingAnnotations == nil {
					t.Errorf("expected no deployment annotations, got %v", deployment.GetAnnotations())
				}
			} else {
				for k, v := range tt.expectedAnnotations {
					actual, ok := deployment.GetAnnotations()[k]
					if !ok {
						t.Errorf("expected deployment annotation %s not found", k)
					}
					if actual != v {
						t.Errorf("expected deployment annotation %s=%s, got %s", k, v, actual)
					}
				}
			}

			// Check pod template annotations
			if tt.expectedPodAnnotations == nil {
				if len(deployment.Spec.Template.GetAnnotations()) != 0 && tt.existingPodAnnotations == nil {
					t.Errorf("expected no pod annotations, got %v", deployment.Spec.Template.GetAnnotations())
				}
			} else {
				for k, v := range tt.expectedPodAnnotations {
					actual, ok := deployment.Spec.Template.GetAnnotations()[k]
					if !ok {
						t.Errorf("expected pod annotation %s not found", k)
					}
					if actual != v {
						t.Errorf("expected pod annotation %s=%s, got %s", k, v, actual)
					}
				}
			}
		})
	}
}

func TestApplyComponentConfig(t *testing.T) {
	tests := []struct {
		name                         string
		componentConfigs             []operatorv1alpha1.ComponentConfig
		componentName                operatorv1alpha1.ComponentName
		expectedRevisionHistoryLimit *int32
		expectedEnvVars              []corev1.EnvVar
		containerName                string
	}{
		{
			name:                         "no component configs",
			componentConfigs:             nil,
			componentName:                operatorv1alpha1.CoreController,
			expectedRevisionHistoryLimit: nil,
			containerName:                "external-secrets",
		},
		{
			name: "apply revisionHistoryLimit",
			componentConfigs: []operatorv1alpha1.ComponentConfig{
				{
					ComponentName: operatorv1alpha1.CoreController,
					DeploymentConfigs: operatorv1alpha1.DeploymentConfig{
						RevisionHistoryLimit: ptr.To(int32(5)),
					},
				},
			},
			componentName:                operatorv1alpha1.CoreController,
			expectedRevisionHistoryLimit: ptr.To(int32(5)),
			containerName:                "external-secrets",
		},
		{
			name: "apply overrideEnv to controller",
			componentConfigs: []operatorv1alpha1.ComponentConfig{
				{
					ComponentName: operatorv1alpha1.CoreController,
					OverrideEnv: []corev1.EnvVar{
						{Name: "GOMAXPROCS", Value: "4"},
					},
				},
			},
			componentName: operatorv1alpha1.CoreController,
			expectedEnvVars: []corev1.EnvVar{
				{Name: "GOMAXPROCS", Value: "4"},
			},
			containerName: "external-secrets",
		},
		{
			name: "apply overrideEnv to webhook",
			componentConfigs: []operatorv1alpha1.ComponentConfig{
				{
					ComponentName: operatorv1alpha1.Webhook,
					OverrideEnv: []corev1.EnvVar{
						{Name: "MY_VAR", Value: "value"},
					},
				},
			},
			componentName: operatorv1alpha1.Webhook,
			expectedEnvVars: []corev1.EnvVar{
				{Name: "MY_VAR", Value: "value"},
			},
			containerName: "webhook",
		},
		{
			name: "no matching component config",
			componentConfigs: []operatorv1alpha1.ComponentConfig{
				{
					ComponentName: operatorv1alpha1.Webhook,
					DeploymentConfigs: operatorv1alpha1.DeploymentConfig{
						RevisionHistoryLimit: ptr.To(int32(3)),
					},
				},
			},
			componentName:                operatorv1alpha1.CoreController,
			expectedRevisionHistoryLimit: nil,
			containerName:                "external-secrets",
		},
		{
			name: "apply both revisionHistoryLimit and overrideEnv",
			componentConfigs: []operatorv1alpha1.ComponentConfig{
				{
					ComponentName: operatorv1alpha1.CertController,
					DeploymentConfigs: operatorv1alpha1.DeploymentConfig{
						RevisionHistoryLimit: ptr.To(int32(10)),
					},
					OverrideEnv: []corev1.EnvVar{
						{Name: "CUSTOM_VAR", Value: "custom-value"},
					},
				},
			},
			componentName:                operatorv1alpha1.CertController,
			expectedRevisionHistoryLimit: ptr.To(int32(10)),
			expectedEnvVars: []corev1.EnvVar{
				{Name: "CUSTOM_VAR", Value: "custom-value"},
			},
			containerName: "cert-controller",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deployment := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-deployment",
				},
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{Name: tt.containerName},
							},
						},
					},
				},
			}

			applyComponentConfig(deployment, tt.componentConfigs, tt.componentName)

			// Check revisionHistoryLimit
			if tt.expectedRevisionHistoryLimit != nil {
				if deployment.Spec.RevisionHistoryLimit == nil {
					t.Error("expected revisionHistoryLimit to be set, got nil")
				} else if *deployment.Spec.RevisionHistoryLimit != *tt.expectedRevisionHistoryLimit {
					t.Errorf("expected revisionHistoryLimit %d, got %d", *tt.expectedRevisionHistoryLimit, *deployment.Spec.RevisionHistoryLimit)
				}
			} else {
				if deployment.Spec.RevisionHistoryLimit != nil {
					t.Errorf("expected revisionHistoryLimit to be nil, got %d", *deployment.Spec.RevisionHistoryLimit)
				}
			}

			// Check env vars
			if tt.expectedEnvVars != nil {
				for _, container := range deployment.Spec.Template.Spec.Containers {
					if container.Name == tt.containerName {
						for _, expected := range tt.expectedEnvVars {
							found := false
							for _, env := range container.Env {
								if env.Name == expected.Name && env.Value == expected.Value {
									found = true
									break
								}
							}
							if !found {
								t.Errorf("expected env var %s=%s not found in container %s", expected.Name, expected.Value, tt.containerName)
							}
						}
					}
				}
			}
		})
	}
}

func TestApplyOverrideEnv(t *testing.T) {
	tests := []struct {
		name          string
		existingEnv   []corev1.EnvVar
		overrideEnv   []corev1.EnvVar
		containerName string
		expectedEnv   []corev1.EnvVar
	}{
		{
			name:          "add env vars to empty container",
			existingEnv:   nil,
			overrideEnv:   []corev1.EnvVar{{Name: "VAR1", Value: "val1"}},
			containerName: "external-secrets",
			expectedEnv:   []corev1.EnvVar{{Name: "VAR1", Value: "val1"}},
		},
		{
			name:          "override existing env var",
			existingEnv:   []corev1.EnvVar{{Name: "VAR1", Value: "old-val"}},
			overrideEnv:   []corev1.EnvVar{{Name: "VAR1", Value: "new-val"}},
			containerName: "external-secrets",
			expectedEnv:   []corev1.EnvVar{{Name: "VAR1", Value: "new-val"}},
		},
		{
			name:          "add new env var alongside existing",
			existingEnv:   []corev1.EnvVar{{Name: "EXISTING", Value: "val"}},
			overrideEnv:   []corev1.EnvVar{{Name: "NEW_VAR", Value: "new-val"}},
			containerName: "external-secrets",
			expectedEnv: []corev1.EnvVar{
				{Name: "EXISTING", Value: "val"},
				{Name: "NEW_VAR", Value: "new-val"},
			},
		},
		{
			name:          "wrong container name - no effect",
			existingEnv:   nil,
			overrideEnv:   []corev1.EnvVar{{Name: "VAR1", Value: "val1"}},
			containerName: "wrong-container",
			expectedEnv:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deployment := &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name: "external-secrets",
									Env:  tt.existingEnv,
								},
							},
						},
					},
				},
			}

			applyOverrideEnv(deployment, tt.containerName, tt.overrideEnv)

			container := deployment.Spec.Template.Spec.Containers[0]
			if tt.expectedEnv == nil && tt.containerName == "wrong-container" {
				if len(container.Env) != len(tt.existingEnv) {
					t.Errorf("expected env to be unchanged")
				}
				return
			}

			if tt.expectedEnv != nil {
				for _, expected := range tt.expectedEnv {
					found := false
					for _, env := range container.Env {
						if env.Name == expected.Name && env.Value == expected.Value {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("expected env var %s=%s not found", expected.Name, expected.Value)
					}
				}
			}
		})
	}
}

func TestGetComponentNameForAsset(t *testing.T) {
	tests := []struct {
		assetName    string
		expectedName operatorv1alpha1.ComponentName
		expectError  bool
	}{
		{controllerDeploymentAssetName, operatorv1alpha1.CoreController, false},
		{webhookDeploymentAssetName, operatorv1alpha1.Webhook, false},
		{certControllerDeploymentAssetName, operatorv1alpha1.CertController, false},
		{bitwardenDeploymentAssetName, operatorv1alpha1.BitwardenSDKServer, false},
		{"unknown-asset", "", true},
	}

	for _, tt := range tests {
		t.Run(string(tt.assetName), func(t *testing.T) {
			name, err := getComponentNameForAsset(tt.assetName)
			if tt.expectError {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if name != tt.expectedName {
				t.Errorf("expected %s, got %s", tt.expectedName, name)
			}
		})
	}
}
