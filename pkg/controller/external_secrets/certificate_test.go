package external_secrets

import (
	"context"
	"fmt"
	"testing"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"

	"github.com/openshift/external-secrets-operator/api/v1alpha1"
	"github.com/openshift/external-secrets-operator/pkg/controller/client/fakes"
	"github.com/openshift/external-secrets-operator/pkg/controller/common"
	"github.com/openshift/external-secrets-operator/pkg/controller/commontest"
)

var (
	testValidateCertificateResourceName = "external-secrets-webhook"
)

const (
	testIssuerName = "test-issuer"
)

func TestCreateOrApplyCertificates(t *testing.T) {
	tests := []struct {
		name                   string
		preReq                 func(*Reconciler, *fakes.FakeCtrlClient)
		esc                    func(*v1alpha1.ExternalSecretsConfig)
		recon                  bool
		wantErr                string
		wantUserConfigErr      bool
		wantUserConfigNotFound bool
		wantIrrecoverableErr   bool
		wantRetryRequiredErr   bool
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
				esc.Spec.ControllerConfig.CertProvider.CertManager.IssuerRef.Kind = issuerKind
			},
			recon:             false,
			wantUserConfigErr: true,
		},
		{
			name: "bitwarden enabled without secretRef or cert-manager returns user configuration error",
			esc: func(esc *v1alpha1.ExternalSecretsConfig) {
				esc.Spec.ControllerConfig.CertProvider = nil
				esc.Spec.Plugins.BitwardenSecretManagerProvider = &v1alpha1.BitwardenSecretManagerProvider{
					Mode: v1alpha1.Enabled,
				}
			},
			recon:             false,
			wantUserConfigErr: true,
		},
		{
			name: "cert manager config enabled but issuer does not exist",
			preReq: func(r *Reconciler, m *fakes.FakeCtrlClient) {
				m.ExistsCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) (bool, error) {
					if ns.Name == testIssuerName {
						return false, nil
					}
					return false, nil
				})
			},
			esc: func(esc *v1alpha1.ExternalSecretsConfig) {
				esc.Spec.ControllerConfig.CertProvider.CertManager.Mode = v1alpha1.Enabled
				esc.Spec.ControllerConfig.CertProvider.CertManager.IssuerRef.Name = testIssuerName
				esc.Spec.ControllerConfig.CertProvider.CertManager.IssuerRef.Kind = issuerKind
			},
			recon:                  false,
			wantUserConfigErr:      true,
			wantUserConfigNotFound: true,
		},
		{
			name: "reconciliation of webhook certificate fails while checking if exists",
			preReq: func(r *Reconciler, m *fakes.FakeCtrlClient) {
				m.ExistsCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) (bool, error) {
					if ns.Name == serviceExternalSecretWebhookName {
						return false, commontest.ErrTestClient
					}
					if ns.Name == testIssuerName {
						return true, nil
					}
					return false, nil
				})
				m.GetCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) error {
					if ns.Name == testIssuerName && ns.Namespace == commontest.TestExternalSecretsNamespace {
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
				esc.Spec.ControllerConfig.CertProvider.CertManager.IssuerRef.Name = testIssuerName
			},
			recon:   false,
			wantErr: fmt.Sprintf("failed to check %s/%s certificate resource already exists: %s", commontest.TestExternalSecretsNamespace, testValidateCertificateResourceName, commontest.ErrTestClient),
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
						if ns.Name == testIssuerName && (o.GetKind() == issuerKind || o.GetKind() == clusterIssuerKind) {
							var issuer client.Object
							if o.GetKind() == issuerKind {
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
					if ns.Name == testIssuerName {
						return true, nil
					}
					return false, nil
				})
				m.UpdateWithRetryCalls(func(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
					if obj.GetName() == serviceExternalSecretWebhookName {
						return commontest.ErrTestClient
					}
					return nil
				})
			},
			esc: func(esc *v1alpha1.ExternalSecretsConfig) {
				esc.Spec.ControllerConfig.CertProvider.CertManager.Mode = v1alpha1.Enabled
				esc.Spec.ControllerConfig.CertProvider.CertManager.IssuerRef.Name = testIssuerName
			},
			recon:   false,
			wantErr: fmt.Sprintf("failed to update %s/%s certificate resource: %s", commontest.TestExternalSecretsNamespace, testValidateCertificateResourceName, commontest.ErrTestClient),
		},
		{
			name: "reconciliation of webhook certificate which already exists in expected state",
			preReq: func(r *Reconciler, m *fakes.FakeCtrlClient) {
				m.GetCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) error {
					switch o := obj.(type) {
					case *certmanagerv1.Certificate:
						if ns.Name == serviceExternalSecretWebhookName {
							esc := testExternalSecretsConfigForCertificate()
							esc.Spec.ControllerConfig.CertProvider.CertManager.IssuerRef.Name = testIssuerName
							desiredCert, _ := r.getCertificateObject(esc, testResourceMetadata(esc), webhookCertificateAssetName)
							desiredCert.DeepCopyInto(o)
							return nil
						}
					case *unstructured.Unstructured:
						if ns.Name == testIssuerName && (o.GetKind() == issuerKind || o.GetKind() == clusterIssuerKind) {
							var issuer client.Object
							if o.GetKind() == issuerKind {
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
					if ns.Name == testIssuerName {
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
				esc.Spec.ControllerConfig.CertProvider.CertManager.IssuerRef.Name = testIssuerName
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
					if ns.Name == testIssuerName {
						return true, nil
					}
					return false, nil
				})
				m.CreateCalls(func(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
					if obj.GetName() == serviceExternalSecretWebhookName {
						return commontest.ErrTestClient
					}
					return nil
				})
				m.GetCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) error {
					if ns.Name == testIssuerName && ns.Namespace == commontest.TestExternalSecretsNamespace {
						testIssuer().DeepCopyInto(obj.(*certmanagerv1.Issuer))
						return nil
					}
					return fmt.Errorf("object not found")
				})
			},
			esc: func(esc *v1alpha1.ExternalSecretsConfig) {
				esc.Spec.ControllerConfig.CertProvider.CertManager.Mode = v1alpha1.Enabled
				esc.Spec.ControllerConfig.CertProvider.CertManager.IssuerRef.Name = testIssuerName
			},
			recon:   false,
			wantErr: fmt.Sprintf("failed to create Certificate %s/%s: %s", commontest.TestExternalSecretsNamespace, testValidateCertificateResourceName, commontest.ErrTestClient),
		},
		{
			name: "successful webhook certificate creation",
			preReq: func(r *Reconciler, m *fakes.FakeCtrlClient) {
				m.ExistsCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) (bool, error) {
					if ns.Name == serviceExternalSecretWebhookName {
						return false, nil
					}
					if ns.Name == testIssuerName {
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
					if ns.Name == testIssuerName && ns.Namespace == commontest.TestExternalSecretsNamespace {
						testIssuer().DeepCopyInto(obj.(*certmanagerv1.Issuer))
						return nil
					}
					return fmt.Errorf("object not found")
				})
			},
			esc: func(esc *v1alpha1.ExternalSecretsConfig) {
				esc.Spec.ControllerConfig.CertProvider.CertManager.Mode = v1alpha1.Enabled
				esc.Spec.ControllerConfig.CertProvider.CertManager.IssuerRef.Name = testIssuerName
			},
			recon: false,
		},
		{
			name: "issuer not found returns user configuration NotFound",
			preReq: func(r *Reconciler, m *fakes.FakeCtrlClient) {
				m.ExistsCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) (bool, error) {
					if ns.Name == testIssuerName {
						return false, nil
					}
					if ns.Name == serviceExternalSecretWebhookName {
						return false, nil
					}
					return false, nil
				})
			},
			esc: func(esc *v1alpha1.ExternalSecretsConfig) {
				esc.Spec.ControllerConfig.CertProvider.CertManager.Mode = v1alpha1.Enabled
				esc.Spec.ControllerConfig.CertProvider.CertManager.IssuerRef.Name = testIssuerName
			},
			recon:                  false,
			wantUserConfigErr:      true,
			wantUserConfigNotFound: true,
		},
		{
			name: "bitwarden enabled: secret ref exists (assertSecretRefExists returns)",
			preReq: func(r *Reconciler, m *fakes.FakeCtrlClient) {
				m.ExistsCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) (bool, error) {
					if ns.Name == serviceExternalSecretWebhookName {
						esc := testExternalSecretsConfigForCertificate()
						esc.Spec.ControllerConfig.CertProvider.CertManager.IssuerRef.Name = testIssuerName
						desiredCert, _ := r.getCertificateObject(esc, testResourceMetadata(esc), webhookCertificateAssetName)
						desiredCert.DeepCopyInto(obj.(*certmanagerv1.Certificate))
						return true, nil
					}
					if ns.Name == testIssuerName {
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
						if ns.Name == testIssuerName && ns.Namespace == commontest.TestExternalSecretsNamespace {
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
				esc.Spec.ControllerConfig.CertProvider.CertManager.IssuerRef.Name = testIssuerName
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
			name: "bitwarden enabled: secret ref not found returns user configuration error",
			preReq: func(r *Reconciler, m *fakes.FakeCtrlClient) {
				m.ExistsCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) (bool, error) {
					if ns.Name == serviceExternalSecretWebhookName {
						esc := testExternalSecretsConfigForCertificate()
						esc.Spec.ControllerConfig.CertProvider.CertManager.IssuerRef.Name = testIssuerName
						desiredCert, _ := r.getCertificateObject(esc, testResourceMetadata(esc), webhookCertificateAssetName)
						desiredCert.DeepCopyInto(obj.(*certmanagerv1.Certificate))
						return true, nil
					}
					if ns.Name == testIssuerName {
						return true, nil
					}
					return false, nil
				})
				m.GetCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) error {
					switch o := obj.(type) {
					case *corev1.Secret:
						if ns.Name == "bitwarden-secret" && ns.Namespace == commontest.TestExternalSecretsNamespace {
							return apierrors.NewNotFound(corev1.Resource("secrets"), ns.Name)
						}
					case *certmanagerv1.Issuer:
						if ns.Name == testIssuerName && ns.Namespace == commontest.TestExternalSecretsNamespace {
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
				esc.Spec.ControllerConfig.CertProvider.CertManager.IssuerRef.Name = testIssuerName
				esc.Spec.Plugins.BitwardenSecretManagerProvider = &v1alpha1.BitwardenSecretManagerProvider{
					SecretRef: &v1alpha1.SecretReference{
						Name: "bitwarden-secret",
					},
					Mode: v1alpha1.Enabled,
				}
			},
			recon:                  false,
			wantUserConfigErr:      true,
			wantUserConfigNotFound: true,
		},
		{
			name: "bitwarden enabled: secret ref fetch fails with retryable error",
			preReq: func(r *Reconciler, m *fakes.FakeCtrlClient) {
				m.ExistsCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) (bool, error) {
					if ns.Name == serviceExternalSecretWebhookName {
						esc := testExternalSecretsConfigForCertificate()
						esc.Spec.ControllerConfig.CertProvider.CertManager.IssuerRef.Name = testIssuerName
						desiredCert, _ := r.getCertificateObject(esc, testResourceMetadata(esc), webhookCertificateAssetName)
						desiredCert.DeepCopyInto(obj.(*certmanagerv1.Certificate))
						return true, nil
					}
					if ns.Name == testIssuerName {
						return true, nil
					}
					return false, nil
				})
				m.GetCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) error {
					switch o := obj.(type) {
					case *corev1.Secret:
						if ns.Name == "bitwarden-secret" && ns.Namespace == commontest.TestExternalSecretsNamespace {
							return commontest.ErrTestClient
						}
					case *certmanagerv1.Issuer:
						if ns.Name == testIssuerName && ns.Namespace == commontest.TestExternalSecretsNamespace {
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
				esc.Spec.ControllerConfig.CertProvider.CertManager.IssuerRef.Name = testIssuerName
				esc.Spec.Plugins.BitwardenSecretManagerProvider = &v1alpha1.BitwardenSecretManagerProvider{
					SecretRef: &v1alpha1.SecretReference{
						Name: "bitwarden-secret",
					},
					Mode: v1alpha1.Enabled,
				}
			},
			recon:                false,
			wantRetryRequiredErr: true,
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
					if ns.Name == testIssuerName {
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
					if ns.Name == testIssuerName && ns.Namespace == commontest.TestExternalSecretsNamespace {
						testIssuer().DeepCopyInto(obj.(*certmanagerv1.Issuer))
						return nil
					}
					return fmt.Errorf("object not found")
				})
			},
			esc: func(esc *v1alpha1.ExternalSecretsConfig) {
				esc.Spec.ControllerConfig.CertProvider.CertManager.Mode = v1alpha1.Enabled
				esc.Spec.ControllerConfig.CertProvider.CertManager.IssuerRef.Name = testIssuerName
				esc.Spec.Plugins.BitwardenSecretManagerProvider = nil
			},
			recon: false,
		},
		{
			name: "certificate with custom annotations applied successfully",
			preReq: func(r *Reconciler, m *fakes.FakeCtrlClient) {
				m.ExistsCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) (bool, error) {
					if ns.Name == "test-issuer" {
						return true, nil
					}
					return false, nil
				})
				m.CreateCalls(func(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
					if cert, ok := obj.(*certmanagerv1.Certificate); ok {
						// Verify annotations are applied
						if cert.Annotations == nil {
							t.Error("certificate annotations should not be nil")
							return nil
						}
						if cert.Annotations["app.io/issue-temporary-certificate"] != "true" {
							t.Errorf("expected annotation 'app.io/issue-temporary-certificate'='true', got '%s'",
								cert.Annotations["app.io/issue-temporary-certificate"])
						}
					}
					return nil
				})
				m.GetCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) error {
					if ns.Name == "test-issuer" {
						testIssuer().DeepCopyInto(obj.(*certmanagerv1.Issuer))
						return nil
					}
					return fmt.Errorf("object not found")
				})
			},
			esc: func(esc *v1alpha1.ExternalSecretsConfig) {
				esc.Spec.ControllerConfig.CertProvider.CertManager.Mode = v1alpha1.Enabled
				esc.Spec.ControllerConfig.CertProvider.CertManager.IssuerRef.Name = "test-issuer"
				esc.Spec.ControllerConfig.Annotations = map[string]string{
					"app.io/issue-temporary-certificate": "true",
					"team/owner":                         "security",
				}
			},
			recon: false,
		},
		{
			name: "certificate tracks managed annotations",
			preReq: func(r *Reconciler, m *fakes.FakeCtrlClient) {
				m.ExistsCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) (bool, error) {
					if ns.Name == "test-issuer" {
						return true, nil
					}
					return false, nil
				})
				m.CreateCalls(func(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
					if cert, ok := obj.(*certmanagerv1.Certificate); ok {
						// Verify all annotations from spec are present as managed annotations
						if cert.Annotations["allowed-cert-annotation"] != "value" {
							t.Errorf("expected 'allowed-cert-annotation'")
						}
					}
					return nil
				})
				m.GetCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) error {
					if ns.Name == "test-issuer" {
						testIssuer().DeepCopyInto(obj.(*certmanagerv1.Issuer))
						return nil
					}
					return fmt.Errorf("object not found")
				})
			},
			esc: func(esc *v1alpha1.ExternalSecretsConfig) {
				esc.Spec.ControllerConfig.CertProvider.CertManager.Mode = v1alpha1.Enabled
				esc.Spec.ControllerConfig.CertProvider.CertManager.IssuerRef.Name = "test-issuer"
				esc.Spec.ControllerConfig.Annotations = map[string]string{
					"allowed-cert-annotation": "value",
				}
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

			err := r.createOrApplyCertificates(esc, testResourceMetadata(esc), tt.recon)
			if tt.wantErr != "" {
				if err == nil || err.Error() != tt.wantErr {
					t.Errorf("createOrApplyCertificates() err: %v, wantErr: %v", err, tt.wantErr)
				}
			} else if err != nil && !tt.wantUserConfigErr && !tt.wantIrrecoverableErr && !tt.wantRetryRequiredErr {
				t.Errorf("createOrApplyCertificates() unexpected err: %v", err)
			}
			if tt.wantUserConfigErr && !common.IsUserConfigurationError(err) {
				t.Fatalf("createOrApplyCertificates() err = %v, want UserConfigurationError", err)
			}
			if tt.wantUserConfigNotFound && !common.IsUserConfigurationNotFound(err) {
				t.Fatalf("createOrApplyCertificates() err = %v, want UserConfigurationNotFound", err)
			}
			if tt.wantIrrecoverableErr && !common.IsIrrecoverableError(err) {
				t.Fatalf("createOrApplyCertificates() err = %v, want IrrecoverableError", err)
			}
			if tt.wantRetryRequiredErr && !common.IsRetryRequiredError(err) {
				t.Fatalf("createOrApplyCertificates() err = %v, want RetryRequiredError", err)
			}
			if !tt.wantUserConfigErr && common.IsUserConfigurationError(err) {
				t.Fatalf("createOrApplyCertificates() err = %v, unexpected UserConfigurationError", err)
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
			Name:      testIssuerName,
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
			Name: testIssuerName,
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

func TestCertificatePrivateKeyAlgorithm(t *testing.T) {
	t.Parallel()

	assertECDSAP256 := func(t *testing.T, cert *certmanagerv1.Certificate, name string) {
		t.Helper()
		if cert.Spec.PrivateKey == nil {
			t.Fatalf("%s: expected privateKey to be set", name)
		}
		if cert.Spec.PrivateKey.Algorithm != certmanagerv1.ECDSAKeyAlgorithm {
			t.Fatalf("%s: expected ECDSA algorithm, got %q", name, cert.Spec.PrivateKey.Algorithm)
		}
		if cert.Spec.PrivateKey.Size != 256 {
			t.Fatalf("%s: expected size 256, got %d", name, cert.Spec.PrivateKey.Size)
		}
		if cert.Spec.PrivateKey.Encoding != certmanagerv1.PKCS8 {
			t.Fatalf("%s: expected PKCS8 encoding, got %q", name, cert.Spec.PrivateKey.Encoding)
		}
	}

	// Bitwarden Certificate bindata is operator-preserved across helm refreshes.
	t.Run("bitwarden bindata uses ECDSA P-256", func(t *testing.T) {
		t.Parallel()
		assertECDSAP256(t, testCertificate(bitwardenCertificateAssetName), bitwardenCertificateAssetName)
	})

	// Webhook Certificate bindata is helm-refreshed; privateKey is enforced in
	// updateCertificateParams rather than the update-manifests script.
	t.Run("updateCertificateParams enforces ECDSA P-256", func(t *testing.T) {
		t.Parallel()
		esc := testExternalSecretsConfigForCertificate()
		esc.Spec.ControllerConfig.CertProvider.CertManager.Mode = v1alpha1.Enabled
		esc.Spec.ControllerConfig.CertProvider.CertManager.IssuerRef.Name = testIssuerName
		esc.Spec.ControllerConfig.CertProvider.CertManager.IssuerRef.Kind = issuerKind

		r := testReconciler(t)
		mock := &fakes.FakeCtrlClient{}
		mock.ExistsCalls(func(ctx context.Context, ns types.NamespacedName, obj client.Object) (bool, error) {
			if ns.Name == testIssuerName {
				if issuer, ok := obj.(*certmanagerv1.Issuer); ok {
					testIssuer().DeepCopyInto(issuer)
				}
				return true, nil
			}
			return false, nil
		})
		r.UncachedClient = mock

		cert := testCertificate(webhookCertificateAssetName)
		cert.Spec.PrivateKey = &certmanagerv1.CertificatePrivateKey{
			Algorithm: certmanagerv1.RSAKeyAlgorithm,
			Encoding:  certmanagerv1.PKCS1,
			Size:      2048,
		}
		if err := r.updateCertificateParams(esc, cert); err != nil {
			t.Fatalf("updateCertificateParams() unexpected err: %v", err)
		}
		assertECDSAP256(t, cert, "reconciled webhook certificate")
	})
}
