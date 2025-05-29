package controller

import (
	"context"
	"fmt"
	"testing"

	"github.com/go-logr/logr/testr"
	webhook "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/openshift/external-secrets-operator/api/v1alpha1"
	"github.com/openshift/external-secrets-operator/pkg/operator/assets"
)

var (
	testError = fmt.Errorf("test client error")
)

const (
	// testResourcesName is the name for ExternalSecrets test CR.
	testResourcesName            = "externalsecrets-test-resource"
	testExternalSecretsNamespace = "test-namespace"
)

func testReconciler(t *testing.T) *ExternalSecretsReconciler {
	return &ExternalSecretsReconciler{
		Scheme:                runtime.NewScheme(),
		ctx:                   context.Background(),
		eventRecorder:         record.NewFakeRecorder(100),
		log:                   testr.New(t),
		esm:                   testExternalSecretsManager(),
		optionalResourcesList: make(map[client.Object]struct{}),
	}
}

func testService(assetName string) *corev1.Service {
	service := decodeServiceObjBytes(assets.MustAsset(assetName))
	service.SetLabels(controllerDefaultResourceLabels)
	return service
}

func testServiceAccount(assetName string) *corev1.ServiceAccount {
	serviceAccount := decodeServiceAccountObjBytes(assets.MustAsset(assetName))
	serviceAccount.SetNamespace(testExternalSecretsNamespace)
	serviceAccount.SetLabels(controllerDefaultResourceLabels)
	return serviceAccount
}

// testExternalSecrets returns a sample ExternalSecrets object.
func testExternalSecrets() *operatorv1alpha1.ExternalSecrets {
	return &operatorv1alpha1.ExternalSecrets{
		ObjectMeta: metav1.ObjectMeta{
			Name: testResourcesName,
		},
	}
}

// testExternalSecretsManager returns a sample ExternalSecretsManager object.
func testExternalSecretsManager() *operatorv1alpha1.ExternalSecretsManager {
	return &operatorv1alpha1.ExternalSecretsManager{
		ObjectMeta: metav1.ObjectMeta{
			Name: testResourcesName,
		},
	}
}

func testValidatingWebhookConfiguration(testValidateWebhookConfigurationFile string) *webhook.ValidatingWebhookConfiguration {
	validateWebhook := decodeValidatingWebhookConfigurationObjBytes(assets.MustAsset(testValidateWebhookConfigurationFile))
	return validateWebhook
}
