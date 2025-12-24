package external_secrets

import (
	"context"
	"testing"

	webhook "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"

	"github.com/go-logr/logr/testr"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"

	"github.com/openshift/external-secrets-operator/pkg/controller/common"
	"github.com/openshift/external-secrets-operator/pkg/controller/commontest"
	"github.com/openshift/external-secrets-operator/pkg/operator/assets"
)

// testReconciler returns a sample Reconciler instance.
func testReconciler(t *testing.T) *Reconciler {
	return &Reconciler{
		Scheme:                runtime.NewScheme(),
		ctx:                   context.Background(),
		eventRecorder:         record.NewFakeRecorder(100),
		log:                   testr.New(t),
		esm:                   commontest.TestExternalSecretsManager(),
		optionalResourcesList: make(map[string]struct{}),
	}
}

// testService returns a Service object decoded from the specified asset file,.
func testService(assetName string) *corev1.Service {
	service := common.DecodeServiceObjBytes(assets.MustAsset(assetName))
	service.SetLabels(controllerDefaultResourceLabels)
	return service
}

// testServiceAccount returns a ServiceAccount object decoded from the specified asset file,.
func testServiceAccount(assetName string) *corev1.ServiceAccount {
	serviceAccount := common.DecodeServiceAccountObjBytes(assets.MustAsset(assetName))
	serviceAccount.SetLabels(controllerDefaultResourceLabels)
	return serviceAccount
}

// testClusterRole returns ClusterRole object read from provided static asset of same kind.
func testClusterRole(assetName string) *rbacv1.ClusterRole {
	role := common.DecodeClusterRoleObjBytes(assets.MustAsset(assetName))
	role.SetLabels(controllerDefaultResourceLabels)
	return role
}

// testClusterRoleBinding returns ClusterRoleBinding object read from provided static asset of same kind.
func testClusterRoleBinding(assetName string) *rbacv1.ClusterRoleBinding {
	roleBinding := common.DecodeClusterRoleBindingObjBytes(assets.MustAsset(assetName))
	roleBinding.SetLabels(controllerDefaultResourceLabels)
	return roleBinding
}

// testRole returns Role object read from provided static asset of same kind.
func testRole(assetName string) *rbacv1.Role {
	role := common.DecodeRoleObjBytes(assets.MustAsset(assetName))
	role.SetLabels(controllerDefaultResourceLabels)
	return role
}

// testRoleBinding returns RoleBinding object read from provided static asset of same kind.
func testRoleBinding(assetName string) *rbacv1.RoleBinding {
	roleBinding := common.DecodeRoleBindingObjBytes(assets.MustAsset(assetName))
	roleBinding.SetLabels(controllerDefaultResourceLabels)
	return roleBinding
}

// testValidatingWebhookConfiguration returns ValidatingWebhookConfiguration object read from provided static asset of same kind.
func testValidatingWebhookConfiguration(assetName string) *webhook.ValidatingWebhookConfiguration {
	validateWebhook := common.DecodeValidatingWebhookConfigurationObjBytes(assets.MustAsset(assetName))
	return validateWebhook
}

// Helper function to create a dummy deployment for testing.
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
							Image: commontest.TestExternalSecretsImageName,
						},
					},
				},
			},
		},
	}
}

// testCertificate returns Certificate object read from provided static asset of same kind.
func testCertificate(assetName string) *certmanagerv1.Certificate {
	validateCertificate := common.DecodeCertificateObjBytes(assets.MustAsset(assetName))
	validateCertificate.SetLabels(controllerDefaultResourceLabels)
	return validateCertificate
}

// testSecret returns Secret object read from provided static asset of same kind.
func testSecret(assetName string) *corev1.Secret {
	validateSecret := common.DecodeSecretObjBytes(assets.MustAsset(assetName))
	validateSecret.SetLabels(controllerDefaultResourceLabels)
	return validateSecret
}

// testNetworkPolicy returns NetworkPolicy object read from provided static asset of same kind.
func testNetworkPolicy(assetName string) *networkingv1.NetworkPolicy {
	networkPolicy := common.DecodeNetworkPolicyObjBytes(assets.MustAsset(assetName))
	networkPolicy.SetLabels(controllerDefaultResourceLabels)
	return networkPolicy
}
