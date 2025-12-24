package external_secrets

import (
	"context"
	"fmt"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"

	"github.com/openshift/external-secrets-operator/api/v1alpha1"
	"github.com/openshift/external-secrets-operator/pkg/controller/client/fakes"
	"github.com/openshift/external-secrets-operator/pkg/controller/commontest"
)

var (
	testValidateCertificateResourceName = "external-secrets-webhook"
)

func TestCreateOrApplyCertificates(t *testing.T) {
	tests := []struct {
		name    string
		preReq  func(*Reconciler, *fakes.FakeCtrlClient)
		esc     func(*v1alpha1.ExternalSecretsConfig)
		recon   bool
		wantErr string
	}{
		{
			name:   "external secret spec disabled",
			preReq: nil,
			esc: func(esc *v1alpha1.ExternalSecretsConfig) {
				esc.Spec = v1alpha1.ExternalSecretsConfigSpec{}
			},
			recon: false,
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
			recon: false,
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
			recon: false,
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
			recon: false,
		},
		{
			name:   "cert manager config enabled but issuerRef.Name is empty",
			preReq: nil,
			esc: func(esc *v1alpha1.ExternalSecretsConfig) {
				esc.Spec.ControllerConfig.CertProvider.CertManager.Mode = v1alpha1.Enabled
				esc.Spec.ControllerConfig.CertProvider.CertManager.IssuerRef.Name = ""
				esc.Spec.ControllerConfig.CertProvider.CertManager.IssuerRef.Kind = "Issuer"
			},
			recon:   false,
			wantErr: fmt.Sprintf("failed to update certificate resource for %s/%s deployment: cert-manager.issuerRef.name is not configured", commontest.TestExternalSecretsNamespace, testExternalSecretsConfigForCertificate().GetName()),
		},
		{
			name: "reconciliation of webhook certificate fails while checking if exists",
			preReq: func(r *Reconciler, m *fakes.FakeCtrlClient) {
				m.ExistsCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) (bool, error) {
					if ns.Name == serviceExternalSecretWebhookName {
						return false, commontest.TestClientError
					}
					if ns.Name == "test-issuer" {
						return true, nil
					}
					return false, nil
				})
				m.GetCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) error {
					if ns.Name == "test-issuer" && ns.Namespace == commontest.TestExternalSecretsNamespace {
						if u, ok := obj.(*unstructured.Unstructured); ok {
							issuer := testIssuer()
							unstructuredIssuer, err := runtime.DefaultUnstructuredConverter.ToUnstructured(issuer)
							if err != nil {
								return err
							}
							u.Object = unstructuredIssuer
							return nil
						}
						if o, ok := obj.(*certmanagerv1.Issuer); ok {
							testIssuer().DeepCopyInto(o)
							return nil
						}
					}
					return fmt.Errorf("object not found: %s/%s", ns.Namespace, ns.Name)
				})
			},
			esc: func(esc *v1alpha1.ExternalSecretsConfig) {
				esc.Spec.ControllerConfig.CertProvider.CertManager.Mode = v1alpha1.Enabled
				esc.Spec.ControllerConfig.CertProvider.CertManager.IssuerRef.Name = "test-issuer"
			},
			recon:   false,
			wantErr: fmt.Sprintf("failed to check %s/%s certificate resource already exists: %s", commontest.TestExternalSecretsNamespace, testValidateCertificateResourceName, commontest.TestClientError),
		},
		{
			name: "reconciliation of webhook certificate fails while restoring to expected state",
			preReq: func(r *Reconciler, m *fakes.FakeCtrlClient) {
				m.GetCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) error {
					switch o := obj.(type) {
					case *certmanagerv1.Certificate:
						if ns.Name == serviceExternalSecretWebhookName {
							cert := testCertificate(webhookCertificateAssetName)
							cert.SetLabels(map[string]string{"different": "labels"})
							cert.DeepCopyInto(o)
							return nil
						}
					case *unstructured.Unstructured:
						if ns.Name == "test-issuer" && (o.GetKind() == "Issuer" || o.GetKind() == "ClusterIssuer") {
							var issuer client.Object
							if o.GetKind() == "Issuer" {
								issuer = testIssuer()
							} else {
								issuer = testClusterIssuer()
							}
							unstructuredContent, err := runtime.DefaultUnstructuredConverter.ToUnstructured(issuer)
							if err != nil {
								return fmt.Errorf("failed to convert issuer to unstructured: %w", err)
							}
							o.Object = unstructuredContent
							return nil
						}
					}
					return fmt.Errorf("object not found")
				})
				m.ExistsCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) (bool, error) {
					if ns.Name == serviceExternalSecretWebhookName {
						return true, nil
					}
					if ns.Name == "test-issuer" {
						return true, nil
					}
					return false, nil
				})
				m.UpdateWithRetryCalls(func(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
					if obj.GetName() == serviceExternalSecretWebhookName {
						return commontest.TestClientError
					}
					return nil
				})
			},
			esc: func(esc *v1alpha1.ExternalSecretsConfig) {
				esc.Spec.ControllerConfig.CertProvider.CertManager.Mode = v1alpha1.Enabled
				esc.Spec.ControllerConfig.CertProvider.CertManager.IssuerRef.Name = "test-issuer"
			},
			recon:   false,
			wantErr: fmt.Sprintf("failed to update %s/%s certificate resource: %s", commontest.TestExternalSecretsNamespace, testValidateCertificateResourceName, commontest.TestClientError),
		},
		{
			name: "reconciliation of webhook certificate which already exists in expected state",
			preReq: func(r *Reconciler, m *fakes.FakeCtrlClient) {
				m.GetCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) error {
					switch o := obj.(type) {
					case *certmanagerv1.Certificate:
						if ns.Name == serviceExternalSecretWebhookName {
							esc := testExternalSecretsConfigForCertificate()
							esc.Spec.ControllerConfig.CertProvider.CertManager.IssuerRef.Name = "test-issuer"
							desiredCert, _ := r.getCertificateObject(esc, controllerDefaultResourceLabels, webhookCertificateAssetName)
							desiredCert.DeepCopyInto(o)
							return nil
						}
					case *unstructured.Unstructured:
						if ns.Name == "test-issuer" && (o.GetKind() == "Issuer" || o.GetKind() == "ClusterIssuer") {
							var issuer client.Object
							if o.GetKind() == "Issuer" {
								issuer = testIssuer()
							} else {
								issuer = testClusterIssuer()
							}
							unstructuredContent, err := runtime.DefaultUnstructuredConverter.ToUnstructured(issuer)
							if err != nil {
								return fmt.Errorf("failed to convert issuer to unstructured: %w", err)
							}
							o.Object = unstructuredContent
							return nil
						}
					}
					return fmt.Errorf("object not found")
				})
				m.ExistsCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) (bool, error) {
					if ns.Name == serviceExternalSecretWebhookName {
						return true, nil
					}
					if ns.Name == "test-issuer" {
						return true, nil
					}
					return false, nil
				})
				m.CreateCalls(func(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
					t.Errorf("Create was called unexpectedly for %s", obj.GetName())
					return nil
				})
				m.UpdateWithRetryCalls(func(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
					return nil
				})
			},
			esc: func(esc *v1alpha1.ExternalSecretsConfig) {
				esc.Spec.ControllerConfig.CertProvider.CertManager.Mode = v1alpha1.Enabled
				esc.Spec.ControllerConfig.CertProvider.CertManager.IssuerRef.Name = "test-issuer"
			},
			recon: false,
		},
		{
			name: "reconciliation of webhook certificate creation fails",
			preReq: func(r *Reconciler, m *fakes.FakeCtrlClient) {
				m.ExistsCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) (bool, error) {
					if ns.Name == serviceExternalSecretWebhookName {
						return false, nil
					}
					if ns.Name == "test-issuer" {
						return true, nil
					}
					return false, nil
				})
				m.CreateCalls(func(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
					if obj.GetName() == serviceExternalSecretWebhookName {
						return commontest.TestClientError
					}
					return nil
				})
				m.GetCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) error {
					if ns.Name == "test-issuer" && ns.Namespace == commontest.TestExternalSecretsNamespace {
						testIssuer().DeepCopyInto(obj.(*certmanagerv1.Issuer))
						return nil
					}
					return fmt.Errorf("object not found")
				})
			},
			esc: func(esc *v1alpha1.ExternalSecretsConfig) {
				esc.Spec.ControllerConfig.CertProvider.CertManager.Mode = v1alpha1.Enabled
				esc.Spec.ControllerConfig.CertProvider.CertManager.IssuerRef.Name = "test-issuer"
			},
			recon:   false,
			wantErr: fmt.Sprintf("failed to create %s/%s certificate resource: %s", commontest.TestExternalSecretsNamespace, testValidateCertificateResourceName, commontest.TestClientError),
		},
		{
			name: "successful webhook certificate creation",
			preReq: func(r *Reconciler, m *fakes.FakeCtrlClient) {
				m.ExistsCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) (bool, error) {
					if ns.Name == serviceExternalSecretWebhookName {
						return false, nil
					}
					if ns.Name == "test-issuer" {
						return true, nil
					}
					return false, nil
				})
				m.CreateCalls(func(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
					if obj.GetName() == serviceExternalSecretWebhookName {
						return nil
					}
					t.Errorf("unexpected create call for %s", obj.GetName())
					return nil
				})
				m.GetCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) error {
					if ns.Name == "test-issuer" && ns.Namespace == commontest.TestExternalSecretsNamespace {
						testIssuer().DeepCopyInto(obj.(*certmanagerv1.Issuer))
						return nil
					}
					return fmt.Errorf("object not found")
				})
			},
			esc: func(esc *v1alpha1.ExternalSecretsConfig) {
				esc.Spec.ControllerConfig.CertProvider.CertManager.Mode = v1alpha1.Enabled
				esc.Spec.ControllerConfig.CertProvider.CertManager.IssuerRef.Name = "test-issuer"
			},
			recon: false,
		},
		{
			name: "bitwarden enabled: secret ref exists (assertSecretRefExists returns)",
			preReq: func(r *Reconciler, m *fakes.FakeCtrlClient) {
				m.ExistsCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) (bool, error) {
					if ns.Name == serviceExternalSecretWebhookName {
						esc := testExternalSecretsConfigForCertificate()
						esc.Spec.ControllerConfig.CertProvider.CertManager.IssuerRef.Name = "test-issuer"
						desiredCert, _ := r.getCertificateObject(esc, controllerDefaultResourceLabels, webhookCertificateAssetName)
						desiredCert.DeepCopyInto(obj.(*certmanagerv1.Certificate))
						return true, nil
					}
					if ns.Name == "test-issuer" {
						return true, nil
					}
					return false, nil
				})
				m.GetCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) error {
					switch o := obj.(type) {
					case *corev1.Secret:
						if ns.Name == "bitwarden-secret" && ns.Namespace == commontest.TestExternalSecretsNamespace {
							testSecretForCertificate().DeepCopyInto(o)
							return nil
						}
					case *certmanagerv1.Issuer:
						if ns.Name == "test-issuer" && ns.Namespace == commontest.TestExternalSecretsNamespace {
							testIssuer().DeepCopyInto(o)
							return nil
						}
					}
					return fmt.Errorf("object not found for %s/%s", ns.Namespace, ns.Name)
				})
				m.CreateCalls(func(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
					t.Errorf("Create was called for %s when SecretRef exists and assertion should return early", obj.GetName())
					return nil
				})
				m.UpdateWithRetryCalls(func(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
					t.Errorf("UpdateWithRetry was called unexpectedly for %s", obj.GetName())
					return nil
				})
			},
			esc: func(esc *v1alpha1.ExternalSecretsConfig) {
				esc.Spec.ControllerConfig.CertProvider.CertManager.Mode = v1alpha1.Enabled
				esc.Spec.ControllerConfig.CertProvider.CertManager.IssuerRef.Name = "test-issuer"
				esc.Spec.Plugins.BitwardenSecretManagerProvider = &v1alpha1.BitwardenSecretManagerProvider{
					SecretRef: &v1alpha1.SecretReference{
						Name: "bitwarden-secret",
					},
					Mode: v1alpha1.Enabled,
				}
			},
			recon:   false,
			wantErr: "",
		},
		{
			name: "bitwarden enabled: secret ref does not exist (assertSecretRefExists fails)",
			preReq: func(r *Reconciler, m *fakes.FakeCtrlClient) {
				m.ExistsCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) (bool, error) {
					if ns.Name == serviceExternalSecretWebhookName {
						esc := testExternalSecretsConfigForCertificate()
						esc.Spec.ControllerConfig.CertProvider.CertManager.IssuerRef.Name = "test-issuer"
						desiredCert, _ := r.getCertificateObject(esc, controllerDefaultResourceLabels, webhookCertificateAssetName)
						desiredCert.DeepCopyInto(obj.(*certmanagerv1.Certificate))
						return true, nil
					}
					if ns.Name == "test-issuer" {
						return true, nil
					}
					return false, nil
				})
				m.GetCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) error {
					switch o := obj.(type) {
					case *corev1.Secret:
						if ns.Name == "bitwarden-secret" && ns.Namespace == commontest.TestExternalSecretsNamespace {
							return commontest.TestClientError
						}
					case *certmanagerv1.Issuer:
						if ns.Name == "test-issuer" && ns.Namespace == commontest.TestExternalSecretsNamespace {
							testIssuer().DeepCopyInto(o)
							return nil
						}
					}
					return fmt.Errorf("object not found")
				})
				m.CreateCalls(func(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
					t.Errorf("Create was called when SecretRef assertion should have failed and returned early")
					return nil
				})
			},
			esc: func(esc *v1alpha1.ExternalSecretsConfig) {
				esc.Spec.ControllerConfig.CertProvider.CertManager.Mode = v1alpha1.Enabled
				esc.Spec.ControllerConfig.CertProvider.CertManager.IssuerRef.Name = "test-issuer"
				esc.Spec.Plugins.BitwardenSecretManagerProvider = &v1alpha1.BitwardenSecretManagerProvider{
					SecretRef: &v1alpha1.SecretReference{
						Name: "bitwarden-secret",
					},
					Mode: v1alpha1.Enabled,
				}
			},
			recon:   false,
			wantErr: fmt.Sprintf("failed to fetch %q secret: %s", types.NamespacedName{Name: "bitwarden-secret", Namespace: commontest.TestExternalSecretsNamespace}, commontest.TestClientError),
		},
		{
			name: "bitwarden disabled (explicitly nil): only webhook certificate reconciled",
			preReq: func(r *Reconciler, m *fakes.FakeCtrlClient) {
				m.ExistsCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) (bool, error) {
					if ns.Name == serviceExternalSecretWebhookName {
						return false, nil
					}
					if ns.Name == "bitwarden-sdk-server" {
						t.Errorf("Should not check for bitwarden-sdk-server certificate when Bitwarden config is nil")
					}
					if ns.Name == "test-issuer" {
						return true, nil
					}
					return false, nil
				})
				m.CreateCalls(func(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
					cert, ok := obj.(*certmanagerv1.Certificate)
					if !ok {
						return fmt.Errorf("expected *certmanagerv1.Certificate, got %T", obj)
					}
					if cert.Name == serviceExternalSecretWebhookName {
						return nil
					}
					t.Errorf("Unexpected create call for %s", cert.Name)
					return nil
				})
				m.GetCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) error {
					if ns.Name == "test-issuer" && ns.Namespace == commontest.TestExternalSecretsNamespace {
						testIssuer().DeepCopyInto(obj.(*certmanagerv1.Issuer))
						return nil
					}
					return fmt.Errorf("object not found")
				})
			},
			esc: func(esc *v1alpha1.ExternalSecretsConfig) {
				esc.Spec.ControllerConfig.CertProvider.CertManager.Mode = v1alpha1.Enabled
				esc.Spec.ControllerConfig.CertProvider.CertManager.IssuerRef.Name = "test-issuer"
				esc.Spec.Plugins.BitwardenSecretManagerProvider = nil
			},
			recon: false,
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
			r.UncachedClient = mock

			esc := testExternalSecretsConfigForCertificate()
			if tt.esc != nil {
				tt.esc(esc)
			}

			err := r.createOrApplyCertificates(esc, controllerDefaultResourceLabels, tt.recon)
			if (tt.wantErr != "" || err != nil) && (err == nil || err.Error() != tt.wantErr) {
				t.Errorf("createOrApplyCertificates() err: %v, wantErr: %v", err, tt.wantErr)
			}
		})
	}
}

