package external_secrets

import (
	"context"
	"encoding/base64"
	"maps"
	"reflect"
	"testing"

	operatorv1alpha1 "github.com/openshift/external-secrets-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/openshift/external-secrets-operator/pkg/controller/client/fakes"
	"github.com/openshift/external-secrets-operator/pkg/controller/common"
	"github.com/openshift/external-secrets-operator/pkg/controller/commontest"
)

func getTestLabels(extraLabels map[string]string) map[string]string {
	labels := map[string]string{
		"app":                          "external-secrets",
		"app.kubernetes.io/managed-by": "external-secrets-operator",
		"app.kubernetes.io/part-of":    "external-secrets-operator",
		"app.kubernetes.io/version":    "",
	}
	maps.Copy(labels, extraLabels)
	return labels
}

func TestCreateOrApplyNamespace(t *testing.T) {
	tests := []struct {
		name             string
		resourceMetadata common.ResourceMetadata
		preReq           func(*Reconciler, *fakes.FakeCtrlClient)
		wantErr          string
		wantCreate       bool
		wantUpdate       bool
	}{
		{
			name:             "namespace created successfully when it does not exist",
			resourceMetadata: testResourceMetadata(commontest.TestExternalSecretsConfig()),
			preReq: func(r *Reconciler, m *fakes.FakeCtrlClient) {
				m.ExistsCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) (bool, error) {
					return false, nil
				})
				m.CreateCalls(func(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
					ns, ok := obj.(*corev1.Namespace)
					if !ok {
						t.Errorf("expected Namespace object, got %T", obj)
					}
					if ns.Name != commontest.TestExternalSecretsNamespace {
						t.Errorf("expected namespace %s, got %s", commontest.TestExternalSecretsNamespace, ns.Name)
					}
					// Verify labels are set on creation
					for k, v := range controllerDefaultResourceLabels {
						if ns.Labels[k] != v {
							t.Errorf("expected label %s=%s, got %s", k, v, ns.Labels[k])
						}
					}
					return nil
				})
			},
			wantCreate: true,
		},
		{
			name:             "namespace exists with same labels, no update needed",
			resourceMetadata: testResourceMetadata(commontest.TestExternalSecretsConfig()),
			preReq: func(r *Reconciler, m *fakes.FakeCtrlClient) {
				m.ExistsCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) (bool, error) {
					if namespace, ok := obj.(*corev1.Namespace); ok {
						existing := &corev1.Namespace{
							ObjectMeta: metav1.ObjectMeta{
								Name:   commontest.TestExternalSecretsNamespace,
								Labels: controllerDefaultResourceLabels,
							},
						}
						existing.DeepCopyInto(namespace)
						return true, nil
					}
					return false, nil
				})
			},
			wantUpdate: false,
		},
		{
			name:             "namespace exists with different labels, update triggered",
			resourceMetadata: testResourceMetadata(commontest.TestExternalSecretsConfig()),
			preReq: func(r *Reconciler, m *fakes.FakeCtrlClient) {
				m.ExistsCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) (bool, error) {
					if namespace, ok := obj.(*corev1.Namespace); ok {
						// Existing namespace has no labels
						existing := &corev1.Namespace{
							ObjectMeta: metav1.ObjectMeta{
								Name:   commontest.TestExternalSecretsNamespace,
								Labels: nil,
							},
						}
						existing.DeepCopyInto(namespace)
						return true, nil
					}
					return false, nil
				})
				m.UpdateWithRetryCalls(func(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
					ns, ok := obj.(*corev1.Namespace)
					if !ok {
						t.Errorf("expected Namespace object, got %T", obj)
					}
					// Verify labels are set on update
					for k, v := range controllerDefaultResourceLabels {
						if ns.Labels[k] != v {
							t.Errorf("expected label %s=%s, got %s", k, v, ns.Labels[k])
						}
					}
					return nil
				})
			},
			wantUpdate: true,
		},
		{
			name: "namespace exists with different annotations, update triggered and obsolete annotations removed",
			resourceMetadata: common.ResourceMetadata{
				Labels:                controllerDefaultResourceLabels,
				Annotations:           map[string]string{"example.com/team": "platform"},
				DeletedAnnotationKeys: []string{"obsolete-key"},
			},
			preReq: func(r *Reconciler, m *fakes.FakeCtrlClient) {
				m.ExistsCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) (bool, error) {
					if namespace, ok := obj.(*corev1.Namespace); ok {
						existing := &corev1.Namespace{
							ObjectMeta: metav1.ObjectMeta{
								Name:   commontest.TestExternalSecretsNamespace,
								Labels: controllerDefaultResourceLabels,
								Annotations: map[string]string{
									"obsolete-key": "to-be-removed",
									"other":        "preserved",
								},
							},
						}
						existing.DeepCopyInto(namespace)
						return true, nil
					}
					return false, nil
				})
				m.UpdateWithRetryCalls(func(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
					ns, ok := obj.(*corev1.Namespace)
					if !ok {
						t.Errorf("expected Namespace object, got %T", obj)
					}
					// Desired annotation applied
					if ns.Annotations["example.com/team"] != "platform" {
						t.Errorf("expected annotation example.com/team=platform, got %q", ns.Annotations["example.com/team"])
					}
					// Obsolete key removed by RemoveObsoleteAnnotations
					if _, has := ns.Annotations["obsolete-key"]; has {
						t.Error("expected obsolete-key to be removed from namespace annotations")
					}
					// Other existing annotation preserved (UpdateResourceAnnotations merges)
					if ns.Annotations["other"] != "preserved" {
						t.Errorf("expected annotation other=preserved, got %q", ns.Annotations["other"])
					}
					return nil
				})
			},
			wantUpdate: true,
		},
		{
			name: "existing labels are preserved while adding new labels",
			resourceMetadata: common.ResourceMetadata{Labels: map[string]string{
				"new-label": "new-value",
			}},
			preReq: func(r *Reconciler, m *fakes.FakeCtrlClient) {
				m.ExistsCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) (bool, error) {
					if namespace, ok := obj.(*corev1.Namespace); ok {
						existing := &corev1.Namespace{
							ObjectMeta: metav1.ObjectMeta{
								Name: commontest.TestExternalSecretsNamespace,
								Labels: map[string]string{
									"existing-label": "existing-value",
								},
							},
						}
						existing.DeepCopyInto(namespace)
						return true, nil
					}
					return false, nil
				})
				m.UpdateWithRetryCalls(func(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
					ns, ok := obj.(*corev1.Namespace)
					if !ok {
						t.Errorf("expected Namespace object, got %T", obj)
					}
					// Verify existing label is preserved
					if ns.Labels["existing-label"] != "existing-value" {
						t.Errorf("expected existing-label=existing-value, got %s", ns.Labels["existing-label"])
					}
					// Verify new label is added
					if ns.Labels["new-label"] != "new-value" {
						t.Errorf("expected new-label=new-value, got %s", ns.Labels["new-label"])
					}
					return nil
				})
			},
			wantUpdate: true,
		},
		{
			name: "resource labels override existing labels with same key",
			resourceMetadata: common.ResourceMetadata{Labels: map[string]string{
				"shared-label": "new-value",
			}},
			preReq: func(r *Reconciler, m *fakes.FakeCtrlClient) {
				m.ExistsCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) (bool, error) {
					if namespace, ok := obj.(*corev1.Namespace); ok {
						existing := &corev1.Namespace{
							ObjectMeta: metav1.ObjectMeta{
								Name: commontest.TestExternalSecretsNamespace,
								Labels: map[string]string{
									"shared-label": "old-value",
								},
							},
						}
						existing.DeepCopyInto(namespace)
						return true, nil
					}
					return false, nil
				})
				m.UpdateWithRetryCalls(func(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
					ns, ok := obj.(*corev1.Namespace)
					if !ok {
						t.Errorf("expected Namespace object, got %T", obj)
					}
					// Verify label is overridden with new value
					if ns.Labels["shared-label"] != "new-value" {
						t.Errorf("expected shared-label=new-value, got %s", ns.Labels["shared-label"])
					}
					return nil
				})
			},
			wantUpdate: true,
		},
		{
			name:             "exists check fails",
			resourceMetadata: testResourceMetadata(commontest.TestExternalSecretsConfig()),
			preReq: func(r *Reconciler, m *fakes.FakeCtrlClient) {
				m.ExistsCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) (bool, error) {
					return false, commontest.ErrTestClient
				})
			},
			wantErr: "failed to check if namespace external-secrets exists: test client error",
		},
		{
			name:             "create fails",
			resourceMetadata: testResourceMetadata(commontest.TestExternalSecretsConfig()),
			preReq: func(r *Reconciler, m *fakes.FakeCtrlClient) {
				m.ExistsCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) (bool, error) {
					return false, nil
				})
				m.CreateCalls(func(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
					return commontest.ErrTestClient
				})
			},
			wantErr: "failed to create namespace external-secrets: test client error",
		},
		{
			name:             "update fails",
			resourceMetadata: testResourceMetadata(commontest.TestExternalSecretsConfig()),
			preReq: func(r *Reconciler, m *fakes.FakeCtrlClient) {
				m.ExistsCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) (bool, error) {
					if namespace, ok := obj.(*corev1.Namespace); ok {
						existing := &corev1.Namespace{
							ObjectMeta: metav1.ObjectMeta{
								Name:   commontest.TestExternalSecretsNamespace,
								Labels: nil, // No labels, triggers update
							},
						}
						existing.DeepCopyInto(namespace)
						return true, nil
					}
					return false, nil
				})
				m.UpdateWithRetryCalls(func(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
					return commontest.ErrTestClient
				})
			},
			wantErr: "failed to update namespace external-secrets: test client error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := testReconciler(t)
			mock := &fakes.FakeCtrlClient{}
			if tt.preReq != nil {
				tt.preReq(r, mock)
			}
			r.CtrlClient = mock

			esc := commontest.TestExternalSecretsConfig()
			err := r.createOrApplyNamespace(esc, tt.resourceMetadata)

			if tt.wantErr != "" {
				if err == nil || err.Error() != tt.wantErr {
					t.Errorf("createOrApplyNamespace() err: %v, wantErr: %v", err, tt.wantErr)
				}
			} else if err != nil {
				t.Errorf("createOrApplyNamespace() unexpected error: %v", err)
			}

			if tt.wantCreate && mock.CreateCallCount() == 0 {
				t.Error("expected Create to be called, but it wasn't")
			}
			if tt.wantUpdate && mock.UpdateWithRetryCallCount() == 0 {
				t.Error("expected UpdateWithRetry to be called, but it wasn't")
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
				common.ManagedAnnotationsKey: base64.StdEncoding.EncodeToString([]byte(`["foo","old-key"]`)),
			},
			wantAnnotations: map[string]string{"foo": "bar"},
			wantDeletedKeys: []string{"old-key"},
		},
		{
			name:            "no deletions when all previous keys still present",
			specAnnotations: map[string]string{"a": "1", "b": "2"},
			crAnnotations: map[string]string{
				common.ManagedAnnotationsKey: base64.StdEncoding.EncodeToString([]byte(`["a","b"]`)),
			},
			wantAnnotations: map[string]string{"a": "1", "b": "2"},
		},
		{
			name:            "invalid tracking annotation value returns error",
			specAnnotations: map[string]string{"foo": "bar"},
			crAnnotations: map[string]string{
				common.ManagedAnnotationsKey: "!!!",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			esc := &operatorv1alpha1.ExternalSecretsConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test",
					Annotations: tt.crAnnotations,
				},
			}
			esc.Spec.ControllerConfig.Annotations = tt.specAnnotations

			r := testReconciler(t)
			metadata := &common.ResourceMetadata{}
			err := r.getResourceAnnotations(esc, metadata)
			if (err != nil) != tt.wantErr {
				t.Fatalf("GetResourceAnnotations() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr {
				if !reflect.DeepEqual(metadata.Annotations, tt.wantAnnotations) {
					t.Errorf("Annotations = %v, want %v", metadata.Annotations, tt.wantAnnotations)
				}
				if !reflect.DeepEqual(metadata.DeletedAnnotationKeys, tt.wantDeletedKeys) {
					t.Errorf("DeletedAnnotationKeys = %v, want %v", metadata.DeletedAnnotationKeys, tt.wantDeletedKeys)
				}
			}
		})
	}
}

