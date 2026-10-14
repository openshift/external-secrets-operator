package common

import (
	"encoding/base64"
	"reflect"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	operatorv1alpha1 "github.com/openshift/external-secrets-operator/api/v1alpha1"
)

// TestUpdateResourceAnnotations verifies UpdateResourceAnnotations merges user annotations
// into the object and does not set the managed-annotations tracking key on resources.
func TestUpdateResourceAnnotations(t *testing.T) {
	tests := []struct {
		name            string
		existingAnnots  map[string]string
		userAnnotations map[string]string
		wantAnnotations map[string]string
	}{
		{
			name:            "sets managed annotations on object with no existing annotations",
			existingAnnots:  nil,
			userAnnotations: map[string]string{"foo": "bar", "baz": "qux"},
			wantAnnotations: map[string]string{"foo": "bar", "baz": "qux"},
		},
		{
			name:            "preserves existing annotations",
			existingAnnots:  map[string]string{"existing": "value"},
			userAnnotations: map[string]string{"foo": "bar"},
			wantAnnotations: map[string]string{"existing": "value", "foo": "bar"},
		},
		{
			name:            "empty user annotations",
			existingAnnots:  map[string]string{"existing": "value"},
			userAnnotations: map[string]string{},
			wantAnnotations: map[string]string{"existing": "value"},
		},
		{
			name:            "nil user annotations",
			existingAnnots:  nil,
			userAnnotations: nil,
			wantAnnotations: map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obj := &corev1.ConfigMap{
				ObjectMeta: v1.ObjectMeta{
					Name:        "test",
					Annotations: tt.existingAnnots,
				},
			}

			UpdateResourceAnnotations(obj, tt.userAnnotations)

			// Verify tracking annotation is NOT set on the resource
			if _, exists := obj.GetAnnotations()[ManagedAnnotationsKey]; exists {
				t.Error("tracking annotation should not be set on managed resource")
			}

			// Verify user annotations are present
			annotations := obj.GetAnnotations()
			for k, v := range tt.wantAnnotations {
				if annotations[k] != v {
					t.Errorf("annotation %q = %q, want %q", k, annotations[k], v)
				}
			}
		})
	}
}

