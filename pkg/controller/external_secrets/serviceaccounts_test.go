package external_secrets

import (
	"context"
	"fmt"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/openshift/external-secrets-operator/api/v1alpha1"
	"github.com/openshift/external-secrets-operator/pkg/controller/client/fakes"
	"github.com/openshift/external-secrets-operator/pkg/controller/commontest"
)

var errTest = fmt.Errorf("test client error")

func staticServiceAccounts() map[string]string {
	return map[string]string{
		"external-secrets":                 "external-secrets/resources/serviceaccount_external-secrets.yml",
		"external-secrets-cert-controller": "external-secrets/resources/serviceaccount_external-secrets-cert-controller.yml",
		"external-secrets-webhook":         "external-secrets/resources/serviceaccount_external-secrets-webhook.yml",
		"bitwarden-sdk-server":             "external-secrets/resources/serviceaccount_bitwarden-sdk-server.yml",
	}
}

func TestCreateOrApplyServiceAccounts(t *testing.T) {
	tests := []struct {
		name                        string
		preReq                      func(*Reconciler, *fakes.FakeCtrlClient)
		updateExternalSecretsConfig func(*operatorv1alpha1.ExternalSecretsConfig)
		wantErr                     string
	}{
		{
			name: "all static serviceaccounts created successfully",
			preReq: func(r *Reconciler, m *fakes.FakeCtrlClient) {
				m.ExistsCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) (bool, error) {
					return false, nil
				})

				expectedSAMap := make(map[string]*corev1.ServiceAccount)
				for name, path := range staticServiceAccounts() {
					expectedSAMap[name] = testServiceAccount(path)
				}

				m.CreateCalls(func(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
					if sa, ok := obj.(*corev1.ServiceAccount); ok {
						if _, found := expectedSAMap[sa.Name]; found {
							return nil
						}
						return fmt.Errorf("unexpected ServiceAccount created: %s", sa.Name)
					}
					return nil
				})
			},
		},
		{
			name: "bitwarden serviceaccount created when enabled",
			preReq: func(r *Reconciler, m *fakes.FakeCtrlClient) {
				m.ExistsCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) (bool, error) {
					return false, nil
				})
				m.CreateCalls(func(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
					expectedSA := testServiceAccount("external-secrets/resources/serviceaccount_bitwarden-sdk-server.yml")
					if sa, ok := obj.(*corev1.ServiceAccount); ok {
						if sa.Name == expectedSA.Name {
							return nil
						}
					}
					return nil
				})
			},
			updateExternalSecretsConfig: func(esc *operatorv1alpha1.ExternalSecretsConfig) {
				esc.Spec = operatorv1alpha1.ExternalSecretsConfigSpec{
					Plugins: operatorv1alpha1.PluginsConfig{
						BitwardenSecretManagerProvider: &operatorv1alpha1.BitwardenSecretManagerProvider{
							Mode: operatorv1alpha1.Enabled,
						},
					},
				}
			},
		},
		{
			name: "cert-controller serviceaccount skipped when cert-manager enabled",
			preReq: func(r *Reconciler, m *fakes.FakeCtrlClient) {
				m.CreateCalls(func(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
					if sa, ok := obj.(*corev1.ServiceAccount); ok {
						if sa.Name == "external-secrets-cert-controller" {
							return errTest // should not be called
						}
					}
					return nil
				})
			},
			wantErr: "", // <- no error expected
			updateExternalSecretsConfig: func(esc *operatorv1alpha1.ExternalSecretsConfig) {
				esc.Spec = operatorv1alpha1.ExternalSecretsConfigSpec{
					ControllerConfig: operatorv1alpha1.ControllerConfig{
						CertProvider: &operatorv1alpha1.CertProvidersConfig{
							CertManager: &operatorv1alpha1.CertManagerConfig{
								Mode: operatorv1alpha1.Enabled,
							},
						},
					},
				}
			},
		},
		{
			name: "creation fails for controller Service account",
			preReq: func(r *Reconciler, m *fakes.FakeCtrlClient) {
				m.ExistsCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) (bool, error) {
					return false, nil
				})
				m.CreateCalls(func(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
					if sa, ok := obj.(*corev1.ServiceAccount); ok && sa.Name == "external-secrets" {
						return errTest
					}
					return nil
				})
			},
			wantErr: "failed to create serviceaccount external-secrets/external-secrets: test client error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := testReconciler(t)
			mock := &fakes.FakeCtrlClient{}
			r.CtrlClient = mock
			if tt.preReq != nil {
				tt.preReq(r, mock)
			}

			esc := commontest.TestExternalSecretsConfig()
			if tt.updateExternalSecretsConfig != nil {
				tt.updateExternalSecretsConfig(esc)
			}

			err := r.createOrApplyServiceAccounts(esc, controllerDefaultResourceLabels, false)
			if tt.wantErr != "" {
				if err == nil || err.Error() != tt.wantErr {
					t.Errorf("Expected error: %v, got: %v", tt.wantErr, err)
				}
			} else if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}
