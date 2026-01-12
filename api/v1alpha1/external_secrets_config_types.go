package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func init() {
	SchemeBuilder.Register(&ExternalSecretsConfig{}, &ExternalSecretsConfigList{})
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true

// ExternalSecretsConfigList is a list of ExternalSecretsConfig objects.
type ExternalSecretsConfigList struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is the standard list's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	metav1.ListMeta `json:"metadata"`
	Items           []ExternalSecretsConfig `json:"items"`
}

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=externalsecretsconfigs,scope=Cluster,categories={external-secrets-operator, external-secrets},shortName=esc;externalsecretsconfig;esconfig
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="Message",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].message"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:metadata:labels={"app.kubernetes.io/name=externalsecretsconfig", "app.kubernetes.io/part-of=external-secrets-operator"}

// ExternalSecretsConfig describes configuration and information about the managed external-secrets deployment.
// The name must be `cluster` as ExternalSecretsConfig is a singleton, allowing only one instance per cluster.
//
// When an ExternalSecretsConfig is created, the controller installs the external-secrets and keeps it in the desired state.
//
// +kubebuilder:validation:XValidation:rule="self.metadata.name == 'cluster'",message="ExternalSecretsConfig is a singleton, .metadata.name must be 'cluster'"
// +operator-sdk:csv:customresourcedefinitions:displayName="ExternalSecretsConfig"
type ExternalSecretsConfig struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is the standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// spec is the specification of the desired behavior of the ExternalSecretsConfig.
	Spec ExternalSecretsConfigSpec `json:"spec,omitempty"`

	// status is the most recently observed status of the ExternalSecretsConfig.
	Status ExternalSecretsConfigStatus `json:"status,omitempty"`
}

// ExternalSecretsConfigSpec is for configuring the external-secrets operand behavior.
// +kubebuilder:validation:XValidation:rule="!has(self.plugins) || !has(self.plugins.bitwardenSecretManagerProvider) || !has(self.plugins.bitwardenSecretManagerProvider.mode) || self.plugins.bitwardenSecretManagerProvider.mode != 'Enabled' || has(self.plugins.bitwardenSecretManagerProvider.secretRef) || (has(self.controllerConfig) && has(self.controllerConfig.certProvider) && has(self.controllerConfig.certProvider.certManager) && has(self.controllerConfig.certProvider.certManager.mode) && self.controllerConfig.certProvider.certManager.mode == 'Enabled')",message="secretRef or certManager must be configured when bitwardenSecretManagerProvider plugin is enabled"
type ExternalSecretsConfigSpec struct {
	// appConfig is for specifying the configurations for the `external-secrets` operand.
	// +kubebuilder:validation:Optional
	ApplicationConfig ApplicationConfig `json:"appConfig,omitempty"`

	// plugins is for configuring the optional provider plugins.
	// +kubebuilder:validation:Optional
	Plugins PluginsConfig `json:"plugins,omitempty"`

	// controllerConfig is for specifying the configurations for the controller to use while installing the `external-secrets` operand and the plugins.
	// +kubebuilder:validation:Optional
	ControllerConfig ControllerConfig `json:"controllerConfig,omitempty"`
}

// ExternalSecretsConfigStatus is the most recently observed status of the ExternalSecretsConfig.
type ExternalSecretsConfigStatus struct {
	// conditions holds information of the current state of the external-secrets deployment.
	ConditionalStatus `json:",inline"`

	// externalSecretsImage is the name of the image and the tag used for deploying external-secrets.
	ExternalSecretsImage string `json:"externalSecretsImage,omitempty"`

	// BitwardenSDKServerImage is the name of the image and the tag used for deploying bitwarden-sdk-server.
	BitwardenSDKServerImage string `json:"bitwardenSDKServerImage,omitempty"`
}

// ApplicationConfig is for specifying the configurations for the external-secrets operand.
type ApplicationConfig struct {
	// operatingNamespace is for restricting the external-secrets operations to the provided namespace.
	// When configured `ClusterSecretStore` and `ClusterExternalSecret` are implicitly disabled.
	// +kubebuilder:validation:MinLength:=1
	// +kubebuilder:validation:MaxLength:=63
	// +kubebuilder:validation:Optional
	OperatingNamespace string `json:"operatingNamespace,omitempty"`

	// webhookConfig is for configuring external-secrets webhook specifics.
	// +kubebuilder:validation:Optional
	WebhookConfig *WebhookConfig `json:"webhookConfig,omitempty"`

	// +kubebuilder:validation:Optional
	CommonConfigs `json:",inline"`
}

