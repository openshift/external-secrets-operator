package common

import (
	"encoding/base64"
	"reflect"
	"testing"

	"github.com/go-logr/logr/testr"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	operatorv1alpha1 "github.com/openshift/external-secrets-operator/api/v1alpha1"
)

func TestSetManagedAnnotations(t *testing.T) {
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

func TestGetManagedAnnotationKeys(t *testing.T) {
	tests := []struct {
		name        string
		annotations map[string]string
		want        []string
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
			name:        "invalid base64",
			annotations: map[string]string{ManagedAnnotationsKey: "not-valid-base64!!!"},
			want:        nil,
		},
		{
			name: "valid tracking annotation",
			annotations: map[string]string{
				ManagedAnnotationsKey: base64.StdEncoding.EncodeToString([]byte(`["foo", "bar"]`)),
			},
			want: []string{"foo", "bar"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			esc := &operatorv1alpha1.ExternalSecretsConfig{
				ObjectMeta: v1.ObjectMeta{
					Name:        "test",
					Annotations: tt.annotations,
				},
			}
			got := GetManagedAnnotationKeys(esc)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetManagedAnnotationKeys() = %v, want %v", got, tt.want)
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetPreviouslyAppliedAnnotationKeys(tt.annotations)
			if (err != nil) != tt.wantErr {
				t.Fatalf("GetPreviouslyAppliedAnnotationKeys() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetPreviouslyAppliedAnnotationKeys() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetResourceAnnotations(t *testing.T) {
	tests := []struct {
		name            string
		specAnnotations map[string]string
		crAnnotations   map[string]string
		wantAnnotations map[string]string
		wantDeletedKeys []string
		wantErr         bool
	}{
		{
			name:            "nil spec annotations, no previous tracking",
			specAnnotations: nil,
			crAnnotations:   nil,
			wantAnnotations: map[string]string{},
		},
		{
			name:            "spec annotations with no previous tracking",
			specAnnotations: map[string]string{"foo": "bar"},
			crAnnotations:   nil,
			wantAnnotations: map[string]string{"foo": "bar"},
		},
		{
			name:            "removed annotation detected as deleted",
			specAnnotations: map[string]string{"foo": "bar"},
			crAnnotations: map[string]string{
				ManagedAnnotationsKey: base64.StdEncoding.EncodeToString([]byte(`["foo","old-key"]`)),
			},
			wantAnnotations: map[string]string{"foo": "bar"},
			wantDeletedKeys: []string{"old-key"},
		},
		{
			name:            "no deletions when all previous keys still present",
			specAnnotations: map[string]string{"a": "1", "b": "2"},
			crAnnotations: map[string]string{
				ManagedAnnotationsKey: base64.StdEncoding.EncodeToString([]byte(`["a","b"]`)),
			},
			wantAnnotations: map[string]string{"a": "1", "b": "2"},
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

			metadata := &ResourceMetadata{}
			err := GetResourceAnnotations(esc, metadata)
			if (err != nil) != tt.wantErr {
				t.Fatalf("GetResourceAnnotations() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(metadata.Annotations, tt.wantAnnotations) {
				t.Errorf("Annotations = %v, want %v", metadata.Annotations, tt.wantAnnotations)
			}
			if !reflect.DeepEqual(metadata.DeletedAnnotationKeys, tt.wantDeletedKeys) {
				t.Errorf("DeletedAnnotationKeys = %v, want %v", metadata.DeletedAnnotationKeys, tt.wantDeletedKeys)
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
				DeletedAnnotationKeys: tt.deletedKeys,
			}
			needsUpdate, err := AddManagedMetadataAnnotation(esc, metadata)
			if (err != nil) != tt.wantErr {
				t.Fatalf("AddManagedMetadataAnnotation() error = %v, wantErr %v", err, tt.wantErr)
			}
			if needsUpdate != tt.wantNeedsUpdate {
				t.Errorf("needsUpdate = %v, want %v", needsUpdate, tt.wantNeedsUpdate)
			}
			// Verify tracking annotation was set on the CR
			crAnnots := esc.GetAnnotations()
			if crAnnots == nil {
				t.Fatal("expected annotations to be set on CR")
			}
			if _, ok := crAnnots[ManagedAnnotationsKey]; !ok {
				t.Error("expected ManagedAnnotationsKey to be set on CR")
			}
		})
	}
}

func TestGetResourceLabels(t *testing.T) {
	log := testr.New(t)
	defaultLabels := map[string]string{
		"app":                          "external-secrets",
		"app.kubernetes.io/managed-by": "external-secrets-operator",
	}

	tests := []struct {
		name       string
		esmLabels  map[string]string
		escLabels  map[string]string
		esm        *operatorv1alpha1.ExternalSecretsManager
		wantLabels map[string]string
	}{
		{
			name:      "only default labels when no custom labels",
			esm:       nil,
			escLabels: nil,
			wantLabels: map[string]string{
				"app":                          "external-secrets",
				"app.kubernetes.io/managed-by": "external-secrets-operator",
			},
		},
		{
			name:      "ESC labels merged with defaults",
			esm:       nil,
			escLabels: map[string]string{"team": "platform"},
			wantLabels: map[string]string{
				"app":                          "external-secrets",
				"app.kubernetes.io/managed-by": "external-secrets-operator",
				"team":                         "platform",
			},
		},
		{
			name: "ESM with non-empty spec - ESM labels not applied per original logic",
			esm: &operatorv1alpha1.ExternalSecretsManager{
				Spec: operatorv1alpha1.ExternalSecretsManagerSpec{
					GlobalConfig: &operatorv1alpha1.GlobalConfig{
						Labels: map[string]string{"team": "esm-team", "env": "prod"},
					},
				},
			},
			escLabels: map[string]string{"team": "esc-team"},
			wantLabels: map[string]string{
				"app":                          "external-secrets",
				"app.kubernetes.io/managed-by": "external-secrets-operator",
				"team":                         "esc-team",
			},
		},
		{
			name:      "disallowed labels are skipped",
			esm:       nil,
			escLabels: map[string]string{"external-secrets.io/foo": "bar", "valid": "ok"},
			wantLabels: map[string]string{
				"app":                          "external-secrets",
				"app.kubernetes.io/managed-by": "external-secrets-operator",
				"valid":                        "ok",
			},
		},
		{
			name: "default labels override custom labels with same key",
			esm:  nil,
			escLabels: map[string]string{
				"app": "custom-app",
			},
			wantLabels: map[string]string{
				"app":                          "external-secrets",
				"app.kubernetes.io/managed-by": "external-secrets-operator",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			esc := &operatorv1alpha1.ExternalSecretsConfig{
				ObjectMeta: v1.ObjectMeta{Name: "test"},
			}
			esc.Spec.ControllerConfig.Labels = tt.escLabels

			metadata := &ResourceMetadata{}
			GetResourceLabels(log, tt.esm, esc, metadata, defaultLabels)

			if !reflect.DeepEqual(metadata.Labels, tt.wantLabels) {
				t.Errorf("Labels = %v, want %v", metadata.Labels, tt.wantLabels)
			}
		})
	}
}

func TestGetResourceMetadata(t *testing.T) {
	log := testr.New(t)
	defaultLabels := map[string]string{
		"app": "external-secrets",
	}

	tests := []struct {
		name            string
		escLabels       map[string]string
		specAnnotations map[string]string
		crAnnotations   map[string]string
		wantLabels      map[string]string
		wantAnnotations map[string]string
		wantDeletedKeys []string
		wantErr         bool
	}{
		{
			name:            "labels and annotations combined",
			escLabels:       map[string]string{"team": "platform"},
			specAnnotations: map[string]string{"note": "hello"},
			wantLabels:      map[string]string{"app": "external-secrets", "team": "platform"},
			wantAnnotations: map[string]string{"note": "hello"},
		},
		{
			name:            "no custom labels or annotations",
			wantLabels:      map[string]string{"app": "external-secrets"},
			wantAnnotations: map[string]string{},
		},
		{
			name:            "deleted annotation keys detected",
			specAnnotations: map[string]string{"keep": "val"},
			crAnnotations: map[string]string{
				ManagedAnnotationsKey: base64.StdEncoding.EncodeToString([]byte(`["keep","removed"]`)),
			},
			wantLabels:      map[string]string{"app": "external-secrets"},
			wantAnnotations: map[string]string{"keep": "val"},
			wantDeletedKeys: []string{"removed"},
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
			esc.Spec.ControllerConfig.Labels = tt.escLabels
			esc.Spec.ControllerConfig.Annotations = tt.specAnnotations

			metadata, err := GetResourceMetadata(log, nil, esc, defaultLabels)
			if (err != nil) != tt.wantErr {
				t.Fatalf("GetResourceMetadata() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(metadata.Labels, tt.wantLabels) {
				t.Errorf("Labels = %v, want %v", metadata.Labels, tt.wantLabels)
			}
			if !reflect.DeepEqual(metadata.Annotations, tt.wantAnnotations) {
				t.Errorf("Annotations = %v, want %v", metadata.Annotations, tt.wantAnnotations)
			}
			if !reflect.DeepEqual(metadata.DeletedAnnotationKeys, tt.wantDeletedKeys) {
				t.Errorf("DeletedAnnotationKeys = %v, want %v", metadata.DeletedAnnotationKeys, tt.wantDeletedKeys)
			}
		})
	}
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
}
