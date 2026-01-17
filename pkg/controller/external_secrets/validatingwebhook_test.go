package external_secrets

import (
	"context"
	"fmt"
	"testing"

	webhook "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/openshift/external-secrets-operator/api/v1alpha1"
	"github.com/openshift/external-secrets-operator/pkg/controller/client/fakes"
	"github.com/openshift/external-secrets-operator/pkg/controller/commontest"
)

var (
	testValidateWebhookConfigurationResourceName = "externalsecret-validate"
)

func TestCreateOrApplyValidatingWebhookConfiguration(t *testing.T) {
	tests := []struct {
		name    string
		preReq  func(*Reconciler, *fakes.FakeCtrlClient)
		wantErr string
	}{
		{
			name: "validatingWebhookConfiguration reconciliation successful",
			preReq: func(r *Reconciler, m *fakes.FakeCtrlClient) {
				m.ExistsCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) (bool, error) {
					if o, ok := obj.(*webhook.ValidatingWebhookConfiguration); ok {
						webhookConfig := testValidatingWebhookConfiguration(validatingWebhookExternalSecretCRDAssetName)
						webhookConfig.DeepCopyInto(o)
					}
					return true, nil
				})
			},
		},
		{
			name: "validatingWebhookConfiguration reconciliation fails while checking if exists",
			preReq: func(r *Reconciler, m *fakes.FakeCtrlClient) {
				m.ExistsCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) (bool, error) {
					if _, ok := obj.(*webhook.ValidatingWebhookConfiguration); ok {
						return false, commontest.ErrTestClient
					}
					return false, nil
				})
			},
			wantErr: fmt.Sprintf("failed to check %s validatingWebhook resource already exists: %s", testValidateWebhookConfigurationResourceName, commontest.ErrTestClient),
		},
		{
			name: "validatingWebhookConfiguration reconciliation fails while updating to desired state",
			preReq: func(r *Reconciler, m *fakes.FakeCtrlClient) {
				m.UpdateWithRetryCalls(func(ctx context.Context, obj client.Object, option ...client.UpdateOption) error {
					if _, ok := obj.(*webhook.ValidatingWebhookConfiguration); ok {
						return commontest.ErrTestClient
					}
					return nil
				})
				m.ExistsCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) (bool, error) {
					if o, ok := obj.(*webhook.ValidatingWebhookConfiguration); ok {
						webhookConfig := testValidatingWebhookConfiguration(validatingWebhookExternalSecretCRDAssetName)
						webhookConfig.SetLabels(nil)
						webhookConfig.DeepCopyInto(o)
						return true, nil
					}
					return false, nil
				})
			},
			wantErr: fmt.Sprintf("failed to update %s validatingWebhook resource with desired state: %s", testValidateWebhookConfigurationResourceName, commontest.ErrTestClient),
		},
		{
			name: "validatingWebhookConfiguration reconciliation fails while creating",
			preReq: func(r *Reconciler, m *fakes.FakeCtrlClient) {
				m.CreateCalls(func(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
					if _, ok := obj.(*webhook.ValidatingWebhookConfiguration); ok {
						return commontest.ErrTestClient
					}
					return nil
				})
			},
			wantErr: fmt.Sprintf("failed to create validatingWebhook resource %s: %s", testValidateWebhookConfigurationResourceName, commontest.ErrTestClient),
		},
		{
			name: "validatingWebhookConfiguration creation successful",
			preReq: func(r *Reconciler, m *fakes.FakeCtrlClient) {
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
			r.CtrlClient = mock
			externalSecretsForValidateWebhook := testExternalSecretsForValidateWebhookConfiguration()

			err := r.createOrApplyValidatingWebhookConfiguration(externalSecretsForValidateWebhook, controllerDefaultResourceLabels, false)
			if (tt.wantErr != "" || err != nil) && (err == nil || err.Error() != tt.wantErr) {
				t.Errorf("createOrApplyValidatingWebhookConfiguration() err: %v, wantErr: %v", err, tt.wantErr)
			}
		})
	}
}

func testExternalSecretsForValidateWebhookConfiguration() *v1alpha1.ExternalSecretsConfig {
	esc := commontest.TestExternalSecretsConfig()
	esc.Spec = v1alpha1.ExternalSecretsConfigSpec{
		ControllerConfig: v1alpha1.ControllerConfig{
			CertProvider: &v1alpha1.CertProvidersConfig{
				CertManager: &v1alpha1.CertManagerConfig{
					InjectAnnotations: "true",
				},
			},
		},
	}
	return esc
}
