package controller

import (
	"context"
	"fmt"
	"testing"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"

	"github.com/openshift/external-secrets-operator/api/v1alpha1"
	"github.com/openshift/external-secrets-operator/pkg/controller/fakes"
)

var (
	testValidateCertificateResourceName = "external-secrets-webhook"
)

func TestCreateOrApplyCertificates(t *testing.T) {
	tests := []struct {
		name    string
		preReq  func(*ExternalSecretsReconciler, *fakes.FakeCtrlClient)
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
					WebhookConfig: &v1alpha1.WebhookConfig{
						CertManagerConfig: nil,
					},
				}
			},
		},
		{
			name: "reconciliation of certificate fails while checking if exists",
			preReq: func(r *ExternalSecretsReconciler, m *fakes.FakeCtrlClient) {
				m.GetCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) error {
					switch o := obj.(type) {
					case *certmanagerv1.Certificate:
						cert := testCertificate()
						cert.DeepCopyInto(o)
					}
					return nil
				})
				m.ExistsCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) (bool, error) {
					switch obj.(type) {
					case *certmanagerv1.Certificate:
						return false, testError
					}
					return true, nil
				})
			},
			wantErr: fmt.Sprintf("failed to check %s/%s certificate resource already exists: %s", testNamespace, testValidateCertificateResourceName, testError),
		},
		{
			name: "reconciliation of certificate fails while restoring to expected state",
			preReq: func(r *ExternalSecretsReconciler, m *fakes.FakeCtrlClient) {
				m.GetCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) error {
					switch o := obj.(type) {
					case *certmanagerv1.Certificate:
						cert := testCertificate()
						cert.DeepCopyInto(o)
					}
					return nil
				})
				m.ExistsCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) (bool, error) {
					switch o := obj.(type) {
					case *certmanagerv1.Certificate:
						cert := testCertificate()
						cert.SetLabels(map[string]string{"test": "test"})
						cert.DeepCopyInto(o)
					}
					return true, nil
				})
				m.UpdateWithRetryCalls(func(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
					switch obj.(type) {
					case *certmanagerv1.Certificate:
						return testError
					}
					return nil
				})
			},
			wantErr: fmt.Sprintf("failed to update %s/%s certificate resource: %s", testNamespace, testValidateCertificateResourceName, testError),
		},
		{
			name: "reconciliation of certificate which already exists in expected state",
			preReq: func(r *ExternalSecretsReconciler, m *fakes.FakeCtrlClient) {
				m.GetCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) error {
					switch o := obj.(type) {
					case *certmanagerv1.Certificate:
						cert := testCertificate()
						cert.DeepCopyInto(o)
					}
					return nil
				})
				m.ExistsCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) (bool, error) {
					switch o := obj.(type) {
					case *certmanagerv1.Certificate:
						cert := testCertificate()
						cert.DeepCopyInto(o)
					}
					return true, nil
				})
			},
		},
		{
			name: "reconciliation of certificate creation fails",
			preReq: func(r *ExternalSecretsReconciler, m *fakes.FakeCtrlClient) {
				m.GetCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) error {
					switch o := obj.(type) {
					case *certmanagerv1.Certificate:
						cert := testCertificate()
						cert.DeepCopyInto(o)
					}
					return nil
				})
				m.ExistsCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) (bool, error) {
					switch obj.(type) {
					case *certmanagerv1.Certificate:
						return false, nil
					}
					return true, nil
				})
				m.CreateCalls(func(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
					switch obj.(type) {
					case *certmanagerv1.Certificate:
						return testError
					}
					return nil
				})
			},
			wantErr: fmt.Sprintf("failed to create %s/%s certificate resource: %s", testNamespace, testValidateCertificateResourceName, testError),
		},
		{
			name: "successful certificate creation",
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
			
			es := testExternalSecretsForCertificate()
			if tt.es != nil {
				tt.es(es)
			}

			err := r.createOrApplyCertificates(es, controllerDefaultResourceLabels, false)
			if (tt.wantErr != "" || err != nil) && (err == nil || err.Error() != tt.wantErr) {
				t.Errorf("createOrApplyCertificates() err: %v, wantErr: %v", err, tt.wantErr)
			}
		})
	}
}

func testExternalSecretsForCertificate() *v1alpha1.ExternalSecrets {
	externalSecrets := testExternalSecrets()

	externalSecrets.Spec = v1alpha1.ExternalSecretsSpec{
		ControllerConfig: &v1alpha1.ControllerConfig{
			Namespace: testNamespace,
		},
		ExternalSecretsConfig: &v1alpha1.ExternalSecretsConfig{
			WebhookConfig: &v1alpha1.WebhookConfig{
				CertManagerConfig: &v1alpha1.CertManagerConfig{
					Enabled: "true",
				},
			},
		},
	}
	return externalSecrets
}
