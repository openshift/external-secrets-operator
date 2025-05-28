package controller

import (
	"context"
	"fmt"
	"testing"

	webhook "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/openshift/external-secrets-operator/api/v1alpha1"
	"github.com/openshift/external-secrets-operator/pkg/controller/fakes"
)

var (
	testValidateWebhookConfigurationResourceName = "externalsecret-validate"
)

func TestCreateOrApplyValidatingWebhookConfiguration(t *testing.T) {
	tests := []struct {
		name    string
		preReq  func(*ExternalSecretsReconciler, *fakes.FakeCtrlClient)
		wantErr string
	}{
		{
			name: "validatingWebhookConfiguration reconciliation successful",
			preReq: func(r *ExternalSecretsReconciler, m *fakes.FakeCtrlClient) {
				m.ExistsCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) (bool, error) {
					switch o := obj.(type) {
					case *webhook.ValidatingWebhookConfiguration:
						webhookConfig := testValidatingWebhookConfiguration(validatingWebhookExternalSecretCRDAssetName)
						webhookConfig.DeepCopyInto(o)
					}
					return true, nil
				})
			},
		},
		{
			name: "validatingWebhookConfiguration reconciliation fails while checking if exists",
			preReq: func(r *ExternalSecretsReconciler, m *fakes.FakeCtrlClient) {
				m.ExistsCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) (bool, error) {
					switch obj.(type) {
					case *webhook.ValidatingWebhookConfiguration:
						return false, testError
					}
					return false, nil
				})
			},
			wantErr: fmt.Sprintf("failed to check %s validatingWebhook resource already exists: %s", testValidateWebhookConfigurationResourceName, testError),
		},
		{
			name: "validatingWebhookConfiguration reconciliation fails while updating to desired state",
			preReq: func(r *ExternalSecretsReconciler, m *fakes.FakeCtrlClient) {
				m.UpdateWithRetryCalls(func(ctx context.Context, obj client.Object, option ...client.UpdateOption) error {
					switch obj.(type) {
					case *webhook.ValidatingWebhookConfiguration:
						return testError
					}
					return nil
				})
				m.ExistsCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) (bool, error) {
					switch o := obj.(type) {
					case *webhook.ValidatingWebhookConfiguration:
						webhookConfig := testValidatingWebhookConfiguration(validatingWebhookExternalSecretCRDAssetName)
						webhookConfig.SetLabels(nil)
						webhookConfig.DeepCopyInto(o)
						return true, nil
					}
					return false, nil
				})
			},
			wantErr: fmt.Sprintf("failed to update %s validatingWebhook resource with desired state: %s", testValidateWebhookConfigurationResourceName, testError),
		},
		{
			name: "validatingWebhookConfiguration reconciliation fails while creating",
			preReq: func(r *ExternalSecretsReconciler, m *fakes.FakeCtrlClient) {
				m.CreateCalls(func(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
					switch obj.(type) {
					case *webhook.ValidatingWebhookConfiguration:
						return testError
					}
					return nil
				})
			},
			wantErr: fmt.Sprintf("failed to create validatingWebhook resource %s: %s", testValidateWebhookConfigurationResourceName, testError),
		},
		{
			name: "validatingWebhookConfiguration creation successful",
			preReq: func(r *ExternalSecretsReconciler, m *fakes.FakeCtrlClient) {
				m.ExistsCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) (bool, error) {
					return false, nil
				})
				m.CreateCalls(func(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
					return nil
				})
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
			r.ctrlClient = mock
			externalSecretsForValidateWebhook := testExternalSecretsForValidateWebhookConfiguration()

			err := r.createOrApplyValidatingWebhookConfiguration(externalSecretsForValidateWebhook, false)
			if (tt.wantErr != "" || err != nil) && (err == nil || err.Error() != tt.wantErr) {
				t.Errorf("createOrApplyValidatingWebhookConfiguration() err: %v, wantErr: %v", err, tt.wantErr)
			}
		})
	}
}

func testExternalSecretsForValidateWebhookConfiguration() *v1alpha1.ExternalSecrets {
	externalSecrets := testExternalSecrets()
	externalSecrets.Spec = v1alpha1.ExternalSecretsSpec{
		ExternalSecretsConfig: &v1alpha1.ExternalSecretsConfig{
			WebhookConfig: &v1alpha1.WebhookConfig{
				CertManagerConfig: &v1alpha1.CertManagerConfig{
					AddInjectorAnnotations: "true",
				},
			},
		},
	}
	return externalSecrets
}
