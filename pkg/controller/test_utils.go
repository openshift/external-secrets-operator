package controller

import (
	"context"
	"fmt"
	"github.com/go-logr/logr/testr"
	"testing"

	v1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"

	webhook "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"

	operatorv1alpha1 "github.com/openshift/external-secrets-operator/api/v1alpha1"
	"github.com/openshift/external-secrets-operator/pkg/operator/assets"
)

var (
	testError     = fmt.Errorf("test client error")
	testNamespace = "test-external-secrets"
)

const (
	// testResourcesName is the name for ExternalSecrets test CR.
	testResourcesName = "externalsecrets-test-resource"
)

func testReconciler(t *testing.T) *ExternalSecretsReconciler {
	return &ExternalSecretsReconciler{
		ctx:           context.Background(),
		eventRecorder: record.NewFakeRecorder(100),
		log:           testr.New(t),
		Scheme:        runtime.NewScheme(),
	}
}

func testValidatingWebhookConfiguration(testValidateWebhookConfigurationFile string) *webhook.ValidatingWebhookConfiguration {
	validateWebhook := decodeValidatingWebhookConfigurationObjBytes(assets.MustAsset(testValidateWebhookConfigurationFile))
	validateWebhook.SetLabels(controllerDefaultResourceLabels)
	return validateWebhook
}

// testExternalSecrets returns a sample ExternalSecrets object.
func testExternalSecrets() *operatorv1alpha1.ExternalSecrets {
	return &operatorv1alpha1.ExternalSecrets{
		ObjectMeta: metav1.ObjectMeta{
			Name: testResourcesName,
		},
	}
}

func testCertificate() *v1.Certificate {
	validateCertificate := decodeCertificateObjBytes(assets.MustAsset(webhookCertificateAssetName))
	validateCertificate.SetLabels(controllerDefaultResourceLabels)
	return validateCertificate
}

func testSecret() *corev1.Secret {
	validateSecret := decodeSecretObjBytes(assets.MustAsset(webhookTLSSecretAssetName))
	validateSecret.SetLabels(controllerDefaultResourceLabels)
	return validateSecret
}
