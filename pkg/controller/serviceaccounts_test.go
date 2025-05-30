package controller

import (
	"context"
	"fmt"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/openshift/external-secrets-operator/api/v1alpha1"
	"github.com/openshift/external-secrets-operator/pkg/controller/fakes"
)

var testErr = fmt.Errorf("test client error")

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
		name                     string
		preReq                   func(*ExternalSecretsReconciler, *fakes.FakeCtrlClient)
		updateExternalSecretsObj func(*operatorv1alpha1.ExternalSecrets)
		wantErr                  string
	}{
		{
			name: "all static serviceaccounts created successfully",
			preReq: func(r *ExternalSecretsReconciler, m *fakes.FakeCtrlClient) {
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
			preReq: func(r *ExternalSecretsReconciler, m *fakes.FakeCtrlClient) {
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
			updateExternalSecretsObj: func(es *operatorv1alpha1.ExternalSecrets) {
				es.Spec.ExternalSecretsConfig = &operatorv1alpha1.ExternalSecretsConfig{
					BitwardenSecretManagerProvider: &operatorv1alpha1.BitwardenSecretManagerProvider{
						Enabled: "true",
					},
				}
			},
		},
		{
			name: "cert-controller serviceaccount skipped when cert-manager enabled",
			preReq: func(r *ExternalSecretsReconciler, m *fakes.FakeCtrlClient) {
				m.CreateCalls(func(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
					if sa, ok := obj.(*corev1.ServiceAccount); ok {
						if sa.Name == "external-secrets-cert-controller" {
							return testErr // should not be called
						}
					}
					return nil
				})
			},
			wantErr: "", // <- no error expected
			updateExternalSecretsObj: func(es *operatorv1alpha1.ExternalSecrets) {
				es.Spec.ExternalSecretsConfig = &operatorv1alpha1.ExternalSecretsConfig{
					WebhookConfig: &operatorv1alpha1.WebhookConfig{
						CertManagerConfig: &operatorv1alpha1.CertManagerConfig{
							Enabled: "true",
						},
					},
				}
			},
		},
		{
			name: "creation fails for controller Service account",
			preReq: func(r *ExternalSecretsReconciler, m *fakes.FakeCtrlClient) {
				m.ExistsCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) (bool, error) {
					return false, nil
				})
				m.CreateCalls(func(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
					if sa, ok := obj.(*corev1.ServiceAccount); ok && sa.Name == "external-secrets" {
						return testErr
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
			r.ctrlClient = mock
			if tt.preReq != nil {
				tt.preReq(r, mock)
			}

			es := testExternalSecrets()
			if tt.updateExternalSecretsObj != nil {
				tt.updateExternalSecretsObj(es)
			}

			err := r.createOrApplyServiceAccounts(es, controllerDefaultResourceLabels, false)
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
