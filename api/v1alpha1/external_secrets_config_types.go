package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func init() {
	SchemeBuilder.Register(&ExternalSecretsConfig{}, &ExternalSecretsConfigList{})
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
//+kubebuilder:object:root=true

// ExternalSecretsConfigList is a list of ExternalSecretsConfig objects.
type ExternalSecretsConfigList struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is the standard list's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	metav1.ListMeta `json:"metadata"`
	Items           []ExternalSecretsConfig `json:"items"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=externalsecretsconfigs,scope=Cluster,categories={external-secrets-operator, external-secrets},shortName=esc;externalsecretsconfig;esconfig
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="Message",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].message"
// +kubebuilder:metadata:labels={"app.kubernetes.io/name=externalsecretsconfig", "app.kubernetes.io/part-of=external-secrets-operator"}

// ExternalSecretsConfig describes configuration and information about the managed external-secrets
// deployment. The name must be `cluster` as ExternalSecretsConfig is a singleton,
// allowing only one instance per cluster.
//
// When an ExternalSecretsConfig is created, a new deployment is created which manages the
// external-secrets and keeps it in the desired state.
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
type ExternalSecretsConfigSpec struct {
	// namespace is for configuring the namespace to install the external-secret operand.
	// +kubebuilder:validation:Optional
	// +kubebuilder:default:="external-secrets"
	// +kubebuilder:validation:MinLength:=1
	// +kubebuilder:validation:MaxLength:=63
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="namespace is immutable once set"
	Namespace string `json:"namespace,omitempty"`

	// operatingNamespace is for restricting the external-secrets operations to provided namespace.
	// And when enabled `ClusterSecretStore` and `ClusterExternalSecret` are implicitly disabled.
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:MinLength:=1
	// +kubebuilder:validation:MaxLength:=63
	OperatingNamespace string `json:"operatingNamespace,omitempty"`

	// bitwardenSecretManagerProvider is for enabling the bitwarden secrets manager provider and
	// for setting up the additional service required for connecting with the bitwarden server.
	// +kubebuilder:validation:Optional
	BitwardenSecretManagerProvider *BitwardenSecretManagerProvider `json:"bitwardenSecretManagerProvider,omitempty"`

	// webhookConfig is for configuring external-secrets webhook specifics.
	// +kubebuilder:validation:Optional
	WebhookConfig *WebhookConfig `json:"webhookConfig,omitempty"`

	// CertManagerConfig is for configuring cert-manager specifics, which will be used for generating
	// certificates for webhook and bitwarden-sdk-server components.
	// +kubebuilder:validation:Optional
	CertManagerConfig *CertManagerConfig `json:"certManagerConfig,omitempty"`

	// +kubebuilder:validation:Optional
	CommonConfigs `json:",inline,omitempty"`
}

// BitwardenSecretManagerProvider is for enabling the bitwarden secrets manager provider and
// for setting up the additional service required for connecting with the bitwarden server.
type BitwardenSecretManagerProvider struct {
	// enabled is for enabling the bitwarden secrets manager provider, which can be indicated
	// by setting `true` or `false`.
	// +kubebuilder:validation:Enum:="true";"false"
	// +kubebuilder:default:="false"
	// +kubebuilder:validation:Optional
	Enabled string `json:"enabled,omitempty"`

	// SecretRef is the kubernetes secret containing the TLS key pair to be used for the bitwarden server.
	// The issuer in CertManagerConfig will be utilized to generate the required certificate if the secret
	// reference is not provided and CertManagerConfig is configured. The key names in secret for certificate
	// must be `tls.crt`, for private key must be `tls.key` and for CA certificate key name must be `ca.crt`.
	// +kubebuilder:validation:Optional
	SecretRef *SecretReference `json:"secretRef,omitempty"`
}

// WebhookConfig is for configuring external-secrets webhook specifics.
type WebhookConfig struct {
	// CertificateCheckInterval is for configuring the polling interval to check the certificate
	// validity.
	// +kubebuilder:default:="5m"
	// +kubebuilder:validation:Optional
	CertificateCheckInterval *metav1.Duration `json:"certificateCheckInterval,omitempty"`
}

// CertManagerConfig is for configuring cert-manager specifics.
// +kubebuilder:validation:XValidation:rule="has(self.addInjectorAnnotations) && self.addInjectorAnnotations != 'false' ? self.enabled != 'false' : true",message="certManagerConfig must have enabled set, to set addInjectorAnnotations"
type CertManagerConfig struct {
	// enabled is for enabling the use of cert-manager for obtaining and renewing the
	// certificates used for webhook server, instead of built-in certificates.
	// Use `true` or `false` to indicate the preference.
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="enabled is immutable once set"
	// +kubebuilder:validation:Enum:="true";"false"
	// +kubebuilder:default:="false"
	// +kubebuilder:validation:Required
	Enabled string `json:"enabled,omitempty"`

	// addInjectorAnnotations is for adding the `cert-manager.io/inject-ca-from` annotation to the
	// webhooks and CRDs to automatically setup webhook to the cert-manager CA. This requires
	// CA Injector to be enabled in cert-manager. Use `true` or `false` to indicate the preference.
	// +kubebuilder:validation:Enum:="true";"false"
	// +kubebuilder:default:="false"
	// +kubebuilder:validation:Optional
	AddInjectorAnnotations string `json:"addInjectorAnnotations,omitempty"`

	// issuerRef contains details to the referenced object used for
	// obtaining the certificates. It must exist in the external-secrets
	// namespace if not using a cluster-scoped cert-manager issuer.
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="issuerRef is immutable once set"
	// +kubebuilder:validation:Required
	IssuerRef *ObjectReference `json:"issuerRef,omitempty"`

	// certificateDuration is the validity period of the webhook certificate.
	// +kubebuilder:default:="8760h"
	// +kubebuilder:validation:Optional
	CertificateDuration *metav1.Duration `json:"certificateDuration,omitempty"`

	// certificateRenewBefore is the ahead time to renew the webhook certificate
	// before expiry.
	// +kubebuilder:default:="30m"
	// +kubebuilder:validation:Optional
	CertificateRenewBefore *metav1.Duration `json:"certificateRenewBefore,omitempty"`
}

// ExternalSecretsConfigStatus is the most recently observed status of the ExternalSecretsConfig.
type ExternalSecretsConfigStatus struct {
	// conditions holds information of the current state of the external-secrets deployment.
	ConditionalStatus `json:",inline,omitempty"`

	// externalSecretsImage is the name of the image and the tag used for deploying external-secrets.
	ExternalSecretsImage string `json:"externalSecretsImage,omitempty"`

	// BitwardenSDKServerImage is the name of the image and the tag used for deploying bitwarden-sdk-server.
	BitwardenSDKServerImage string `json:"bitwardenSDKServerImage,omitempty"`
}