// ControllerConfig is for specifying the configurations for the controller to use while installing the `external-secrets` operand and the plugins.
type ControllerConfig struct {
	// certProvider is for defining the configuration for certificate providers used to manage TLS certificates for webhook and plugins.
	// +kubebuilder:validation:Optional
	CertProvider *CertProvidersConfig `json:"certProvider,omitempty"`

	// labels to apply to all resources created for the external-secrets operand deployment.
	// This field can have a maximum of 20 entries.
	// +mapType=granular
	// +kubebuilder:validation:MinProperties:=0
	// +kubebuilder:validation:MaxProperties:=20
	// +kubebuilder:validation:Optional
	Labels map[string]string `json:"labels,omitempty"`

	// annotations are for adding custom annotations to all the resources created for external-secrets deployment. The annotations are merged with any default annotations set by the operator. User-specified annotations takes precedence over defaults in case of conflicts. Annotation keys with prefixes `kubernetes.io/`, `app.kubernetes.io/`, `openshift.io/`, or `k8s.io/` are not allowed
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:MaxItems:=20
	// +kubebuilder:validation:XValidation:rule="self.all(a, !['kubernetes.io/', 'app.kubernetes.io/', 'openshift.io/', 'k8s.io/'].exists(p, a.key.startsWith(p)))",message="annotations with reserved prefixes 'kubernetes.io/', 'app.kubernetes.io/', 'openshift.io/', 'k8s.io/' are not allowed"
	// +listType=map
	// +listMapKey=key
	// +optional
	Annotations []Annotation `json:"annotations,omitempty"`

	// networkPolicies specifies the list of network policy configurations
	// to be applied to external-secrets pods.
	//
	// Each entry allows specifying a name for the generated NetworkPolicy object,
	// along with its full Kubernetes NetworkPolicy definition.
	//
	// If this field is not provided, external-secrets components will be isolated
	// with deny-all network policies, which will prevent proper operation.
	//
	// +kubebuilder:validation:XValidation:rule="oldSelf.all(op, self.exists(p, p.name == op.name && p.componentName == op.componentName))",message="name and componentName fields in networkPolicies are immutable"
	// +kubebuilder:validation:MinItems:=0
	// +kubebuilder:validation:MaxItems:=50
	// +kubebuilder:validation:Optional
	// +listType=map
	// +listMapKey=name
	// +listMapKey=componentName
	NetworkPolicies []NetworkPolicy `json:"networkPolicies,omitempty"`

	// componentConfigs allows specifying deployment-level configuration overrides for individual external-secrets components. This field enables fine-grained control over deployment settings such as revisionHistoryLimit for each component independently.Valid component names: ExternalSecretsCoreController, Webhook, CertController, BitwardenSDKServer.
	// +kubebuilder:validation:MinItems:=0
	// +kubebuilder:validation:MaxItems:=4
	// +kubebuilder:validation:Optional
	// +listType=map
	// +listMapKey=componentName
	ComponentConfigs []ComponentConfig `json:"componentConfigs,omitempty"`
}

type ComponentConfig struct {
	// componentName specifies which deployment component this configuration applies to.
	// Allowed values: ExternalSecretsCoreController, Webhook, CertController, BitwardenSDKServer
	// +kubebuilder:validation:Enum:=ExternalSecretsCoreController;Webhook;CertController;BitwardenSDKServer
	// +kubebuilder:validation:Required
	ComponentName ComponentName `json:"componentName"`

	// deploymentConfigs allows for component-specific overrides of the Kubernetes Deployment resource properties.
	// +kubebuilder:validation:Optional
	// +optional
	DeploymentConfigs DeploymentConfig `json:"deploymentConfigs,omitempty"`

	// overrideEnv specifies custom environment variables for a specific component's container. These are merged with operator-defaults, with user-defined keys taking precedence. Keys starting with 'HOSTNAME', 'KUBERNETES_', or 'EXTERNAL_SECRETS_' are reserved and will be rejected.
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:MaxItems:=50
	// +kubebuilder:validation:XValidation:rule="self.all(e, !['HOSTNAME', 'KUBERNETES_', 'EXTERNAL_SECRETS_'].exists(p, e.name.startsWith(p)))",message="Environment variable names with reserved prefixes 'HOSTNAME', 'KUBERNETES_', 'EXTERNAL_SECRETS_' are not allowed"
	// +optional
	OverrideEnv []corev1.EnvVar `json:"overrideEnv,omitempty"`
}
type DeploymentConfig struct {
	// revisionHistoryLimit specifies the number of old ReplicaSets to retain for rollback purposes. This controls how many previous deployment versions are kept in the cluster, allowing you to rollback to recent versions.
	// Minimum value of 1 is enforced to ensure at least one rollback is possible.using 'kubectl rollout undo'.
	// If not specified, Kubernetes default of 10 is used.
	// +kubebuilder:default:=10
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Optional
	// +optional
	RevisionHistoryLimit *int32 `json:"revisionHistoryLimit,omitempty"`
}