func testExternalSecretsConfigForCertificate() *v1alpha1.ExternalSecretsConfig {
	esc := commontest.TestExternalSecretsConfig()
	esc.Spec = v1alpha1.ExternalSecretsConfigSpec{
		ControllerConfig: v1alpha1.ControllerConfig{
			CertProvider: &v1alpha1.CertProvidersConfig{
				CertManager: &v1alpha1.CertManagerConfig{
					IssuerRef: &v1alpha1.ObjectReference{},
				},
			},
		},
		ApplicationConfig: v1alpha1.ApplicationConfig{
			OperatingNamespace: "test-ns",
		},
		Plugins: v1alpha1.PluginsConfig{
			BitwardenSecretManagerProvider: &v1alpha1.BitwardenSecretManagerProvider{},
		},
	}
	return esc
}

// testIssuer creates a dummy cert-manager Issuer for testing.
func testIssuer() *certmanagerv1.Issuer {
	return &certmanagerv1.Issuer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-issuer",
			Namespace: commontest.TestExternalSecretsNamespace,
		},
		Spec: certmanagerv1.IssuerSpec{
			IssuerConfig: certmanagerv1.IssuerConfig{
				SelfSigned: &certmanagerv1.SelfSignedIssuer{},
			},
		},
	}
}

// testClusterIssuer creates a dummy cert-manager ClusterIssuer for testing.
func testClusterIssuer() *certmanagerv1.ClusterIssuer {
	return &certmanagerv1.ClusterIssuer{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-issuer",
		},
		Spec: certmanagerv1.IssuerSpec{
			IssuerConfig: certmanagerv1.IssuerConfig{
				SelfSigned: &certmanagerv1.SelfSignedIssuer{},
			},
		},
	}
}

func testSecretForCertificate() *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "bitwarden-secret",
			Namespace: commontest.TestExternalSecretsNamespace,
		},
		Data: map[string][]byte{
			"username": []byte("testuser"),
			"password": []byte("testpassword"),
		},
	}
}
