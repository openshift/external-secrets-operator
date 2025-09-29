package v1alpha1

import (
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

	// periodicReconcileInterval specifies the time interval in seconds for periodic reconciliation by the operator.
	// This controls how often the operator checks resources created for external-secrets operand to ensure they remain in desired state.
	// Interval can have value between 120-18000 seconds (2 minutes to 5 hours). Defaults to 300 seconds (5 minutes) if not specified.
	// +kubebuilder:default:=300
	// +kubebuilder:validation:Minimum:=120
	// +kubebuilder:validation:Maximum:=18000
	// +kubebuilder:validation:Optional
	PeriodicReconcileInterval uint32 `json:"periodicReconcileInterval,omitempty"`
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
	// +kubebuilder:default:=Disabled
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
	IssuerRef ObjectReference `json:"issuerRef,omitempty"`

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