// KVPair represents a generic key-value pair for configuration.
type KVPair struct {
	// +kubebuilder:validation:MinLength:=1
	// +kubebuilder:validation:MaxLength:=253
	// +kubebuilder:validation:Required
	Key string `json:"key"`

	// +kubebuilder:validation:Optional
	Value string `json:"value,omitempty"`
}

// Annotation represents a custom annotation key-value pair.
// +kubebuilder:validation:XValidation:rule="self.key.matches('^([a-z0-9]([-a-z0-9]*[a-z0-9])?(\\\\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*\\\\/)?([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]$')",message="Annotation key must consist of an optional DNS subdomain prefix followed by '/', and a name. The name must begin and end with an alphanumeric character and contain only alphanumeric characters, dots (.), dashes (-), and underscores (_)."
type Annotation struct {
	KVPair `json:",inline"`
}

// BitwardenSecretManagerProvider is for enabling the bitwarden secrets manager provider and for setting up the additional service required for connecting with the bitwarden server.
type BitwardenSecretManagerProvider struct {
	// mode indicates bitwarden secrets manager provider state, which can be indicated by setting Enabled or Disabled.
	// Enabled: Enables the Bitwarden provider plugin. The operator will ensure the plugin is deployed and its state is synchronized.
	// Disabled: Disables reconciliation of the Bitwarden provider plugin. The plugin and its resources will remain in their current state and will not be managed by the operator.
	// +kubebuilder:validation:Enum:=Enabled;Disabled
	// +kubebuilder:default:=Disabled
	// +kubebuilder:validation:Optional
	Mode Mode `json:"mode,omitempty"`

	// SecretRef is the Kubernetes secret containing the TLS key pair to be used for the bitwarden server.
	// The issuer in CertManagerConfig will be utilized to generate the required certificate if the secret reference is not provided and CertManagerConfig is configured.
	// The key names in secret for certificate must be `tls.crt`, for private key must be `tls.key` and for CA certificate key name must be `ca.crt`.
	// +kubebuilder:validation:Optional
	SecretRef *SecretReference `json:"secretRef,omitempty"`
}

// WebhookConfig is for configuring external-secrets webhook specifics.
type WebhookConfig struct {
	// CertificateCheckInterval is for configuring the polling interval to check the certificate validity.
	// +kubebuilder:default:="5m"
	// +kubebuilder:validation:Optional
	CertificateCheckInterval *metav1.Duration `json:"certificateCheckInterval,omitempty"`
}