func TestObjectMetadataModified(t *testing.T) {
	tests := []struct {
		name               string
		metadata           ResourceMetadata
		desiredAnnotations map[string]string
		fetchedAnnotations map[string]string
		want               bool
	}{
		{
			name: "managed annotation changed - returns true",
			metadata: ResourceMetadata{
				Annotations:           map[string]string{"user-key": "new-value"},
				DeletedAnnotationKeys: []string{},
			},
			desiredAnnotations: map[string]string{
				"user-key": "new-value",
			},
			fetchedAnnotations: map[string]string{
				"user-key": "old-value",
			},
			want: true,
		},
		{
			name: "unmanaged annotation added by external actor - ignored, returns false",
			metadata: ResourceMetadata{
				Annotations:           map[string]string{"user-key": "same-value"},
				DeletedAnnotationKeys: []string{},
			},
			desiredAnnotations: map[string]string{
				"user-key": "same-value",
			},
			fetchedAnnotations: map[string]string{
				"user-key":                          "same-value",
				"deployment.kubernetes.io/revision": "4",
			},
			want: false,
		},
		{
			name: "managed annotation removed from desired - returns true",
			metadata: ResourceMetadata{
				Annotations:           map[string]string{},
				DeletedAnnotationKeys: []string{"removed-key"},
			},
			desiredAnnotations: map[string]string{},
			fetchedAnnotations: map[string]string{
				"removed-key": "value",
			},
			want: true,
		},
		{
			name:               "empty desired annotations - external annotations ignored",
			metadata:           ResourceMetadata{},
			desiredAnnotations: map[string]string{},
			fetchedAnnotations: map[string]string{
				"deployment.kubernetes.io/revision": "4",
			},
			want: false,
		},
		{
			name:               "both nil - no change",
			metadata:           ResourceMetadata{},
			desiredAnnotations: nil,
			fetchedAnnotations: nil,
			want:               false,
		},
		{
			name: "new managed annotation added - returns true",
			metadata: ResourceMetadata{
				Annotations:           map[string]string{"new-key": "value"},
				DeletedAnnotationKeys: []string{},
			},
			desiredAnnotations: map[string]string{
				"new-key": "value",
			},
			fetchedAnnotations: map[string]string{},
			want:               true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			desired := &corev1.ConfigMap{
				ObjectMeta: v1.ObjectMeta{
					Name:        "test",
					Annotations: tt.desiredAnnotations,
				},
			}
			fetched := &corev1.ConfigMap{
				ObjectMeta: v1.ObjectMeta{
					Name:        "test",
					Annotations: tt.fetchedAnnotations,
				},
			}
			got := ObjectMetadataModified(desired, fetched, &tt.metadata)
			if got != tt.want {
				t.Errorf("ObjectMetadataModified() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetPreviouslyAppliedAnnotationKeys(t *testing.T) {
	tests := []struct {
		name        string
		annotations map[string]string
		want        []string
		wantErr     bool
	}{
		{
			name:        "nil annotations",
			annotations: nil,
			want:        nil,
		},
		{
			name:        "no tracking annotation",
			annotations: map[string]string{"foo": "bar"},
			want:        nil,
		},
		{
			name:        "empty tracking annotation",
			annotations: map[string]string{ManagedAnnotationsKey: ""},
			want:        nil,
		},
		{
			name:        "invalid base64 in tracking annotation",
			annotations: map[string]string{ManagedAnnotationsKey: "!!!"},
			wantErr:     true,
		},
		{
			name: "valid keys",
			annotations: map[string]string{
				ManagedAnnotationsKey: base64.StdEncoding.EncodeToString([]byte(`["key1","key2"]`)),
			},
			want: []string{"key1", "key2"},
		},
		{
			name: "valid base64 but invalid JSON",
			annotations: map[string]string{
				ManagedAnnotationsKey: base64.StdEncoding.EncodeToString([]byte(`["key1`)),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetPreviouslyAppliedAnnotationKeys(tt.annotations, ManagedAnnotationsKey)
			if (err != nil) != tt.wantErr {
				t.Fatalf("GetPreviouslyAppliedAnnotationKeys() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetPreviouslyAppliedAnnotationKeys() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAddManagedMetadataAnnotation(t *testing.T) {
	tests := []struct {
		name            string
		specAnnotations map[string]string
		crAnnotations   map[string]string
		deletedKeys     []string
		wantNeedsUpdate bool
		wantErr         bool
	}{
		{
			name:            "first time - no previous keys, sets tracking annotation",
			specAnnotations: map[string]string{"foo": "bar"},
			crAnnotations:   nil,
			wantNeedsUpdate: true,
		},
		{
			name:            "same keys - no update needed",
			specAnnotations: map[string]string{"foo": "bar"},
			crAnnotations: map[string]string{
				ManagedAnnotationsKey: base64.StdEncoding.EncodeToString([]byte(`["foo"]`)),
			},
			wantNeedsUpdate: false,
		},
		{
			name:            "key added - update needed",
			specAnnotations: map[string]string{"foo": "bar", "baz": "qux"},
			crAnnotations: map[string]string{
				ManagedAnnotationsKey: base64.StdEncoding.EncodeToString([]byte(`["foo"]`)),
			},
			wantNeedsUpdate: true,
		},
		{
			name:            "key removed - update needed via deletedKeys",
			specAnnotations: map[string]string{"foo": "bar"},
			crAnnotations: map[string]string{
				ManagedAnnotationsKey: base64.StdEncoding.EncodeToString([]byte(`["foo","old"]`)),
			},
			deletedKeys:     []string{"old"},
			wantNeedsUpdate: true,
		},
		{
			name:            "empty spec annotations, no previous",
			specAnnotations: nil,
			crAnnotations:   nil,
			wantNeedsUpdate: false,
		},
		{
			name:            "invalid tracking annotation value returns error",
			specAnnotations: map[string]string{"foo": "bar"},
			crAnnotations: map[string]string{
				ManagedAnnotationsKey: "!!!",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			esc := &operatorv1alpha1.ExternalSecretsConfig{
				ObjectMeta: v1.ObjectMeta{
					Name:        "test",
					Annotations: tt.crAnnotations,
				},
			}
			esc.Spec.ControllerConfig.Annotations = tt.specAnnotations

			metadata := ResourceMetadata{
				Annotations:           tt.specAnnotations,
				DeletedAnnotationKeys: tt.deletedKeys,
			}
			needsUpdate, err := AddManagedMetadataAnnotation(esc, ManagedAnnotationsKey, metadata)
			if (err != nil) != tt.wantErr {
				t.Fatalf("AddManagedMetadataAnnotation() error = %v, wantErr %v", err, tt.wantErr)
			}
			if needsUpdate != tt.wantNeedsUpdate {
				t.Errorf("needsUpdate = %v, want %v", needsUpdate, tt.wantNeedsUpdate)
			}
			if tt.wantNeedsUpdate && !tt.wantErr {
				// Verify tracking annotation was set on the CR
				crAnnots := esc.GetAnnotations()
				if crAnnots == nil {
					t.Fatal("expected annotations to be set on CR")
				}
				if _, ok := crAnnots[ManagedAnnotationsKey]; !ok {
					t.Error("expected ManagedAnnotationsKey to be set on CR")
				}
			}
		})
	}
}

func TestRemoveObsoleteAnnotations(t *testing.T) {
	t.Run("removes deleted keys from object metadata", func(t *testing.T) {
		obj := &corev1.ConfigMap{
			ObjectMeta: v1.ObjectMeta{
				Name:        "test",
				Annotations: map[string]string{"keep": "v1", "remove-me": "v2", "also-remove": "v3"},
			},
		}
		meta := ResourceMetadata{
			Annotations:           map[string]string{"keep": "v1"},
			DeletedAnnotationKeys: []string{"remove-me", "also-remove"},
		}
		RemoveObsoleteAnnotations(obj, meta)
		ann := obj.GetAnnotations()
		if ann["keep"] != "v1" {
			t.Errorf("expected keep=v1, got %q", ann["keep"])
		}
		if _, ok := ann["remove-me"]; ok {
			t.Error("expected remove-me to be removed")
		}
		if _, ok := ann["also-remove"]; ok {
			t.Error("expected also-remove to be removed")
		}
	})

	t.Run("removes deleted keys from Deployment pod template", func(t *testing.T) {
		deploy := &appsv1.Deployment{
			ObjectMeta: v1.ObjectMeta{
				Name:        "test",
				Annotations: map[string]string{"obsolete-on-object": "x"},
			},
			Spec: appsv1.DeploymentSpec{
				Template: corev1.PodTemplateSpec{
					ObjectMeta: v1.ObjectMeta{
						Annotations: map[string]string{"keep-pod": "v1", "obsolete-on-pod": "v2"},
					},
				},
			},
		}
		meta := ResourceMetadata{
			Annotations:           map[string]string{"keep-pod": "v1"},
			DeletedAnnotationKeys: []string{"obsolete-on-object", "obsolete-on-pod"},
		}
		RemoveObsoleteAnnotations(deploy, meta)
		if _, ok := deploy.GetAnnotations()["obsolete-on-object"]; ok {
			t.Error("expected obsolete-on-object to be removed from object")
		}
		templateAnn := deploy.Spec.Template.GetAnnotations()
		if templateAnn["keep-pod"] != "v1" {
			t.Errorf("expected keep-pod=v1 on template, got %q", templateAnn["keep-pod"])
		}
		if _, ok := templateAnn["obsolete-on-pod"]; ok {
			t.Error("expected obsolete-on-pod to be removed from pod template")
		}
	})

	t.Run("no-op when DeletedAnnotationKeys is empty", func(t *testing.T) {
		obj := &corev1.ConfigMap{
			ObjectMeta: v1.ObjectMeta{
				Name:        "test",
				Annotations: map[string]string{"foo": "bar"},
			},
		}
		meta := ResourceMetadata{
			Annotations:           map[string]string{"foo": "bar"},
			DeletedAnnotationKeys: nil,
		}
		RemoveObsoleteAnnotations(obj, meta)
		if obj.GetAnnotations()["foo"] != "bar" {
			t.Errorf("expected foo=bar unchanged, got %q", obj.GetAnnotations()["foo"])
		}
	})
}

func TestDeploymentObjectChanged(t *testing.T) {
	t.Run("unmanaged annotation added by foreign actor should not lead to change", func(t *testing.T) {
		// Deployment with external annotation on fetched should NOT trigger change
		// when desired has no managed annotations — external annotations are ignored
		fetched := appsv1.Deployment{
			ObjectMeta: v1.ObjectMeta{
				Annotations: map[string]string{
					"deployment.kubernetes.io/revision": "4",
				},
			},
		}

		desired := appsv1.Deployment{
			ObjectMeta: v1.ObjectMeta{
				Annotations: map[string]string{},
			},
		}

		managedMetaState := ResourceMetadata{}
		if HasObjectChanged(&desired, &fetched, &managedMetaState) {
			t.Fatal("expected no change when fetched only has external annotations")
		}
	})

	t.Run("managed annotation updated by foreign actor should trigger change", func(t *testing.T) {
		desired := appsv1.Deployment{
			ObjectMeta: v1.ObjectMeta{
				Annotations: map[string]string{
					"my-annotation": "new-value",
				},
			},
		}

		fetched := appsv1.Deployment{
			ObjectMeta: v1.ObjectMeta{
				Annotations: map[string]string{
					"my-annotation":                     "old-value",
					"deployment.kubernetes.io/revision": "4",
				},
			},
		}

		managedMetaState := ResourceMetadata{
			Annotations:           map[string]string{"my-annotation": "new-value"},
			DeletedAnnotationKeys: []string{},
		}

		if !HasObjectChanged(&desired, &fetched, &managedMetaState) {
			t.Fatal("expected change when managed annotation value differs")
		}
	})

	t.Run("unmanaged pod annotation added by foreign actor should not lead to change", func(t *testing.T) {
		desired := appsv1.Deployment{
			Spec: appsv1.DeploymentSpec{
				Template: corev1.PodTemplateSpec{
					ObjectMeta: v1.ObjectMeta{
						Annotations: map[string]string{
							"user-key": "value",
						},
					},
				},
			},
		}

		fetched := appsv1.Deployment{
			Spec: appsv1.DeploymentSpec{
				Template: corev1.PodTemplateSpec{
					ObjectMeta: v1.ObjectMeta{
						Annotations: map[string]string{
							"user-key":                          "value",
							"kubectl.kubernetes.io/restartedAt": "2024-01-01",
						},
					},
				},
			},
		}

		managedMetaState := ResourceMetadata{
			Annotations:           map[string]string{"user-key": "value"},
			DeletedAnnotationKeys: []string{},
		}

		if HasObjectChanged(&desired, &fetched, &managedMetaState) {
			t.Fatal("expected no change when fetched pod template only has extra external annotations")
		}
	})

	t.Run("env var order difference should not trigger change", func(t *testing.T) {
		desired := deploymentWithContainerEnv([]corev1.EnvVar{
			{Name: "HTTP_PROXY", Value: "http://proxy.example.com"},
			{Name: "HTTPS_PROXY", Value: "https://proxy.example.com"},
			{Name: "NO_PROXY", Value: "localhost"},
		})
		fetched := deploymentWithContainerEnv([]corev1.EnvVar{
			{Name: "HTTPS_PROXY", Value: "https://proxy.example.com"},
			{Name: "HTTP_PROXY", Value: "http://proxy.example.com"},
			{Name: "NO_PROXY", Value: "localhost"},
		})

		if HasObjectChanged(&desired, &fetched, &ResourceMetadata{}) {
			t.Fatal("expected no change when env vars differ only in order")
		}
	})

	t.Run("env var value difference should trigger change", func(t *testing.T) {
		desired := deploymentWithContainerEnv([]corev1.EnvVar{
			{Name: "HTTP_PROXY", Value: "http://proxy.example.com"},
		})
		fetched := deploymentWithContainerEnv([]corev1.EnvVar{
			{Name: "HTTP_PROXY", Value: "http://other.example.com"},
		})

		if !HasObjectChanged(&desired, &fetched, &ResourceMetadata{}) {
			t.Fatal("expected change when env var value differs")
		}
	})

	t.Run("volume mount order difference should not trigger change", func(t *testing.T) {
		desired := deploymentWithContainerVolumeMounts([]corev1.VolumeMount{
			{Name: "trusted-ca-bundle", MountPath: "/etc/pki/tls/certs", ReadOnly: true},
			{Name: "user-ca-bundle", MountPath: "/etc/pki/tls/user-certs", ReadOnly: true},
		})
		fetched := deploymentWithContainerVolumeMounts([]corev1.VolumeMount{
			{Name: "user-ca-bundle", MountPath: "/etc/pki/tls/user-certs", ReadOnly: true},
			{Name: "trusted-ca-bundle", MountPath: "/etc/pki/tls/certs", ReadOnly: true},
		})

		if HasObjectChanged(&desired, &fetched, &ResourceMetadata{}) {
			t.Fatal("expected no change when volume mounts differ only in order")
		}
	})

	t.Run("volume mount path difference should trigger change", func(t *testing.T) {
		desired := deploymentWithContainerVolumeMounts([]corev1.VolumeMount{
			{Name: "trusted-ca-bundle", MountPath: "/etc/pki/tls/certs", ReadOnly: true},
		})
		fetched := deploymentWithContainerVolumeMounts([]corev1.VolumeMount{
			{Name: "trusted-ca-bundle", MountPath: "/etc/ssl/certs", ReadOnly: true},
		})

		if !HasObjectChanged(&desired, &fetched, &ResourceMetadata{}) {
			t.Fatal("expected change when volume mount path differs")
		}
	})
}

func deploymentWithContainerEnv(env []corev1.EnvVar) appsv1.Deployment {
	return appsv1.Deployment{
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "external-secrets",
							Image: "ghcr.io/external-secrets/external-secrets:v2.12.0",
							Env:   env,
						},
					},
				},
			},
		},
	}
}

func deploymentWithContainerVolumeMounts(mounts []corev1.VolumeMount) appsv1.Deployment {
	return appsv1.Deployment{
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:         "external-secrets",
							Image:        "ghcr.io/external-secrets/external-secrets:v2.12.0",
							VolumeMounts: mounts,
						},
					},
				},
			},
		},
	}
}

