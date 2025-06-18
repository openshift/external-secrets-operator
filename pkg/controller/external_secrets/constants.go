package external_secrets

import (
	"fmt"
	"os"

	certmanagerapi "github.com/cert-manager/cert-manager/pkg/apis/certmanager"
	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	"github.com/openshift/external-secrets-operator/pkg/controller/common"
)

const (
	// externalsecretsCommonName is the name commonly used for naming resources.
	externalsecretsCommonName = "external-secrets"

	// ControllerName is the name of the controller used in logs and events.
	ControllerName = externalsecretsCommonName + "-controller"

	// finalizer name for external-secrets.openshift.operator.io resource.
	finalizer = "externalsecrets.openshift.operator.io/" + ControllerName

	// controllerProcessedAnnotation is the annotation added to external-secrets resource once after
	// successful reconciliation by the controller.
	controllerProcessedAnnotation = "operator.openshift.io/external-secrets-processed"

	// certificateCRDGroupVersion is the group and version of the Certificate CRD provided by cert-manager project.
	certificateCRDGroupVersion = "cert-manager.io/v1"

	// certificateCRDName is the name of the Certificate CRD provided by cert-manager project.
	certificateCRDName = "certificates"

	// externalsecretsImageVersionEnvVarName is the environment variable key name
	// containing the image version of the external-secrets operand as value.
	externalsecretsImageVersionEnvVarName = "OPERAND_EXTERNAL_SECRETS_IMAGE_VERSION"

	// externalsecretsImageEnvVarName is the environment variable key name
	// containing the image version of the external-secrets as value.
	externalsecretsImageEnvVarName = "RELATED_IMAGE_EXTERNAL_SECRETS"

	// externalsecretsDefaultNamespace is the namespace where the `external-secrets` operand required resources
	// will be created, when ExternalSecrets.Spec.ControllerConfig.Namespace is not set.
	externalsecretsDefaultNamespace = "external-secrets"
)

var (
	// certificateCRDGKV is the group.version/kind of the Certificate CRD.
	certificateCRDGKV = fmt.Sprintf("certificate.%s/%s", certmanagerv1.SchemeGroupVersion.Group, certmanagerv1.SchemeGroupVersion.Version)
)

var (
	// controllerDefaultResourceLabels is default set of labels added to all resources
	// created for external-secrets deployment.
	controllerDefaultResourceLabels = map[string]string{
		"app":                          externalsecretsCommonName,
		"app.kubernetes.io/version":    os.Getenv(externalsecretsImageVersionEnvVarName),
		"app.kubernetes.io/managed-by": common.ExternalSecretsOperatorCommonName,
		"app.kubernetes.io/part-of":    common.ExternalSecretsOperatorCommonName,
	}
)

// asset names are the files present in the root `bindata/` dir, which are then loaded to
// and made available by the pkg/operator/assets package.
const (
	externalsecretsNamespaceAssetName             = "external-secrets/external-secrets-namespace.yaml"
	bitwardenCertificateAssetName                 = "external-secrets/certificate_bitwarden-tls-certs.yml"
	webhookCertificateAssetName                   = "external-secrets/resources/certificate_external-secrets-webhook.yml"
	certControllerClusterRoleAssetName            = "external-secrets/resources/clusterrole_external-secrets-cert-controller.yml"
	controllerClusterRoleAssetName                = "external-secrets/resources/clusterrole_external-secrets-controller.yml"
	controllerClusterRoleEditAssetName            = "external-secrets/resources/clusterrole_external-secrets-edit.yml"
	controllerClusterRoleServiceBindingsAssetName = "external-secrets/resources/clusterrole_external-secrets-servicebindings.yml"
	controllerClusterRoleViewAssetName            = "external-secrets/resources/clusterrole_external-secrets-view.yml"
	certControllerClusterRoleBindingAssetName     = "external-secrets/resources/clusterrolebinding_external-secrets-cert-controller.yml"
	controllerClusterRoleBindingAssetName         = "external-secrets/resources/clusterrolebinding_external-secrets-controller.yml"
	bitwardenDeploymentAssetName                  = "external-secrets/resources/deployment_bitwarden-sdk-server.yml"
	controllerDeploymentAssetName                 = "external-secrets/resources/deployment_external-secrets.yml"
	certControllerDeploymentAssetName             = "external-secrets/resources/deployment_external-secrets-cert-controller.yml"
	webhookDeploymentAssetName                    = "external-secrets/resources/deployment_external-secrets-webhook.yml"
	controllerRoleLeaderElectionAssetName         = "external-secrets/resources/role_external-secrets-leaderelection.yml"
	controllerRoleBindingLeaderElectionAssetName  = "external-secrets/resources/rolebinding_external-secrets-leaderelection.yml"
	webhookTLSSecretAssetName                     = "external-secrets/resources/secret_external-secrets-webhook.yml"
	bitwardenServiceAssetName                     = "external-secrets/resources/service_bitwarden-sdk-server.yml"
	webhookServiceAssetName                       = "external-secrets/resources/service_external-secrets-webhook.yml"
	controllerServiceAccountAssetName             = "external-secrets/resources/serviceaccount_external-secrets.yml"
	bitwardenServiceAccountAssetName              = "external-secrets/resources/serviceaccount_bitwarden-sdk-server.yml"
	certControllerServiceAccountAssetName         = "external-secrets/resources/serviceaccount_external-secrets-cert-controller.yml"
	webhookServiceAccountAssetName                = "external-secrets/resources/serviceaccount_external-secrets-webhook.yml"
	validatingWebhookExternalSecretCRDAssetName   = "external-secrets/resources/validatingwebhookconfiguration_externalsecret-validate.yml"
	validatingWebhookSecretStoreCRDAssetName      = "external-secrets/resources/validatingwebhookconfiguration_secretstore-validate.yml"
)

var (
	clusterIssuerKind = certmanagerv1.ClusterIssuerKind
	issuerKind        = certmanagerv1.IssuerKind
	issuerGroup       = certmanagerapi.GroupName
)
