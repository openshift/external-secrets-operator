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
		name                        string
		preReq                      func(*Reconciler, *fakes.FakeCtrlClient)
		updateExternalSecretsConfig func(*v1alpha1.ExternalSecretsConfig)
		wantErr                     string
	}{
		{
			name: "validatingWebhookConfiguration reconciliation successful",
			preReq: func(r *Reconciler, m *fakes.FakeCtrlClient) {
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
			preReq: func(r *Reconciler, m *fakes.FakeCtrlClient) {
				m.ExistsCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) (bool, error) {
					switch obj.(type) {
					case *webhook.ValidatingWebhookConfiguration:
						return false, commontest.TestClientError
					}
					return false, nil
				})
			},
			wantErr: fmt.Sprintf("failed to check %s validatingWebhook resource already exists: %s", testValidateWebhookConfigurationResourceName, commontest.TestClientError),
		},
		{
			name: "validatingWebhookConfiguration reconciliation fails while updating to desired state",
			preReq: func(r *Reconciler, m *fakes.FakeCtrlClient) {
				m.UpdateWithRetryCalls(func(ctx context.Context, obj client.Object, option ...client.UpdateOption) error {
					switch obj.(type) {
					case *webhook.ValidatingWebhookConfiguration:
						return commontest.TestClientError
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
			wantErr: fmt.Sprintf("failed to update %s validatingWebhook resource with desired state: %s", testValidateWebhookConfigurationResourceName, commontest.TestClientError),
		},
		{
			name: "validatingWebhookConfiguration reconciliation fails while creating",
			preReq: func(r *Reconciler, m *fakes.FakeCtrlClient) {
				m.CreateCalls(func(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
					switch obj.(type) {
					case *webhook.ValidatingWebhookConfiguration:
						return commontest.TestClientError
					}
					return nil
				})
			},
			wantErr: fmt.Sprintf("failed to create validatingWebhook resource %s: %s", testValidateWebhookConfigurationResourceName, commontest.TestClientError),
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
		{
			name: "validatingWebhookConfiguration with custom annotations applied successfully",
			preReq: func(r *Reconciler, m *fakes.FakeCtrlClient) {
				m.ExistsCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) (bool, error) {
					return false, nil
				})
				m.CreateCalls(func(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
					if vwc, ok := obj.(*webhook.ValidatingWebhookConfiguration); ok {
						// Verify annotations are applied
						if vwc.Annotations == nil {
							t.Error("validatingwebhook annotations should not be nil")
							return nil
						}
						if vwc.Annotations["monitoring/enabled"] != "true" {
							t.Errorf("expected annotation 'monitoring/enabled'='true', got '%s'", 
								vwc.Annotations["monitoring/enabled"])
						}
					}
					return nil
				})
			},
			updateExternalSecretsConfig: func(esc *v1alpha1.ExternalSecretsConfig) {
				esc.Spec.ControllerConfig.Annotations = map[string]string{
					"monitoring/enabled": "true",
					"team/owner":         "platform",
				}
			},
		},
		{
			name: "validatingWebhookConfiguration filters reserved annotation prefixes",
			preReq: func(r *Reconciler, m *fakes.FakeCtrlClient) {
				m.ExistsCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) (bool, error) {
					return false, nil
				})
				m.CreateCalls(func(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
					if vwc, ok := obj.(*webhook.ValidatingWebhookConfiguration); ok {
						// Verify only allowed annotation is present
						if vwc.Annotations["custom-key"] != "custom-value" {
							t.Errorf("expected 'custom-key' annotation")
						}
						// Verify reserved prefixes were filtered
						if _, exists := vwc.Annotations["kubernetes.io/test"]; exists {
							t.Error("reserved prefix 'kubernetes.io/' should have been filtered")
						}
					}
					return nil
				})
			},
			updateExternalSecretsConfig: func(esc *v1alpha1.ExternalSecretsConfig) {
				esc.Spec.ControllerConfig.Annotations = map[string]string{
					"custom-key":         "custom-value",
					"kubernetes.io/test": "filtered",
				}
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
		if tt.updateExternalSecretsConfig != nil {
			tt.updateExternalSecretsConfig(externalSecretsForValidateWebhook)
		}

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
