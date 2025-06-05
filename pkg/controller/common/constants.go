package common

import (
	"time"
)

const (
	// DefaultRequeueTime is the default reconcile requeue time.
	DefaultRequeueTime = time.Second * 30

	// ExternalSecretsObjectName is the default name of the externalsecrets.openshift.operator.io CR.
	ExternalSecretsObjectName = "cluster"

	// CertManagerInjectCAFromAnnotation is the annotation key added to external-secrets resource once
	// if certManager field is enabled in webhook config
	// after successful reconciliation by the controller.
	CertManagerInjectCAFromAnnotation = "cert-manager.io/inject-ca-from"

	// CertManagerInjectCAFromAnnotationValue is the annotation value added to external-secrets resource once
	// if certManager field is enabled in webhook config
	// after successful reconciliation by the controller.
	CertManagerInjectCAFromAnnotationValue = "external-secrets/external-secrets-webhook"
)
