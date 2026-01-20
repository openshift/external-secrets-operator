package external_secrets

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/openshift/external-secrets-operator/pkg/controller/client/fakes"
	"github.com/openshift/external-secrets-operator/pkg/controller/commontest"
)

func TestCreateOrApplyNamespace(t *testing.T) {
	tests := []struct {
		name           string
		resourceLabels map[string]string
		preReq         func(*Reconciler, *fakes.FakeCtrlClient)
		wantErr        string
		wantCreate     bool
		wantUpdate     bool
	}{
		{
			name:           "namespace created successfully when it does not exist",
			resourceLabels: controllerDefaultResourceLabels,
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
			name:           "namespace exists with same labels, no update needed",
			resourceLabels: controllerDefaultResourceLabels,
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
			name:           "namespace exists with different labels, update triggered",
			resourceLabels: controllerDefaultResourceLabels,
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
			name: "existing labels are preserved while adding new labels",
			resourceLabels: map[string]string{
				"new-label": "new-value",
			},
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
			resourceLabels: map[string]string{
				"shared-label": "new-value",
			},
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
			name:           "exists check fails",
			resourceLabels: controllerDefaultResourceLabels,
			preReq: func(r *Reconciler, m *fakes.FakeCtrlClient) {
				m.ExistsCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) (bool, error) {
					return false, commontest.ErrTestClient
				})
			},
			wantErr: "failed to check if namespace external-secrets exists: test client error",
		},
		{
			name:           "create fails",
			resourceLabels: controllerDefaultResourceLabels,
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
			name:           "update fails",
			resourceLabels: controllerDefaultResourceLabels,
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
			err := r.createOrApplyNamespace(esc, tt.resourceLabels)

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
