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
		es      func(*v1alpha1.ExternalSecrets)
		wantErr string
	}{
		{
			name:   "external secret spec disabled",
			preReq: nil,
			es: func(es *v1alpha1.ExternalSecrets) {
				es.Spec = v1alpha1.ExternalSecretsSpec{}
			},
		},
		{
			name:   "externalSecretConfig is nil",
			preReq: nil,
			es: func(es *v1alpha1.ExternalSecrets) {
				es.Spec.ExternalSecretsConfig = nil
			},
		},
		{
			name:   "webhook config is nil",
			preReq: nil,
			es: func(es *v1alpha1.ExternalSecrets) {
				es.Spec.ExternalSecretsConfig = &v1alpha1.ExternalSecretsConfig{
					WebhookConfig: nil,
				}
			},
		},
		{
			name:   "webhook config is empty",
			preReq: nil,
			es: func(es *v1alpha1.ExternalSecrets) {
				es.Spec.ExternalSecretsConfig = &v1alpha1.ExternalSecretsConfig{
					WebhookConfig: &v1alpha1.WebhookConfig{},
				}
			},
		},
		{
			name:   "cert manager config is nil",
			preReq: nil,
			es: func(es *v1alpha1.ExternalSecrets) {
				es.Spec.ExternalSecretsConfig = &v1alpha1.ExternalSecretsConfig{
					CertManagerConfig: nil,
				}
			},
		},
		{
			name: "reconciliation of secret fails while checking if exists",
			preReq: func(r *Reconciler, m *fakes.FakeCtrlClient) {
				m.GetCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) error {
					switch o := obj.(type) {
					case *corev1.Secret:
						secret := testSecret(webhookTLSSecretAssetName)
						secret.DeepCopyInto(o)
					}
					return nil
				})
				m.ExistsCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) (bool, error) {
					switch obj.(type) {
					case *corev1.Secret:
						return false, commontest.TestClientError
					}
					return true, nil
				})
			},
			wantErr: fmt.Sprintf("failed to check %s/%s secret resource already exists: %s", commontest.TestExternalSecretsNamespace, testValidateSecretResourceName, commontest.TestClientError),
		},
		{
			name: "reconciliation of secret fails while restoring to expected state",
			preReq: func(r *Reconciler, m *fakes.FakeCtrlClient) {
				m.GetCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) error {
					switch o := obj.(type) {
					case *corev1.Secret:
						secret := testSecret(webhookTLSSecretAssetName)
						secret.DeepCopyInto(o)
					}
					return nil
				})
				m.ExistsCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) (bool, error) {
					switch o := obj.(type) {
					case *corev1.Secret:
						secret := testSecret(webhookTLSSecretAssetName)
						secret.SetLabels(map[string]string{"test": "test"})
						secret.DeepCopyInto(o)
					}
					return true, nil
				})
				m.UpdateWithRetryCalls(func(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
					switch obj.(type) {
					case *corev1.Secret:
						return commontest.TestClientError
					}
					return nil
				})
			},
			wantErr: fmt.Sprintf("failed to update %s/%s secret resource: %s", commontest.TestExternalSecretsNamespace, testValidateSecretResourceName, commontest.TestClientError),
		},
		{
			name: "reconciliation of secret which already exists in expected state",
			preReq: func(r *Reconciler, m *fakes.FakeCtrlClient) {
				m.GetCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) error {
					switch o := obj.(type) {
					case *corev1.Secret:
						secret := testSecret(webhookTLSSecretAssetName)
						secret.DeepCopyInto(o)
					}
					return nil
				})
				m.ExistsCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) (bool, error) {
					switch o := obj.(type) {
					case *corev1.Secret:
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
					switch o := obj.(type) {
					case *corev1.Secret:
						secret := testSecret(webhookTLSSecretAssetName)
						secret.DeepCopyInto(o)
					}
					return nil
				})
				m.ExistsCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) (bool, error) {
					switch obj.(type) {
					case *corev1.Secret:
						return false, nil
					}
					return true, nil
				})
				m.CreateCalls(func(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
					switch obj.(type) {
					case *corev1.Secret:
						return commontest.TestClientError
					}
					return nil
				})
			},
			wantErr: fmt.Sprintf("failed to create %s/%s secret resource: %s", commontest.TestExternalSecretsNamespace, testValidateSecretResourceName, commontest.TestClientError),
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
		{
			name: "webhook secret deletion fails",
			preReq: func(r *Reconciler, m *fakes.FakeCtrlClient) {
				m.ExistsCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) (bool, error) {
					switch o := obj.(type) {
					case *corev1.Secret:
						secret := testSecret(webhookTLSSecretAssetName)
						secret.DeepCopyInto(o)
					}
					return true, nil
				})
				m.DeleteCalls(func(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
					return commontest.TestClientError
				})
			},
			es: func(es *v1alpha1.ExternalSecrets) {
				es.Spec.ExternalSecretsConfig = &v1alpha1.ExternalSecretsConfig{
					CertManagerConfig: &v1alpha1.CertManagerConfig{
						Enabled: "true",
					},
				}
			},
			wantErr: `failed to delete secret resource of webhook component: test client error`,
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
			es := testExternalSecretsForSecrets()
			if tt.es != nil {
				tt.es(es)
			}

			err := r.createOrApplySecret(es, controllerDefaultResourceLabels, false)
			if (tt.wantErr != "" || err != nil) && (err == nil || err.Error() != tt.wantErr) {
				t.Errorf("createOrApplySecret() err: %v, wantErr: %v", err, tt.wantErr)
			}
		})
	}
}

func testExternalSecretsForSecrets() *v1alpha1.ExternalSecrets {
	externalSecrets := commontest.TestExternalSecrets()

	externalSecrets.Spec = v1alpha1.ExternalSecretsSpec{
		ControllerConfig: &v1alpha1.ControllerConfig{
			Namespace: commontest.TestExternalSecretsNamespace,
		},
		ExternalSecretsConfig: &v1alpha1.ExternalSecretsConfig{
			CertManagerConfig: &v1alpha1.CertManagerConfig{
				Enabled: "false",
			},
		},
	}
	return externalSecrets
}
