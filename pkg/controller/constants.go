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

	externalsecretsObjectName = "default"
)