// CertManagerConfig is for configuring cert-manager specifics.
// +kubebuilder:validation:XValidation:rule="self.mode != 'Enabled' || has(self.issuerRef)",message="issuerRef must be provided when mode is set to Enabled."
// +kubebuilder:validation:XValidation:rule="has(self.injectAnnotations) && self.injectAnnotations != 'false' ? self.mode != 'Disabled' : true",message="injectAnnotations can only be set when mode is set to Enabled."
type CertManagerConfig struct {
	// mode indicates whether to use cert-manager for certificate management, instead of built-in cert-controller.
	// Enabled: Makes use of cert-manager for obtaining the certificates for webhook server and other components.
	// Disabled: Makes use of in-built cert-controller for obtaining the certificates for webhook server, which is the default behavior.
	// This field is immutable once set.
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="mode is immutable once set"
	// +kubebuilder:validation:Enum:=Enabled;Disabled
	// +kubebuilder:validation:Required
	Mode Mode `json:"mode,omitempty"`

	// injectAnnotations is for adding the `cert-manager.io/inject-ca-from` annotation to the webhooks and CRDs to automatically setup webhook to use the cert-manager CA. This requires CA Injector to be enabled in cert-manager.
	// Use `true` or `false` to indicate the preference. This field is immutable once set.
	// +kubebuilder:validation:Enum:="true";"false"
	// +kubebuilder:default:="false"
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="injectAnnotations is immutable once set"
	// +kubebuilder:validation:Optional
	InjectAnnotations string `json:"injectAnnotations,omitempty"`

	// issuerRef contains details of the referenced object used for obtaining certificates.
	// When `issuerRef.Kind` is `Issuer`, it must exist in the `external-secrets` namespace.
	// This field is immutable once set.
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="issuerRef is immutable once set"
	// +kubebuilder:validation:XValidation:rule="!has(self.kind) || self.kind.lowerAscii() == 'issuer' || self.kind.lowerAscii() == 'clusterissuer'",message="kind must be either 'Issuer' or 'ClusterIssuer'"
	// +kubebuilder:validation:XValidation:rule="!has(self.group) || self.group.lowerAscii() == 'cert-manager.io'",message="group must be 'cert-manager.io'"
	// +kubebuilder:validation:Optional
	IssuerRef *ObjectReference `json:"issuerRef,omitempty"`

	// certificateDuration is the validity period of the webhook certificate.
	// +kubebuilder:default:="8760h"
	// +kubebuilder:validation:Optional
	CertificateDuration *metav1.Duration `json:"certificateDuration,omitempty"`

	// certificateRenewBefore is the ahead time to renew the webhook certificate before expiry.
	// +kubebuilder:default:="30m"
	// +kubebuilder:validation:Optional
	CertificateRenewBefore *metav1.Duration `json:"certificateRenewBefore,omitempty"`
}

// PluginsConfig is for configuring the optional plugins.
type PluginsConfig struct {
	// bitwardenSecretManagerProvider is for enabling the bitwarden secrets manager provider plugin for connecting with the bitwarden secrets manager.
	// +kubebuilder:validation:Optional
	BitwardenSecretManagerProvider *BitwardenSecretManagerProvider `json:"bitwardenSecretManagerProvider,omitempty"`
}

// CertProvidersConfig defines the configuration for certificate providers used to manage TLS certificates for webhook and plugins.
type CertProvidersConfig struct {
	// certManager is for configuring cert-manager provider specifics.
	// +kubebuilder:validation:Optional
	CertManager *CertManagerConfig `json:"certManager,omitempty"`
}

// ComponentName represents the different external-secrets components that can have network policies applied.
type ComponentName string

const (
	// CoreController represents the external-secrets component
	CoreController ComponentName = "ExternalSecretsCoreController"

	// BitwardenSDKServer represents the bitwarden-sdk-server component
	BitwardenSDKServer ComponentName = "BitwardenSDKServer"

	// Webhook represents the external-secrets webhook component
	Webhook ComponentName = "Webhook"

	// CertController represents the cert-controller component
	CertController ComponentName = "CertController"
)

// NetworkPolicy represents a custom network policy configuration for operator-managed components.
// It includes a name for identification and the network policy rules to be enforced.
type NetworkPolicy struct {
	// name is a unique identifier for this network policy configuration.
	// This name will be used as part of the generated NetworkPolicy resource name.
	// +kubebuilder:validation:MinLength:=1
	// +kubebuilder:validation:MaxLength:=253
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// componentName specifies which external-secrets component this network policy applies to.
	// +kubebuilder:validation:Enum:=ExternalSecretsCoreController;BitwardenSDKServer
	// +kubebuilder:validation:Required
	ComponentName ComponentName `json:"componentName"`

	// egress is a list of egress rules to be applied to the selected pods. Outgoing traffic
	// is allowed if there are no NetworkPolicies selecting the pod (and cluster policy
	// otherwise allows the traffic), OR if the traffic matches at least one egress rule
	// across all the NetworkPolicy objects whose podSelector matches the pod. If
	// this field is empty then this NetworkPolicy limits all outgoing traffic (and serves
	// solely to ensure that the pods it selects are isolated by default).
	// The operator will automatically handle ingress rules based on the current running ports.
	// +kubebuilder:validation:Required
	//+listType=atomic
	Egress []networkingv1.NetworkPolicyEgressRule `json:"egress,omitempty" protobuf:"bytes,3,rep,name=egress"`
}
