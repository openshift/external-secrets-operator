package controller

import "time"

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
)

// asset names are the files present in the root `bindata/` dir, which are then loaded to
// and made available by the pkg/operator/assets package.
const (
	validatingWebhookExternalSecretCRDAssetName = "external-secrets/resources/validatingwebhookconfiguration_externalsecret-validate.yml"
	validatingWebhookSecretStoreCRDAssetName    = "external-secrets/resources/validatingwebhookconfiguration_secretstore-validate.yml"
)
