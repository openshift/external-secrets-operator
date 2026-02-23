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

			SetManagedAnnotations(obj, tt.userAnnotations)

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

func TestSetManagedAnnotationsTracking(t *testing.T) {
	tests := []struct {
		name            string
		userAnnotations map[string]string
		wantKeys        []string
	}{
		{
			name:            "sets tracking annotation with sorted keys",
			userAnnotations: map[string]string{"foo": "bar", "baz": "qux"},
			wantKeys:        []string{"baz", "foo"},
		},
		{
			name:            "empty annotations",
			userAnnotations: map[string]string{},
			wantKeys:        []string{},
		},
		{
			name:            "nil annotations",
			userAnnotations: nil,
			wantKeys:        []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			esc := &operatorv1alpha1.ExternalSecretsConfig{
				ObjectMeta: v1.ObjectMeta{
					Name: "cluster",
				},
			}

			SetManagedAnnotationsTracking(esc, tt.userAnnotations)

			gotKeys := GetManagedAnnotationKeys(esc)
			if !reflect.DeepEqual(gotKeys, tt.wantKeys) {
				t.Errorf("GetManagedAnnotationKeys() = %v, want %v", gotKeys, tt.wantKeys)
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

func TestMergeFetchedAnnotations(t *testing.T) {
	tests := []struct {
		name                string
		metadata            ResourceMetadata
		desiredAnnotations  map[string]string
		fetchedAnnotations  map[string]string
		expectedAnnotations map[string]string
	}{
		{
			name: "preserves external annotations from fetched",
			metadata: ResourceMetadata{
				CurrentlyManagedAnnotKeys:  []string{"user-key"},
				PreviouslyManagedAnnotKeys: []string{"user-key"},
			},
			desiredAnnotations: map[string]string{
				"user-key": "user-value",
			},
			fetchedAnnotations: map[string]string{
				"user-key":                          "old-value",
				"deployment.kubernetes.io/revision": "4",
				"openshift.io/owning-component":     "CNO",
				"cert-manager.io/inject-ca-from":    "ns/cert",
			},
			expectedAnnotations: map[string]string{
				"user-key":                          "user-value",
				"deployment.kubernetes.io/revision": "4",
				"openshift.io/owning-component":     "CNO",
				"cert-manager.io/inject-ca-from":    "ns/cert",
			},
		},
		{
			name: "does not copy previously-managed keys that were removed",
			metadata: ResourceMetadata{
				CurrentlyManagedAnnotKeys:  []string{},
				PreviouslyManagedAnnotKeys: []string{"removed-key"},
			},
			desiredAnnotations: map[string]string{},
			fetchedAnnotations: map[string]string{
				"removed-key":         "old-value",
				"external-annotation": "keep",
			},
			expectedAnnotations: map[string]string{
				"external-annotation": "keep",
			},
		},
		{
			name: "no fetched annotations",
			metadata: ResourceMetadata{
				CurrentlyManagedAnnotKeys: []string{"foo"},
			},
			desiredAnnotations: map[string]string{
				"foo": "bar",
			},
			fetchedAnnotations: nil,
			expectedAnnotations: map[string]string{
				"foo": "bar",
			},
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
			MergeFetchedAnnotations(desired, fetched, &tt.metadata)
			if !reflect.DeepEqual(desired.GetAnnotations(), tt.expectedAnnotations) {
				t.Errorf("after MergeFetchedAnnotations:\ngot:  %v\nwant: %v", desired.GetAnnotations(), tt.expectedAnnotations)
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
				CurrentlyManagedAnnotKeys:  []string{"user-key"},
				PreviouslyManagedAnnotKeys: []string{"user-key"},
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
			name: "unmanaged annotation changed by external actor - returns false",
			metadata: ResourceMetadata{
				CurrentlyManagedAnnotKeys:  []string{"user-key"},
				PreviouslyManagedAnnotKeys: []string{"user-key"},
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
				CurrentlyManagedAnnotKeys:  []string{},
				PreviouslyManagedAnnotKeys: []string{"removed-key"},
			},
			desiredAnnotations: map[string]string{},
			fetchedAnnotations: map[string]string{
				"removed-key": "value",
			},
			want: true,
		},
		{
			name:               "empty managed annotations - no false positives",
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
				CurrentlyManagedAnnotKeys: []string{"new-key"},
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

func TestDeploymentObjectChanged(t *testing.T) {
	t.Run("unmanaged annotation updated by foreign actor should lead to no change", func(t *testing.T) {
		// Deployment with external annotation on fetched should not trigger change
		// when desired has no managed annotations
		x := appsv1.Deployment{
			ObjectMeta: v1.ObjectMeta{
				Annotations: map[string]string{
					"deployment.kubernetes.io/revision": "4",
				},
			},
		}

		y := appsv1.Deployment{
			ObjectMeta: v1.ObjectMeta{
				Annotations: map[string]string{},
			},
		}

		managedMetaState := ResourceMetadata{}
		if HasObjectChanged(&y, &x, &managedMetaState) {
			t.Fatal("expected no change when fetched has only external annotations and desired has none")
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
			CurrentlyManagedAnnotKeys:  []string{"my-annotation"},
			PreviouslyManagedAnnotKeys: []string{"my-annotation"},
		}

		if !HasObjectChanged(&desired, &fetched, &managedMetaState) {
			t.Fatal("expected change when managed annotation value differs")
		}
	})

	t.Run("unmanaged pod annotation updated by foreign actor should lead to no change", func(t *testing.T) {
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
			CurrentlyManagedAnnotKeys:  []string{"user-key"},
			PreviouslyManagedAnnotKeys: []string{"user-key"},
		}

		if HasObjectChanged(&desired, &fetched, &managedMetaState) {
			t.Fatal("expected no change when fetched pod template has only external annotations added")
		}
	})
}
