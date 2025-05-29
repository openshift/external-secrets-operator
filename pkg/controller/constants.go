package controller

import (
	"os"
	"strings"
	"time"

	certmanagerapi "github.com/cert-manager/cert-manager/pkg/apis/certmanager"
	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
)

const (
	// externalsecretsOperatorCommonName is the name commonly used for labelling resources.
	externalsecretsOperatorCommonName = "external-secrets-operator"

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

	// externalsecretsImageVersionEnvVarName is the environment variable key name
	// containing the image version of the external-secrets operand as value.
	externalsecretsImageVersionEnvVarName = "OPERAND_EXTERNAL_SECRETS_IMAGE_VERSION"
)

var (
	// controllerDefaultResourceLabels is default set of labels added to all resources
	// created for external-secrets deployment.
	controllerDefaultResourceLabels = map[string]string{
		"app":                          externalsecretsCommonName,
		"app.kubernetes.io/name":       externalsecretsCommonName,
		"app.kubernetes.io/instance":   externalsecretsCommonName,
		"app.kubernetes.io/version":    os.Getenv(externalsecretsImageVersionEnvVarName),
		"app.kubernetes.io/managed-by": externalsecretsOperatorCommonName,
		"app.kubernetes.io/part-of":    externalsecretsOperatorCommonName,
	}
)

// asset names are the files present in the root `bindata/` dir, which are then loaded to
// and made available by the pkg/operator/assets package.
const (
	validatingWebhookExternalSecretCRDAssetName = "external-secrets/resources/validatingwebhookconfiguration_externalsecret-validate.yml"
	validatingWebhookSecretStoreCRDAssetName    = "external-secrets/resources/validatingwebhookconfiguration_secretstore-validate.yml"
	webhookTLSSecretAssetName                   = "external-secrets/resources/secret_external-secrets-webhook.yml"
	webhookCertificateAssetName                 = "external-secrets/resources/certificate_external-secrets-webhook.yml"
)

var (
	clusterIssuerKind = strings.ToLower(certmanagerv1.ClusterIssuerKind)
	issuerKind        = strings.ToLower(certmanagerv1.IssuerKind)
	issuerGroup       = strings.ToLower(certmanagerapi.GroupName)
)
