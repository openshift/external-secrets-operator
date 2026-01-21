package external_secrets

import (
	"context"
	"fmt"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/openshift/external-secrets-operator/api/v1alpha1"
	"github.com/openshift/external-secrets-operator/pkg/controller/client/fakes"
	"github.com/openshift/external-secrets-operator/pkg/controller/commontest"
)

const (
	testValidateSecretResourceName = "external-secrets-webhook"
)

func TestCreateOrApplySecret(t *testing.T) {
	tests := []struct {
		name    string
		preReq  func(*Reconciler, *fakes.FakeCtrlClient)
		esc     func(*v1alpha1.ExternalSecretsConfig)
		wantErr string
	}{
		{
			name:   "external secret spec disabled",
			preReq: nil,
			esc: func(esc *v1alpha1.ExternalSecretsConfig) {
				esc.Spec = v1alpha1.ExternalSecretsConfigSpec{}
			},
		},
		{
			name:   "webhook config is nil",
			preReq: nil,
			esc: func(esc *v1alpha1.ExternalSecretsConfig) {
				esc.Spec = v1alpha1.ExternalSecretsConfigSpec{
					ApplicationConfig: v1alpha1.ApplicationConfig{
						WebhookConfig: nil,
					},
				}
			},
		},
		{
			name:   "webhook config is empty",
			preReq: nil,
			esc: func(esc *v1alpha1.ExternalSecretsConfig) {
				esc.Spec = v1alpha1.ExternalSecretsConfigSpec{
					ApplicationConfig: v1alpha1.ApplicationConfig{
						WebhookConfig: &v1alpha1.WebhookConfig{},
					},
				}
			},
		},
		{
			name:   "cert manager config is nil",
			preReq: nil,
			esc: func(esc *v1alpha1.ExternalSecretsConfig) {
				esc.Spec = v1alpha1.ExternalSecretsConfigSpec{
					ControllerConfig: v1alpha1.ControllerConfig{
						CertProvider: &v1alpha1.CertProvidersConfig{
							CertManager: nil,
						},
					},
				}
			},
		},
		{
			name: "reconciliation of secret fails while checking if exists",
			preReq: func(r *Reconciler, m *fakes.FakeCtrlClient) {
				m.GetCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) error {
					if o, ok := obj.(*corev1.Secret); ok {
						secret := testSecret(webhookTLSSecretAssetName)
						secret.DeepCopyInto(o)
					}
					return nil
				})
				m.ExistsCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) (bool, error) {
					if _, ok := obj.(*corev1.Secret); ok {
						return false, commontest.ErrTestClient
					}
					return true, nil
				})
			},
			wantErr: fmt.Sprintf("failed to check %s/%s secret resource already exists: %s", commontest.TestExternalSecretsNamespace, testValidateSecretResourceName, commontest.ErrTestClient),
		},
		{
			name: "reconciliation of secret fails while restoring to expected state",
			preReq: func(r *Reconciler, m *fakes.FakeCtrlClient) {
				m.GetCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) error {
					if o, ok := obj.(*corev1.Secret); ok {
						secret := testSecret(webhookTLSSecretAssetName)
						secret.DeepCopyInto(o)
					}
					return nil
				})
				m.ExistsCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) (bool, error) {
					if o, ok := obj.(*corev1.Secret); ok {
						secret := testSecret(webhookTLSSecretAssetName)
						secret.SetLabels(map[string]string{"test": "test"})
						secret.DeepCopyInto(o)
					}
					return true, nil
				})
				m.UpdateWithRetryCalls(func(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
					if _, ok := obj.(*corev1.Secret); ok {
						return commontest.ErrTestClient
					}
					return nil
				})
			},
			wantErr: fmt.Sprintf("failed to update %s/%s secret resource: %s", commontest.TestExternalSecretsNamespace, testValidateSecretResourceName, commontest.ErrTestClient),
		},
		{
			name: "reconciliation of secret which already exists in expected state",
			preReq: func(r *Reconciler, m *fakes.FakeCtrlClient) {
				m.GetCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) error {
					if o, ok := obj.(*corev1.Secret); ok {
						secret := testSecret(webhookTLSSecretAssetName)
						secret.DeepCopyInto(o)
					}
					return nil
				})
				m.ExistsCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) (bool, error) {
					if o, ok := obj.(*corev1.Secret); ok {
						secret := testSecret(webhookTLSSecretAssetName)
						secret.DeepCopyInto(o)
					}
					return true, nil
				})
			},
		},
		{
			name: "reconciliation of secret creation fails",
			preReq: func(r *Reconciler, m *fakes.FakeCtrlClient) {
				m.GetCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) error {
					if o, ok := obj.(*corev1.Secret); ok {
						secret := testSecret(webhookTLSSecretAssetName)
						secret.DeepCopyInto(o)
					}
					return nil
				})
				m.ExistsCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) (bool, error) {
					if _, ok := obj.(*corev1.Secret); ok {
						return false, nil
					}
					return true, nil
				})
				m.CreateCalls(func(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
					if _, ok := obj.(*corev1.Secret); ok {
						return commontest.ErrTestClient
					}
					return nil
				})
			},
			wantErr: fmt.Sprintf("failed to create %s/%s secret resource: %s", commontest.TestExternalSecretsNamespace, testValidateSecretResourceName, commontest.ErrTestClient),
		},
		{
			name: "successful secret creation",
			preReq: func(r *Reconciler, m *fakes.FakeCtrlClient) {
				m.ExistsCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) (bool, error) {
					return false, nil
				})

				m.CreateCalls(func(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
					return nil
				})
			},
		},
		{
			name: "secret creation skipped when cert-manager config is enabled",
			preReq: func(r *Reconciler, m *fakes.FakeCtrlClient) {
			},
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
			esc := testExternalSecretsConfigForSecrets()
			if tt.esc != nil {
				tt.esc(esc)
			}

			err := r.createOrApplySecret(esc, controllerDefaultResourceLabels, false)
			if (tt.wantErr != "" || err != nil) && (err == nil || err.Error() != tt.wantErr) {
				t.Errorf("createOrApplySecret() err: %v, wantErr: %v", err, tt.wantErr)
			}
		})
	}
}

func testExternalSecretsConfigForSecrets() *v1alpha1.ExternalSecretsConfig {
	esc := commontest.TestExternalSecretsConfig()

	esc.Spec = v1alpha1.ExternalSecretsConfigSpec{
		ControllerConfig: v1alpha1.ControllerConfig{
			CertProvider: &v1alpha1.CertProvidersConfig{
				CertManager: &v1alpha1.CertManagerConfig{
					Mode: v1alpha1.Disabled,
				},
			},
		},
		ApplicationConfig: v1alpha1.ApplicationConfig{},
	}
	return esc
}
