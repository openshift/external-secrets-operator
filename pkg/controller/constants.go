package controller

import "time"

const (
	// externalsecretsCommonName is the name commonly used for naming resources.
	externalsecretsCommonName = "external-secrets"

	// ControllerName is the name of the controller used in logs and events.
	ControllerName = externalsecretsCommonName + "-controller"

	// externalsecretsNamespaceMappingLabelName is the label name for identifying the cluster resources or resources
	// created in other namespaces by the controller.
	externalsecretsNamespaceMappingLabelName = "external-secrets-namespace"

	// externalsecretsResourceWatchLabelName is the label name for identifying the resources of interest for the
	// controller but does not create or manage the resource.
	externalsecretsResourceWatchLabelName = "external-secrets.openshift.operator.io/watched-by"

	externalsecretsObjectName = "default"

	// defaultRequeueTime is the default reconcile requeue time.
	defaultRequeueTime = time.Second * 30

	// finalizer name for external-secrets.openshift.operator.io resource.
	finalizer = "external-secrets.openshift.operator.io/" + ControllerName

	// controllerProcessedAnnotation is the annotation added to external-secrets resource once after
	// successful reconciliation by the controller.
	controllerProcessedAnnotation = "operator.openshift.io/external-secrets-processed"
)
