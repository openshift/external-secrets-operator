package external_secrets

import (
	"fmt"
	"os"

	certmanagerapi "github.com/cert-manager/cert-manager/pkg/apis/certmanager"
	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	operatorv1alpha1 "github.com/openshift/external-secrets-operator/api/v1alpha1"
	"github.com/openshift/external-secrets-operator/pkg/controller/common"
)

const (
	// externalsecretsCommonName is the name commonly used for naming resources.
	externalsecretsCommonName = "external-secrets"

	// ControllerName is the name of the controller used in logs and events.
	ControllerName = externalsecretsCommonName + "-controller"

	// OperandDefaultNamespace is the default namespace for the external-secrets operand.
	OperandDefaultNamespace = externalsecretsCommonName

	// OperandCoreControllerDeployment is the core external-secrets controller Deployment name.
	OperandCoreControllerDeployment = externalsecretsCommonName
	// OperandWebhookDeployment is the external-secrets webhook Deployment name.
	OperandWebhookDeployment = externalsecretsCommonName + "-webhook"
	// OperandCertControllerDeployment is the in-tree cert-controller Deployment name.
	OperandCertControllerDeployment = externalsecretsCommonName + "-cert-controller"
	// OperandBitwardenSDKServerDeployment is the bitwarden-sdk-server Deployment name.
	OperandBitwardenSDKServerDeployment = "bitwarden-sdk-server"

	// OperandCoreControllerContainer is the core controller container name.
	OperandCoreControllerContainer = externalsecretsCommonName
	// OperandWebhookContainer is the webhook container name.
	OperandWebhookContainer = "webhook"
	// OperandCertControllerContainer is the cert-controller container name.
	OperandCertControllerContainer = "cert-controller"

	// UnsafeAllowGenericTargetsArg is the core controller argument that enables generic target support.
	UnsafeAllowGenericTargetsArg = "--unsafe-allow-generic-targets=true"

	// LeaderElectionArg is injected into the core controller when replicas > 1 so that
	// only one replica actively reconciles at a time.
	LeaderElectionArg = "--enable-leader-election=true"

	// OperandBitwardenContainer is the bitwarden container name.
	OperandBitwardenContainer = "bitwarden-sdk-server"

	// OperandCoreControllerPodPrefix matches the core controller pod name prefix.
	OperandCoreControllerPodPrefix = OperandCoreControllerDeployment + "-"
	// OperandWebhookPodPrefix matches the webhook pod name prefix.
	OperandWebhookPodPrefix = OperandWebhookDeployment + "-"
	// OperandCertControllerPodPrefix matches the cert-controller pod name prefix.
	OperandCertControllerPodPrefix = OperandCertControllerDeployment + "-"

	// ManagedResourceLabelKey identifies operator-managed operand resources.
	// The controller uses this label on secondary watches to map event signals
	// back to the primary ExternalSecretsConfig owner instance.
	ManagedResourceLabelKey = "app"
	// ManagedResourceLabelValue is the label value paired with ManagedResourceLabelKey.
	ManagedResourceLabelValue = "external-secrets"
	// WatchedResourceLabelKey is applied to user-provided resources referenced in
	// ExternalSecretsConfig spec so changes enqueue reconciliation.
	WatchedResourceLabelKey = "externalsecretsconfig.operator.openshift.io/watching"
	// WatchedResourceLabelValue is paired with WatchedResourceLabelKey.
	WatchedResourceLabelValue = "true"

	// TrustedCABundleInjectLabel triggers OpenShift CNO to inject cluster-wide CA certificates.
	TrustedCABundleInjectLabel = "config.openshift.io/inject-trusted-cabundle"

	// ProxyTrustedCABundleConfigMapName is the operator-managed ConfigMap for proxy CA injection.
	ProxyTrustedCABundleConfigMapName = externalsecretsCommonName + "-trusted-ca-bundle"
	// ProxyTrustedCABundleVolumeName is the volume name for the proxy/CNO trusted CA bundle.
	ProxyTrustedCABundleVolumeName = "trusted-ca-bundle"
	// ProxyTrustedCABundleMountPath is where the proxy trusted CA bundle is mounted in containers.
	ProxyTrustedCABundleMountPath = "/etc/pki/tls/certs"

	// UserCABundleVolumeName is the volume name for a user-specified trustedCABundle ConfigMap.
	UserCABundleVolumeName = "user-ca-bundle"
	// UserCABundleMountPath is where the user CA bundle is mounted in the core controller.
	UserCABundleMountPath = "/etc/pki/tls/user-certs"
	// UserCABundleKeyPath is the ConfigMap data key and projected filename for the user CA bundle.
	UserCABundleKeyPath = "ca-bundle.crt"

	// SSLCertDirEnvVar tells Go's TLS library which directories to search for CA certificates.
	SSLCertDirEnvVar = "SSL_CERT_DIR"
	// SSLCertDirValue is set on the core controller when user trustedCABundle is configured.
	SSLCertDirValue = "/etc/pki/tls/user-certs:/etc/pki/tls/certs:/etc/ssl/certs"

	// finalizer name for externalsecretsconfigs.operator.openshift.io resource.
	finalizer = "externalsecretsconfigs.operator.openshift.io/" + ControllerName

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
	// containing the image name of the external-secrets as value.
	externalsecretsImageEnvVarName = "RELATED_IMAGE_EXTERNAL_SECRETS"

	// bitwardenImageEnvVarName is the environment variable key name
	// containing the image name of the bitwarden-sdk-server as value.
	bitwardenImageEnvVarName = "RELATED_IMAGE_BITWARDEN_SDK_SERVER"

	// bitwardenImageVersionEnvVarName is the environment variable key name
	// containing the image version of the bitwarden-sdk-server as value.
	bitwardenImageVersionEnvVarName = "BITWARDEN_SDK_SERVER_IMAGE_VERSION"

	// TODO: Remove in v1.4.0. Backported to 1.1/1.2 as a temporary escape hatch;
	// v1.3.0 adds ExternalSecretsConfig advancedOverrides for per-component Deployment
	// overrides and is the migration window before these env vars are removed.
	//
	// OperandExternalSecretsArgsEnvVar is the operator env var for core controller container args overrides.
	OperandExternalSecretsArgsEnvVar = "OPERAND_EXTERNAL_SECRETS_ARGS"
	// OperandWebhookArgsEnvVar is the operator env var for webhook container args overrides.
	OperandWebhookArgsEnvVar = "OPERAND_WEBHOOK_ARGS"
	// OperandCertControllerArgsEnvVar is the operator env var for cert-controller container args overrides.
	OperandCertControllerArgsEnvVar = "OPERAND_CERT_CONTROLLER_ARGS"
	// OperandBitwardenSDKServerArgsEnvVar is the operator env var for bitwarden-sdk-server container args overrides.
	OperandBitwardenSDKServerArgsEnvVar = "OPERAND_BITWARDEN_SDK_SERVER_ARGS"

	// certmanagerTLSSecretWebhook is the TLS secret created by cert-manager for the webhook component. A different
	// name is used to avoiding clash with the secret created by the inbuilt cert-controller component.
	certmanagerTLSSecretWebhook = "external-secrets-webhook-cm"

	// Proxy environment variable names (uppercase).
	httpProxyEnvVar  = "HTTP_PROXY"
	httpsProxyEnvVar = "HTTPS_PROXY"
	noProxyEnvVar    = "NO_PROXY"

	// Proxy environment variable names (lowercase) - required for compatibility with some applications.
	httpProxyEnvVarLowercase  = "http_proxy"
	httpsProxyEnvVarLowercase = "https_proxy"
	noProxyEnvVarLowercase    = "no_proxy"

	// systemNetworkPolicyPrefix is prepended to all operator-managed static network policy names.
	systemNetworkPolicyPrefix = "eso-sys-"

	// userNetworkPolicyPrefix is prepended to user-defined network policy names from the CR spec.
	userNetworkPolicyPrefix = "eso-user-"

	// TODO: Remove after 3 releases(in v1.5.0) once the migration from
	// unprefixed to eso-sys-/eso-user- network policy names is no longer needed.
	//
	// skipNPCleanupAnnotation marks that the one-time migration cleanup of unprefixed
	// network policies has already run, so subsequent reconciles can skip it.
	skipNPCleanupAnnotation = "externalsecretsconfig.operator.openshift.io/skip-np-cleanup-check"

	// proxyEgressPolicyName is the Kubernetes object name for the automatic proxy egress policy.
	proxyEgressPolicyName = systemNetworkPolicyPrefix + "allow-proxy-egress"
)

