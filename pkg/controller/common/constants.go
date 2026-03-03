package common

import (
	"os"
	"regexp"
	"time"
)

const (
	// DefaultRequeueTime is the default reconcile requeue time.
	DefaultRequeueTime = time.Second * 30

	// ExternalSecretsConfigObjectName is the default name of the externalsecretsconfigs.operator.openshift.io CR.
	ExternalSecretsConfigObjectName = "cluster"

	// ExternalSecretsManagerObjectName is the default name of the externalsecretsmanagers.operator.openshift.io CR.
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

	// ManagedAnnotationsKey is the annotation key used to track which annotation keys
	// are managed by the operator. The value is a base64-encoded JSON array of annotation keys.
	ManagedAnnotationsKey = "externalsecretsconfig.operator.openshift.io/managed-annotations"
)

var (
	ExternalSecretsOperatorVersion = os.Getenv("OPERATOR_IMAGE_VERSION")

	// DisallowedLabelMatcher restricts labels that cannot be applied to managed resources.
	// Matches app.kubernetes.io/, external-secrets.io/, rbac.authorization.k8s.io/,
	// servicebinding.io/controller, and app labels.
	DisallowedLabelMatcher = regexp.MustCompile(`^app.kubernetes.io\/|^external-secrets.io\/|^rbac.authorization.k8s.io\/|^servicebinding.io\/controller$|^app$`)
)