func TestIsFeatureEnabled(t *testing.T) {
	enabledESM := &operatorv1alpha1.ExternalSecretsManager{
		Spec: operatorv1alpha1.ExternalSecretsManagerSpec{
			Features: []operatorv1alpha1.Feature{
				{Name: operatorv1alpha1.UnsafeAllowGenericTargets, Mode: operatorv1alpha1.Enabled},
			},
		},
	}
	disabledESM := &operatorv1alpha1.ExternalSecretsManager{
		Spec: operatorv1alpha1.ExternalSecretsManagerSpec{
			Features: []operatorv1alpha1.Feature{
				{Name: operatorv1alpha1.UnsafeAllowGenericTargets, Mode: operatorv1alpha1.Disabled},
			},
		},
	}
	defaultModeESM := &operatorv1alpha1.ExternalSecretsManager{
		Spec: operatorv1alpha1.ExternalSecretsManagerSpec{
			Features: []operatorv1alpha1.Feature{
				{Name: operatorv1alpha1.UnsafeAllowGenericTargets},
			},
		},
	}

	tests := []struct {
		name string
		esm  *operatorv1alpha1.ExternalSecretsManager
		want bool
	}{
		{name: "nil esm returns false", esm: nil, want: false},
		{name: "missing feature returns false", esm: &operatorv1alpha1.ExternalSecretsManager{}, want: false},
		{name: "enabled feature", esm: enabledESM, want: true},
		{name: "disabled feature", esm: disabledESM, want: false},
		{name: "omitted mode defaults to false", esm: defaultModeESM, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsFeatureEnabled(tt.esm, operatorv1alpha1.UnsafeAllowGenericTargets); got != tt.want {
				t.Errorf("IsFeatureEnabled() = %v, want %v", got, tt.want)
			}
		})
	}
}