func TestGetResourceLabels(t *testing.T) {
	tests := []struct {
		name       string
		esmLabels  map[string]string
		escLabels  map[string]string
		esm        *operatorv1alpha1.ExternalSecretsManager
		wantLabels map[string]string
	}{
		{
			name:       "only default labels when no custom labels",
			esm:        nil,
			escLabels:  nil,
			wantLabels: getTestLabels(nil),
		},
		{
			name:      "ESC labels merged with defaults",
			esm:       nil,
			escLabels: map[string]string{"team": "platform"},
			wantLabels: getTestLabels(map[string]string{
				"team": "platform",
			}),
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
			wantLabels: getTestLabels(map[string]string{
				"team": "esc-team",
				"env":  "prod",
			}),
		},
		{
			name:      "disallowed labels are skipped",
			esm:       nil,
			escLabels: map[string]string{"external-secrets.io/foo": "bar", "valid": "ok"},
			wantLabels: getTestLabels(map[string]string{
				"valid": "ok",
			}),
		},
		{
			name: "default labels override custom labels with same key",
			esm:  nil,
			escLabels: map[string]string{
				"app": "custom-app",
			},
			wantLabels: getTestLabels(nil),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			esc := &operatorv1alpha1.ExternalSecretsConfig{
				ObjectMeta: metav1.ObjectMeta{Name: "test"},
			}
			esc.Spec.ControllerConfig.Labels = tt.escLabels

			r := testReconciler(t)
			r.esm = tt.esm
			metadata := &common.ResourceMetadata{}
			r.getResourceLabels(esc, metadata)

			if !reflect.DeepEqual(metadata.Labels, tt.wantLabels) {
				t.Errorf("Labels = %v, want %v", metadata.Labels, tt.wantLabels)
			}
		})
	}
}

