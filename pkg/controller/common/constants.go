package common

import (
	"os"
	"time"
)

const (
	// DefaultRequeueTime is the default reconcile requeue time.
	DefaultRequeueTime = time.Second * 30

	// ExternalSecretsConfigObjectName is the default name of the externalsecretsconfig.openshift.operator.io CR.
	ExternalSecretsConfigObjectName = "cluster"

	// ExternalSecretsManagerObjectName is the default name of the externalsecretsmanager.openshift.operator.io CR.
	ExternalSecretsManagerObjectName = "cluster"

	// CertManagerInjectCAFromAnnotation is the annotation key added to external-secrets resource once
	// if certManager field is enabled in webhook config
	// after successful reconciliation by the controller.
	CertManagerInjectCAFromAnnotation = "cert-manager.io/inject-ca-from"

	// CertManagerInjectCAFromAnnotationValue is the annotation value added to external-secrets resource once
	// if certManager field is enabled in webhook config
	// after successful reconciliation by the controller.
	CertManagerInjectCAFromAnnotationValue = "external-secrets/external-secrets-webhook"

	// ExternalSecretsOperatorCommonName is the name commonly used for labelling resources.
	ExternalSecretsOperatorCommonName = "external-secrets-operator"
)

var (
	ExternalSecretsOperatorVersion = os.Getenv("OPERATOR_IMAGE_VERSION")
)