var (
	// certificateCRDGKV is the group.version/kind of the Certificate CRD.
	certificateCRDGKV = fmt.Sprintf("certificate.%s/%s", certmanagerv1.SchemeGroupVersion.Group, certmanagerv1.SchemeGroupVersion.Version)
)

var (
	// controllerDefaultResourceLabels is the default set of labels added to all resources
	// created for external-secrets deployment.
	controllerDefaultResourceLabels = map[string]string{
		"app":                          externalsecretsCommonName,
		"app.kubernetes.io/version":    os.Getenv(externalsecretsImageVersionEnvVarName),
		"app.kubernetes.io/managed-by": common.ExternalSecretsOperatorCommonName,
		"app.kubernetes.io/part-of":    common.ExternalSecretsOperatorCommonName,
	}

	// featureContainerArgs maps ExternalSecretsManager feature names to the container
	// argument appended when that feature is enabled and the deployment declares support
	// via updateOptionalFeatures. Only features listed in both ESM (enabled) and the
	// deployment's supportedFeatures slice are applied.
	featureContainerArgs = map[operatorv1alpha1.FeatureName]string{
		operatorv1alpha1.UnsafeAllowGenericTargets: UnsafeAllowGenericTargetsArg,
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
	metricsServiceAssetName                       = "external-secrets/resources/service_external-secrets-metrics.yml"
	certControllerMetricsServiceAssetName         = "external-secrets/resources/service_external-secrets-cert-controller-metrics.yml"
	controllerServiceAccountAssetName             = "external-secrets/resources/serviceaccount_external-secrets.yml"
	bitwardenServiceAccountAssetName              = "external-secrets/resources/serviceaccount_bitwarden-sdk-server.yml"
	certControllerServiceAccountAssetName         = "external-secrets/resources/serviceaccount_external-secrets-cert-controller.yml"
	webhookServiceAccountAssetName                = "external-secrets/resources/serviceaccount_external-secrets-webhook.yml"
	validatingWebhookExternalSecretCRDAssetName   = "external-secrets/resources/validatingwebhookconfiguration_externalsecret-validate.yml"
	validatingWebhookSecretStoreCRDAssetName      = "external-secrets/resources/validatingwebhookconfiguration_secretstore-validate.yml"
	denyAllNetworkPolicyAssetName                 = "external-secrets/networkpolicy_deny-all.yaml"
	allowMainControllerTrafficAssetName           = "external-secrets/networkpolicy_allow-api-server-egress-for-main-controller-traffic.yaml"
	allowWebhookTrafficAssetName                  = "external-secrets/networkpolicy_allow-api-server-and-webhook-traffic.yaml"
	allowCertControllerTrafficAssetName           = "external-secrets/networkpolicy_allow-api-server-egress-for-cert-controller-traffic.yaml"
	allowBitwardenServerTrafficAssetName          = "external-secrets/networkpolicy_allow-api-server-egress-for-bitwarden-sever.yaml"
	allowDNSTrafficAssetName                      = "external-secrets/networkpolicy_allow-dns.yaml"
)

var (
	clusterIssuerKind = certmanagerv1.ClusterIssuerKind
	issuerKind        = certmanagerv1.IssuerKind
	issuerGroup       = certmanagerapi.GroupName
)
