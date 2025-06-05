package external_secrets

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/openshift/external-secrets-operator/api/v1alpha1"
	"github.com/openshift/external-secrets-operator/pkg/controller/client/fakes"
	"github.com/openshift/external-secrets-operator/pkg/controller/commontest"
)

func TestCreateOrApplyServices(t *testing.T) {
	tests := []struct {
		name                     string
		preReq                   func(*Reconciler, *fakes.FakeCtrlClient)
		updateExternalSecretsObj func(*operatorv1alpha1.ExternalSecrets)
		wantErr                  string
	}{
		{
			name: "service reconciliation successful",
			preReq: func(r *Reconciler, m *fakes.FakeCtrlClient) {
				m.ExistsCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) (bool, error) {
					switch o := obj.(type) {
					case *corev1.Service:
						svc := testService("external-secrets/resources/service_external-secrets-webhook.yml")
						svc.DeepCopyInto(o)
						return true, nil
					}
					return false, nil
				})
			},
		},
		{
			name: "bitwarden service created when enabled",
			preReq: func(r *Reconciler, m *fakes.FakeCtrlClient) {
				m.ExistsCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) (bool, error) {
					switch o := obj.(type) {
					case *corev1.Service:
						svc := testService("external-secrets/resources/service_bitwarden-sdk-server.yml")
						svc.DeepCopyInto(o)
						return false, nil
					}
					return false, nil
				})
				m.CreateCalls(func(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
					switch svc := obj.(type) {
					case *corev1.Service:
						if svc.Name == "bitwarden-sdk-server" {
							return commontest.TestClientError // trigger error
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
			wantErr: `failed to create service external-secrets/bitwarden-sdk-server: test client error`,
		},

		{
			name: "service reconciliation fails while checking if exists",
			preReq: func(r *Reconciler, m *fakes.FakeCtrlClient) {
				m.ExistsCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) (bool, error) {
					return false, commontest.TestClientError
				})
			},
			wantErr: `failed to check existence of service external-secrets/external-secrets-webhook: test client error`,
		},
		{
			name: "service reconciliation fails while updating to desired state",
			preReq: func(r *Reconciler, m *fakes.FakeCtrlClient) {
				m.ExistsCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) (bool, error) {
					switch o := obj.(type) {
					case *corev1.Service:
						svc := testService("external-secrets/resources/service_external-secrets-webhook.yml")
						svc.SetLabels(nil) // Trigger update
						svc.DeepCopyInto(o)
						return true, nil
					}
					return false, nil
				})
				m.UpdateWithRetryCalls(func(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
					return commontest.TestClientError
				})
			},
			wantErr: `failed to update service external-secrets/external-secrets-webhook: test client error`,
		},
		{
			name: "service reconciliation fails while creating",
			preReq: func(r *Reconciler, m *fakes.FakeCtrlClient) {
				m.ExistsCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) (bool, error) {
					return false, nil
				})
				m.CreateCalls(func(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
					switch svc := obj.(type) {
					case *corev1.Service:
						if svc.Name != "external-secrets-webhook" {
							t.Errorf("Expected webhook service to be created, got %s", svc.Name)
						}
					}
					return commontest.TestClientError
				})
			},
			wantErr: `failed to create service external-secrets/external-secrets-webhook: test client error`,
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
			es := commontest.TestExternalSecrets()
			if tt.updateExternalSecretsObj != nil {
				tt.updateExternalSecretsObj(es)
			}
			err := r.createOrApplyServices(es, controllerDefaultResourceLabels, false)
			if (tt.wantErr != "" || err != nil) && (err == nil || err.Error() != tt.wantErr) {
				t.Errorf("createOrApplyServices() err: %v, wantErr: %v", err, tt.wantErr)
			}
		})
	}
}
