package common

import (
	"encoding/base64"
	"encoding/json"
	"reflect"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

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

	t.Run("SecurityContext nil on fetched should trigger change", func(t *testing.T) {
		t.Parallel()
		allowPrivEsc := false
		desired := baseDeployment()
		desired.Spec.Template.Spec.Containers[0].SecurityContext = &corev1.SecurityContext{
			AllowPrivilegeEscalation: &allowPrivEsc,
		}
		fetched := baseDeployment()
		fetched.Spec.Template.Spec.Containers[0].SecurityContext = nil

		if !HasObjectChanged(&desired, &fetched, &ResourceMetadata{}) {
			t.Fatal("expected change when desired has SecurityContext but fetched is nil")
		}
	})

	t.Run("SecurityContext both nil should not trigger change", func(t *testing.T) {
		t.Parallel()
		desired := baseDeployment()
		desired.Spec.Template.Spec.Containers[0].SecurityContext = nil
		fetched := baseDeployment()
		fetched.Spec.Template.Spec.Containers[0].SecurityContext = nil

		if HasObjectChanged(&desired, &fetched, &ResourceMetadata{}) {
			t.Fatal("expected no change when both SecurityContexts are nil")
		}
	})

	t.Run("ReadinessProbe difference should trigger change", func(t *testing.T) {
		t.Parallel()
		desired := baseDeployment()
		desired.Spec.Template.Spec.Containers[0].ReadinessProbe = &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{Path: "/readyz", Port: intstr.FromInt32(8080)},
			},
		}
		fetched := baseDeployment()
		fetched.Spec.Template.Spec.Containers[0].ReadinessProbe = &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{Path: "/healthz", Port: intstr.FromInt32(8080)},
			},
		}

		if !HasObjectChanged(&desired, &fetched, &ResourceMetadata{}) {
			t.Fatal("expected change when ReadinessProbe path differs")
		}
	})

	t.Run("LivenessProbe difference should trigger change", func(t *testing.T) {
		t.Parallel()
		desired := baseDeployment()
		desired.Spec.Template.Spec.Containers[0].LivenessProbe = &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{Path: "/livez", Port: intstr.FromInt32(8081)},
			},
		}
		fetched := baseDeployment()
		fetched.Spec.Template.Spec.Containers[0].LivenessProbe = &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{Path: "/healthz", Port: intstr.FromInt32(8081)},
			},
		}

		if !HasObjectChanged(&desired, &fetched, &ResourceMetadata{}) {
			t.Fatal("expected change when LivenessProbe path differs")
		}
	})

	t.Run("LivenessProbe both nil should not trigger change", func(t *testing.T) {
		t.Parallel()
		desired := baseDeployment()
		desired.Spec.Template.Spec.Containers[0].LivenessProbe = nil
		fetched := baseDeployment()
		fetched.Spec.Template.Spec.Containers[0].LivenessProbe = nil

		if HasObjectChanged(&desired, &fetched, &ResourceMetadata{}) {
			t.Fatal("expected no change when both LivenessProbes are nil")
		}
	})

	t.Run("Port protocol difference should trigger change even when ContainerPort matches", func(t *testing.T) {
		t.Parallel()
		desired := baseDeployment()
		desired.Spec.Template.Spec.Containers[0].Ports = []corev1.ContainerPort{
			{Name: "metrics", ContainerPort: 8080, Protocol: corev1.ProtocolTCP},
		}
		fetched := baseDeployment()
		fetched.Spec.Template.Spec.Containers[0].Ports = []corev1.ContainerPort{
			{Name: "metrics", ContainerPort: 8080, Protocol: corev1.ProtocolUDP},
		}

		if !HasObjectChanged(&desired, &fetched, &ResourceMetadata{}) {
			t.Fatal("expected change when port protocol differs")
		}
	})

	t.Run("Port name difference should trigger change even when ContainerPort matches", func(t *testing.T) {
		t.Parallel()
		desired := baseDeployment()
		desired.Spec.Template.Spec.Containers[0].Ports = []corev1.ContainerPort{
			{Name: "metrics", ContainerPort: 8080, Protocol: corev1.ProtocolTCP},
		}
		fetched := baseDeployment()
		fetched.Spec.Template.Spec.Containers[0].Ports = []corev1.ContainerPort{
			{Name: "http", ContainerPort: 8080, Protocol: corev1.ProtocolTCP},
		}

		if !HasObjectChanged(&desired, &fetched, &ResourceMetadata{}) {
			t.Fatal("expected change when port name differs")
		}
	})

	t.Run("Identical ports should not trigger change", func(t *testing.T) {
		t.Parallel()
		desired := baseDeployment()
		desired.Spec.Template.Spec.Containers[0].Ports = []corev1.ContainerPort{
			{Name: "metrics", ContainerPort: 8080, Protocol: corev1.ProtocolTCP},
		}
		fetched := baseDeployment()
		fetched.Spec.Template.Spec.Containers[0].Ports = []corev1.ContainerPort{
			{Name: "metrics", ContainerPort: 8080, Protocol: corev1.ProtocolTCP},
		}

		if HasObjectChanged(&desired, &fetched, &ResourceMetadata{}) {
			t.Fatal("expected no change when ports are identical")
		}
	})

	t.Run("Port with empty protocol should not trigger change against the API server's TCP default", func(t *testing.T) {
		t.Parallel()
		desired := baseDeployment()
		desired.Spec.Template.Spec.Containers[0].Ports = []corev1.ContainerPort{
			{Name: "metrics", ContainerPort: 8080}, // Protocol omitted, as an in-memory object would
		}
		fetched := baseDeployment()
		fetched.Spec.Template.Spec.Containers[0].Ports = []corev1.ContainerPort{
			{Name: "metrics", ContainerPort: 8080, Protocol: corev1.ProtocolTCP}, // API-server-defaulted
		}

		if HasObjectChanged(&desired, &fetched, &ResourceMetadata{}) {
			t.Fatal("expected no change between an omitted protocol and the API server's TCP default")
		}
	})

	t.Run("Resources with equal-but-differently-represented quantities should not trigger change", func(t *testing.T) {
		t.Parallel()
		desired := baseDeployment()
		desired.Spec.Template.Spec.Containers[0].Resources = corev1.ResourceRequirements{
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("1000m"), // literal form, string cache unset
				corev1.ResourceMemory: resource.MustParse("1Gi"),
			},
		}
		fetched := baseDeployment()
		fetched.Spec.Template.Spec.Containers[0].Resources = roundTripThroughJSON(t, desired.Spec.Template.Spec.Containers[0].Resources)

		// Sanity-check that this reproduces the exact pre-fix condition: same value, same JSON,
		// but NOT reflect.DeepEqual.
		if reflect.DeepEqual(desired.Spec.Template.Spec.Containers[0].Resources, fetched.Spec.Template.Spec.Containers[0].Resources) {
			t.Fatal("test setup invalid: expected the round-tripped quantity to differ from reflect.DeepEqual, reproducing the bug precondition")
		}

		if HasObjectChanged(&desired, &fetched, &ResourceMetadata{}) {
			t.Fatal("expected no change when Resources quantities are equal but have differing internal representations")
		}
	})

	t.Run("Resources value difference should trigger change", func(t *testing.T) {
		t.Parallel()
		desired := baseDeployment()
		desired.Spec.Template.Spec.Containers[0].Resources = corev1.ResourceRequirements{
			Limits: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("1000m")},
		}
		fetched := baseDeployment()
		fetched.Spec.Template.Spec.Containers[0].Resources = corev1.ResourceRequirements{
			Limits: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("2000m")},
		}

		if !HasObjectChanged(&desired, &fetched, &ResourceMetadata{}) {
			t.Fatal("expected change when a Resources quantity genuinely differs")
		}
	})

	t.Run("Resources limits-only desired vs limits+requests fetched should trigger change", func(t *testing.T) {
		t.Parallel()
		desired := baseDeployment()
		desired.Spec.Template.Spec.Containers[0].Resources = corev1.ResourceRequirements{
			Limits: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("1000m")},
		}
		fetched := baseDeployment()
		fetched.Spec.Template.Spec.Containers[0].Resources = corev1.ResourceRequirements{
			Limits:   corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("1000m")},
			Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("250m")},
		}

		if !HasObjectChanged(&desired, &fetched, &ResourceMetadata{}) {
			t.Fatal("expected change when fetched has a Requests entry that desired does not")
		}
	})

	t.Run("Resources both empty should not trigger change", func(t *testing.T) {
		t.Parallel()
		desired := baseDeployment()
		fetched := baseDeployment()

		if HasObjectChanged(&desired, &fetched, &ResourceMetadata{}) {
			t.Fatal("expected no change when neither has Resources set")
		}
	})

	// InitContainers is checked by the same containersModified helper as Containers; these pin
	// down that the shared helper behaves identically for both PodSpec fields.
	t.Run("InitContainers image difference should trigger change", func(t *testing.T) {
		t.Parallel()
		desired := baseDeployment()
		desired.Spec.Template.Spec.InitContainers = []corev1.Container{
			{Name: "init", Image: "registry.example.com/init:v1"},
		}
		fetched := baseDeployment()
		fetched.Spec.Template.Spec.InitContainers = []corev1.Container{
			{Name: "init", Image: "registry.example.com/init:v2"},
		}

		if !HasObjectChanged(&desired, &fetched, &ResourceMetadata{}) {
			t.Fatal("expected change when an InitContainer image differs")
		}
	})

	t.Run("InitContainers missing from fetched should trigger change", func(t *testing.T) {
		t.Parallel()
		desired := baseDeployment()
		desired.Spec.Template.Spec.InitContainers = []corev1.Container{
			{Name: "init", Image: "registry.example.com/init:v1"},
		}
		fetched := baseDeployment()

		if !HasObjectChanged(&desired, &fetched, &ResourceMetadata{}) {
			t.Fatal("expected change when a desired InitContainer is missing from fetched")
		}
	})

	t.Run("Identical InitContainers should not trigger change", func(t *testing.T) {
		t.Parallel()
		desired := baseDeployment()
		desired.Spec.Template.Spec.InitContainers = []corev1.Container{
			{Name: "init", Image: "registry.example.com/init:v1"},
		}
		fetched := baseDeployment()
		fetched.Spec.Template.Spec.InitContainers = []corev1.Container{
			{Name: "init", Image: "registry.example.com/init:v1"},
		}

		if HasObjectChanged(&desired, &fetched, &ResourceMetadata{}) {
			t.Fatal("expected no change when InitContainers are identical")
		}
	})

	t.Run("TopologySpreadConstraints difference should trigger change", func(t *testing.T) {
		t.Parallel()
		desired := baseDeployment()
		desired.Spec.Template.Spec.TopologySpreadConstraints = []corev1.TopologySpreadConstraint{
			{MaxSkew: 1, TopologyKey: "topology.kubernetes.io/zone", WhenUnsatisfiable: corev1.DoNotSchedule},
		}
		fetched := baseDeployment()
		fetched.Spec.Template.Spec.TopologySpreadConstraints = nil

		if !HasObjectChanged(&desired, &fetched, &ResourceMetadata{}) {
			t.Fatal("expected change when desired has TopologySpreadConstraints but fetched does not")
		}
	})

	t.Run("TopologySpreadConstraints removed externally should trigger change", func(t *testing.T) {
		t.Parallel()
		constraints := []corev1.TopologySpreadConstraint{
			{MaxSkew: 1, TopologyKey: "topology.kubernetes.io/zone", WhenUnsatisfiable: corev1.DoNotSchedule},
		}
		desired := baseDeployment()
		desired.Spec.Template.Spec.TopologySpreadConstraints = constraints
		fetched := baseDeployment()
		fetched.Spec.Template.Spec.TopologySpreadConstraints = []corev1.TopologySpreadConstraint{
			{MaxSkew: 1, TopologyKey: "topology.kubernetes.io/hostname", WhenUnsatisfiable: corev1.ScheduleAnyway},
		}

		if !HasObjectChanged(&desired, &fetched, &ResourceMetadata{}) {
			t.Fatal("expected change when TopologySpreadConstraints differ")
		}
	})

	t.Run("TopologySpreadConstraints absent on both should not trigger change", func(t *testing.T) {
		t.Parallel()
		desired := baseDeployment()
		fetched := baseDeployment()

		if HasObjectChanged(&desired, &fetched, &ResourceMetadata{}) {
			t.Fatal("expected no change when neither has TopologySpreadConstraints")
		}
	})

	// Regression tests for the asymmetric nil-guard bug: NodeSelector/Tolerations/
	// TopologySpreadConstraints are all settable via advancedOverrides, so desired going from
	// set to nil (the user removed their override) must be detected as a change, not skipped -
	// and the reverse (desired newly set, fetched still nil) must be detected too. A one-sided
	// "if desired != nil" guard misses the first case; a one-sided "if fetched != nil" guard
	// misses the second. Covering both directions, plus nil-vs-empty equivalence, for all four
	// fields pins down that neither asymmetry regresses.
	topologySpreadConstraints := []corev1.TopologySpreadConstraint{
		{MaxSkew: 1, TopologyKey: "topology.kubernetes.io/zone", WhenUnsatisfiable: corev1.DoNotSchedule},
	}
	tolerations := []corev1.Toleration{
		{Key: "dedicated", Operator: corev1.TolerationOpEqual, Value: "eso", Effect: corev1.TaintEffectNoSchedule},
	}
	nodeSelector := map[string]string{"disktype": "ssd"}
	affinity := &corev1.Affinity{
		NodeAffinity: &corev1.NodeAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
				NodeSelectorTerms: []corev1.NodeSelectorTerm{{
					MatchExpressions: []corev1.NodeSelectorRequirement{
						{Key: "kubernetes.io/arch", Operator: corev1.NodeSelectorOpIn, Values: []string{"amd64"}},
					},
				}},
			},
		},
	}

	t.Run("TopologySpreadConstraints removal (desired nil, fetched set) should trigger change", func(t *testing.T) {
		t.Parallel()
		desired := baseDeployment()
		fetched := baseDeployment()
		fetched.Spec.Template.Spec.TopologySpreadConstraints = topologySpreadConstraints

		if !HasObjectChanged(&desired, &fetched, &ResourceMetadata{}) {
			t.Fatal("expected change when a previously-set TopologySpreadConstraints override is removed")
		}
	})

	t.Run("TopologySpreadConstraints addition (desired set, fetched nil) should trigger change", func(t *testing.T) {
		t.Parallel()
		desired := baseDeployment()
		desired.Spec.Template.Spec.TopologySpreadConstraints = topologySpreadConstraints
		fetched := baseDeployment()

		if !HasObjectChanged(&desired, &fetched, &ResourceMetadata{}) {
			t.Fatal("expected change when a new TopologySpreadConstraints override is added")
		}
	})

	t.Run("TopologySpreadConstraints nil vs empty slice should not trigger change", func(t *testing.T) {
		t.Parallel()
		desired := baseDeployment()
		desired.Spec.Template.Spec.TopologySpreadConstraints = nil
		fetched := baseDeployment()
		fetched.Spec.Template.Spec.TopologySpreadConstraints = []corev1.TopologySpreadConstraint{}

		if HasObjectChanged(&desired, &fetched, &ResourceMetadata{}) {
			t.Fatal("expected no change between a nil and an empty TopologySpreadConstraints slice")
		}
	})

	t.Run("Tolerations removal (desired nil, fetched set) should trigger change", func(t *testing.T) {
		t.Parallel()
		desired := baseDeployment()
		fetched := baseDeployment()
		fetched.Spec.Template.Spec.Tolerations = tolerations

		if !HasObjectChanged(&desired, &fetched, &ResourceMetadata{}) {
			t.Fatal("expected change when a previously-set Tolerations override is removed")
		}
	})

	t.Run("Tolerations addition (desired set, fetched nil) should trigger change", func(t *testing.T) {
		t.Parallel()
		desired := baseDeployment()
		desired.Spec.Template.Spec.Tolerations = tolerations
		fetched := baseDeployment()

		if !HasObjectChanged(&desired, &fetched, &ResourceMetadata{}) {
			t.Fatal("expected change when a new Tolerations override is added")
		}
	})

	t.Run("Tolerations nil vs empty slice should not trigger change", func(t *testing.T) {
		t.Parallel()
		desired := baseDeployment()
		desired.Spec.Template.Spec.Tolerations = nil
		fetched := baseDeployment()
		fetched.Spec.Template.Spec.Tolerations = []corev1.Toleration{}

		if HasObjectChanged(&desired, &fetched, &ResourceMetadata{}) {
			t.Fatal("expected no change between a nil and an empty Tolerations slice")
		}
	})

	t.Run("NodeSelector removal (desired nil, fetched set) should trigger change", func(t *testing.T) {
		t.Parallel()
		desired := baseDeployment()
		fetched := baseDeployment()
		fetched.Spec.Template.Spec.NodeSelector = nodeSelector

		if !HasObjectChanged(&desired, &fetched, &ResourceMetadata{}) {
			t.Fatal("expected change when a previously-set NodeSelector override is removed")
		}
	})

	t.Run("NodeSelector addition (desired set, fetched nil) should trigger change", func(t *testing.T) {
		t.Parallel()
		desired := baseDeployment()
		desired.Spec.Template.Spec.NodeSelector = nodeSelector
		fetched := baseDeployment()

		if !HasObjectChanged(&desired, &fetched, &ResourceMetadata{}) {
			t.Fatal("expected change when a new NodeSelector override is added")
		}
	})

	t.Run("NodeSelector nil vs empty map should not trigger change", func(t *testing.T) {
		t.Parallel()
		desired := baseDeployment()
		desired.Spec.Template.Spec.NodeSelector = nil
		fetched := baseDeployment()
		fetched.Spec.Template.Spec.NodeSelector = map[string]string{}

		if HasObjectChanged(&desired, &fetched, &ResourceMetadata{}) {
			t.Fatal("expected no change between a nil and an empty NodeSelector map")
		}
	})

	t.Run("Template.Labels removal (desired nil, fetched set) should trigger change", func(t *testing.T) {
		t.Parallel()
		desired := baseDeployment()
		desired.Spec.Template.Labels = nil
		fetched := baseDeployment()
		fetched.Spec.Template.Labels = map[string]string{"app.kubernetes.io/name": "external-secrets"}

		if !HasObjectChanged(&desired, &fetched, &ResourceMetadata{}) {
			t.Fatal("expected change when a previously-set Template.Labels entry is removed")
		}
	})

	t.Run("Template.Labels addition (desired set, fetched nil) should trigger change", func(t *testing.T) {
		t.Parallel()
		desired := baseDeployment()
		desired.Spec.Template.Labels = map[string]string{"app.kubernetes.io/name": "external-secrets"}
		fetched := baseDeployment()
		fetched.Spec.Template.Labels = nil

		if !HasObjectChanged(&desired, &fetched, &ResourceMetadata{}) {
			t.Fatal("expected change when a new Template.Labels entry is added")
		}
	})

	t.Run("Template.Labels nil vs empty map should not trigger change", func(t *testing.T) {
		t.Parallel()
		desired := baseDeployment()
		desired.Spec.Template.Labels = nil
		fetched := baseDeployment()
		fetched.Spec.Template.Labels = map[string]string{}

		if HasObjectChanged(&desired, &fetched, &ResourceMetadata{}) {
			t.Fatal("expected no change between a nil and an empty Template.Labels map")
		}
	})

	t.Run("Affinity removal (desired nil, fetched set) should trigger change", func(t *testing.T) {
		t.Parallel()
		desired := baseDeployment()
		fetched := baseDeployment()
		fetched.Spec.Template.Spec.Affinity = affinity

		if !HasObjectChanged(&desired, &fetched, &ResourceMetadata{}) {
			t.Fatal("expected change when a previously-set Affinity override is removed")
		}
	})

	t.Run("Affinity addition (desired set, fetched nil) should trigger change", func(t *testing.T) {
		t.Parallel()
		desired := baseDeployment()
		desired.Spec.Template.Spec.Affinity = affinity
		fetched := baseDeployment()

		if !HasObjectChanged(&desired, &fetched, &ResourceMetadata{}) {
			t.Fatal("expected change when a new Affinity override is added")
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

// roundTripThroughJSON marshals v to JSON and unmarshals it back into a new T, simulating what
// happens to a resource.Quantity when it travels through a client Update() to the API server and
// is later re-fetched via Get(): the canonicalized string form gets re-parsed, which can leave
// the result's internal (value, scale, cached-string) representation different from the
// original even though both represent the same quantity.
func roundTripThroughJSON[T any](t *testing.T, v T) T {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("roundTripThroughJSON: marshal: %v", err)
	}
	var out T
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("roundTripThroughJSON: unmarshal: %v", err)
	}
	return out
}

func baseDeployment() appsv1.Deployment {
	return appsv1.Deployment{
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "external-secrets",
							Image: "ghcr.io/external-secrets/external-secrets:v2.12.0",
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
