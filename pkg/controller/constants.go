package controller

import (
	"os"
	"time"
)

const (
	// externalsecretsCommonName is the name commonly used for naming resources.
	externalsecretsCommonName = "external-secrets"

	// ControllerName is the name of the controller used in logs and events.
	ControllerName = externalsecretsCommonName + "-controller"

	// defaultRequeueTime is the default reconcile requeue time.
	defaultRequeueTime = time.Second * 30

	// finalizer name for external-secrets.openshift.operator.io resource.
	finalizer = "external-secrets.openshift.operator.io/" + ControllerName

	// controllerProcessedAnnotation is the annotation added to external-secrets resource once after
	// successful reconciliation by the controller.
	controllerProcessedAnnotation = "operator.openshift.io/external-secrets-processed"

	externalsecretsObjectName = "cluster"

	// certificateCRDGroupVersion is the group and version of the Certificate CRD provided by cert-manager project.
	certificateCRDGroupVersion = "cert-manager.io/v1"

	// certificateCRDName is the name of the Certificate CRD provided by cert-manager project.
	certificateCRDName = "certificates"

	// certManagerInjectCAFromAnnotation is the annotation key added to external-secrets resource once
	// if certManager field is enabled in webhook config
	// after successful reconciliation by the controller.
	certManagerInjectCAFromAnnotation = "cert-manager.io/inject-ca-from"

	// certManagerInjectCAFromAnnotationValue is the annotation value added to external-secrets resource once
	// if certManager field is enabled in webhook config
	// after successful reconciliation by the controller.
	certManagerInjectCAFromAnnotationValue = "external-secrets/external-secrets-webhook"

	// externalsecretsImageEnvVarName is the environment variable key name
	// containing the image version of the external-secrets operand as value.
	externalsecretsImageVersionEnvVarName = "OPERAND_EXTERNAL_SECRETS_IMAGE_VERSION"

	// externalsecretsImageEnvVarName is the environment variable key name
	// containing the image version of the external-secrets as value.
	externalsecretsImageEnvVarName = "RELATED_IMAGE_EXTERNAL_SECRETS"
)

var (
	controllerDefaultResourceLabels = map[string]string{
		"app":                          externalsecretsCommonName,
		"app.kubernetes.io/name":       externalsecretsCommonName,
		"app.kubernetes.io/instance":   externalsecretsCommonName,
		"app.kubernetes.io/version":    os.Getenv(externalsecretsImageVersionEnvVarName),
		"app.kubernetes.io/managed-by": "external-secrets-operator",
		"app.kubernetes.io/part-of":    "external-secrets-operator",
	}
)

// asset names are the files present in the root `bindata/` dir, which are then loaded to
// and made available by the pkg/operator/assets package.
const (
	validatingWebhookExternalSecretCRDAssetName = "external-secrets/resources/validatingwebhookconfiguration_externalsecret-validate.yml"
	validatingWebhookSecretStoreCRDAssetName    = "external-secrets/resources/validatingwebhookconfiguration_secretstore-validate.yml"
	controllerDeploymentAssetName               = "external-secrets/resources/deployment_external-secrets.yml"
	bitwardenDeploymentAssetName                = "external-secrets/resources/deployment_bitwarden-sdk-server.yml"
	certControllerDeploymentAssetName           = "external-secrets/resources/deployment_external-secrets-cert-controller.yml"
	webhookDeploymentAssetName                  = "external-secrets/resources/deployment_external-secrets-webhook.yml"
	controllerServiceAccountAssetName           = "external-secrets/resources/serviceaccount_external-secrets.yml"
	bitwardenServiceAccountAssetName            = "external-secrets/resources/serviceaccount_bitwarden-sdk-server.yml"
	certControllerServiceAccountAssetName       = "external-secrets/resources/serviceaccount_external-secrets-cert-controller.yml"
	webhookServiceAccountAssetName              = "external-secrets/resources/serviceaccount_external-secrets-webhook.yml"
	webhookServiceAssetName                     = "external-secrets/resources/service_external-secrets-webhook.yml"
	bitwardenServiceAssetName                   = "external-secrets/resources/service_bitwarden-sdk-server.yml"
)