func TestGetResourceMetadata(t *testing.T) {
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
			wantLabels: getTestLabels(map[string]string{
				"team": "platform",
			}),
			wantAnnotations: map[string]string{"note": "hello"},
		},
		{
			name:            "no custom labels or annotations",
			wantLabels:      getTestLabels(nil),
			wantAnnotations: map[string]string{},
		},
		{
			name:            "deleted annotation keys detected",
			specAnnotations: map[string]string{"keep": "val"},
			crAnnotations: map[string]string{
				common.ManagedAnnotationsKey: base64.StdEncoding.EncodeToString([]byte(`["keep","removed"]`)),
			},
			wantLabels:      getTestLabels(nil),
			wantAnnotations: map[string]string{"keep": "val"},
			wantDeletedKeys: []string{"removed"},
		},
		{
			name:            "invalid tracking annotation on CR returns error",
			specAnnotations: map[string]string{"foo": "bar"},
			crAnnotations: map[string]string{
				common.ManagedAnnotationsKey: "!!!",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			esc := &operatorv1alpha1.ExternalSecretsConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test",
					Annotations: tt.crAnnotations,
				},
			}
			esc.Spec.ControllerConfig.Labels = tt.escLabels
			esc.Spec.ControllerConfig.Annotations = tt.specAnnotations

			r := testReconciler(t)
			metadata, err := r.getResourceMetadata(esc)
			if (err != nil) != tt.wantErr {
				t.Fatalf("GetResourceMetadata() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr {
				if !reflect.DeepEqual(metadata.Labels, tt.wantLabels) {
					t.Errorf("Labels = %v, want %v", metadata.Labels, tt.wantLabels)
				}
				if !reflect.DeepEqual(metadata.Annotations, tt.wantAnnotations) {
					t.Errorf("Annotations = %v, want %v", metadata.Annotations, tt.wantAnnotations)
				}
				if !reflect.DeepEqual(metadata.DeletedAnnotationKeys, tt.wantDeletedKeys) {
					t.Errorf("DeletedAnnotationKeys = %v, want %v", metadata.DeletedAnnotationKeys, tt.wantDeletedKeys)
				}
			}
		})
	}
}
