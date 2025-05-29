package controller

import (
	"context"
	"fmt"
	"testing"

	webhook "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-logr/logr/testr"

	v1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"

	operatorv1alpha1 "github.com/openshift/external-secrets-operator/api/v1alpha1"
	"github.com/openshift/external-secrets-operator/pkg/operator/assets"
)

const (
	// testResourcesName is the name for ExternalSecrets test CR.
	testResourcesName = "externalsecrets-test-resource"

	testImageName = "registry.redhat.io/external-secrets-operator/external-secrets-operator-rhel9"

	testNamespace = "test-external-secrets"
)

var (
	// testError is the error to return for client failure scenarios.
	testError = fmt.Errorf("test client error")
)

// testReconciler returns a sample ExternalSecretsReconciler instance.
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

// testService returns a Service object decoded from the specified asset file,
func testService(assetName string) *corev1.Service {
	service := decodeServiceObjBytes(assets.MustAsset(assetName))
	service.SetLabels(controllerDefaultResourceLabels)
	return service
}

// testServiceAccount returns a ServiceAccount object decoded from the specified asset file,
func testServiceAccount(assetName string) *corev1.ServiceAccount {
	serviceAccount := decodeServiceAccountObjBytes(assets.MustAsset(assetName))
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

// testClusterRole returns ClusterRole object read from provided static asset of same kind.
func testClusterRole(assetName string) *rbacv1.ClusterRole {
	role := decodeClusterRoleObjBytes(assets.MustAsset(assetName))
	role.SetLabels(controllerDefaultResourceLabels)
	return role
}

// testClusterRoleBinding returns ClusterRoleBinding object read from provided static asset of same kind.
func testClusterRoleBinding(assetName string) *rbacv1.ClusterRoleBinding {
	roleBinding := decodeClusterRoleBindingObjBytes(assets.MustAsset(assetName))
	roleBinding.SetLabels(controllerDefaultResourceLabels)
	return roleBinding
}

// testRole returns Role object read from provided static asset of same kind.
func testRole(assetName string) *rbacv1.Role {
	role := decodeRoleObjBytes(assets.MustAsset(assetName))
	role.SetLabels(controllerDefaultResourceLabels)
	return role
}

// testRoleBinding returns RoleBinding object read from provided static asset of same kind.
func testRoleBinding(assetName string) *rbacv1.RoleBinding {
	roleBinding := decodeRoleBindingObjBytes(assets.MustAsset(assetName))
	roleBinding.SetLabels(controllerDefaultResourceLabels)
	return roleBinding
}

func testValidatingWebhookConfiguration(testValidateWebhookConfigurationFile string) *webhook.ValidatingWebhookConfiguration {
	validateWebhook := decodeValidatingWebhookConfigurationObjBytes(assets.MustAsset(testValidateWebhookConfigurationFile))
	return validateWebhook
}

// Helper function to create a dummy deployment for testing
func testDeployment(name string) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: controllerDefaultResourceLabels,
		},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: controllerDefaultResourceLabels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  externalsecretsCommonName,
							Image: testImageName,
						},
					},
				},
			},
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
