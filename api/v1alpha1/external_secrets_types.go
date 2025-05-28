package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func init() {
	SchemeBuilder.Register(&ExternalSecrets{}, &ExternalSecretsList{})
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
//+kubebuilder:object:root=true

// ExternalSecretsList is a list of ExternalSecrets objects.
type ExternalSecretsList struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is the standard list's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	metav1.ListMeta `json:"metadata"`
	Items           []ExternalSecrets `json:"items"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster

// ExternalSecrets describes configuration and information about the managed external-secrets
// deployment. The name must be `cluster` as ExternalSecrets is a singleton,
// allowing only one instance per cluster.
//
// When an ExternalSecrets is created, a new deployment is created which manages the
// external-secrets and keeps it in the desired state.
//
// +kubebuilder:validation:XValidation:rule="self.metadata.name == 'cluster'",message="ExternalSecrets is a singleton, .metadata.name must be 'cluster'"
// +operator-sdk:csv:customresourcedefinitions:displayName="ExternalSecrets"
type ExternalSecrets struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is the standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// spec is the specification of the desired behavior of the ExternalSecrets.
	Spec ExternalSecretsSpec `json:"spec,omitempty"`

	// status is the most recently observed status of the ExternalSecrets.
	Status ExternalSecretsStatus `json:"status,omitempty"`
}

// ExternalSecretsSpec is the specification of the desired behavior of the ExternalSecrets.
type ExternalSecretsSpec struct {
	// externalSecretsConfig is for configuring the external-secrets behavior.
	// +kubebuilder:validation:Optional
	ExternalSecretsConfig *ExternalSecretsConfig `json:"externalSecretsConfig,omitempty"`

	// controllerConfig is for configuring the controller for setting up
	// defaults to enable external-secrets.
	// +kubebuilder:validation:Optional
	ControllerConfig *ControllerConfig `json:"controllerConfig,omitempty"`
}

// ExternalSecretsStatus is the most recently observed status of the ExternalSecrets.
type ExternalSecretsStatus struct {
	// conditions holds information of the current state of the external-secrets deployment.
	ConditionalStatus `json:",inline,omitempty"`

	// externalSecretsImage is the name of the image and the tag used for deploying external-secrets.
	ExternalSecretsImage string `json:"externalSecretsImage,omitempty"`
}

// ExternalSecretsConfig is for configuring the external-secrets behavior.
type ExternalSecretsConfig struct {
	// logLevel supports value range as per [kubernetes logging guidelines](https://github.com/kubernetes/community/blob/master/contributors/devel/sig-instrumentation/logging.md#what-method-to-use).
	// +kubebuilder:default:=1
	// +kubebuilder:validation:Minimum:=1
	// +kubebuilder:validation:Maximum:=5
	// +kubebuilder:validation:Optional
	LogLevel int32 `json:"logLevel,omitempty"`

	// bitwardenSecretManagerProvider is for enabling the bitwarden secrets manager provider and
	// for setting up the additional service required for connecting with the bitwarden server.
	// +kubebuilder:validation:Optional
	BitwardenSecretManagerProvider *BitwardenSecretManagerProvider `json:"bitwardenSecretManagerProvider,omitempty"`

	// operatingNamespace is for restricting the external-secrets operations to provided namespace.
	// And when enabled `ClusterSecretStore` and `ClusterExternalSecret` are implicitly disabled.
	// +kubebuilder:validation:Optional
	OperatingNamespace string `json:"operatingNamespace,omitempty"`

	// webhookConfig is for configuring external-secrets webhook specifics.
	WebhookConfig *WebhookConfig `json:"webhookConfig,omitempty"`

	// resources is for defining the resource requirements.
	// Cannot be updated.
	// ref: https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/
	// +kubebuilder:validation:Optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`

	// affinity is for setting scheduling affinity rules.
	// ref: https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/
	// +kubebuilder:validation:Optional
	Affinity *corev1.Affinity `json:"affinity,omitempty"`

	// tolerations is for setting the pod tolerations.
	// ref: https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/
	// +kubebuilder:validation:Optional
	// +listType=atomic
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`

	// nodeSelector is for defining the scheduling criteria using node labels.
	// ref: https://kubernetes.io/docs/concepts/configuration/assign-pod-node/
	// +kubebuilder:validation:Optional
	// +mapType=atomic
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`
}

// ControllerConfig is for configuring the operator for setting up
// defaults to install external-secrets.
type ControllerConfig struct {
	// namespace is for configuring the namespace to install the external-secret operand.
	// +kubebuilder:validation:Optional
	// +kubebuilder:default:="external-secrets"
	Namespace string `json:"namespace,omitempty"`

	// labels to apply to all resources created for external-secrets deployment.
	// +mapType=granular
	// +kubebuilder:validation:Optional
	Labels map[string]string `json:"labels,omitempty"`
}

// BitwardenSecretManagerProvider is for enabling the bitwarden secrets manager provider and
// for setting up the additional service required for connecting with the bitwarden server.
type BitwardenSecretManagerProvider struct {
	// enabled is for enabling the bitwarden secrets manager provider, which can be indicated
	// by setting `true` or `false`.
	// +kubebuilder:default:="false"
	// +kubebuilder:validation:Enum:="true";"false"
	// +kubebuilder:validation:Optional
	Enabled string `json:"enabled,omitempty"`
}

// WebhookConfig is for configuring external-secrets webhook specifics.
type WebhookConfig struct {
	// CertificateCheckInterval is for configuring the polling interval to check the certificate
	// validity.
	// +kubebuilder:default:="5m"
	// +kubebuilder:validation:Optional
	CertificateCheckInterval metav1.Duration `json:"certificateCheckInterval,omitempty"`

	// CertManagerConfig is for configuring cert-manager specifics.
	// +kubebuilder:validation:Optional
	CertManagerConfig *CertManagerConfig `json:"certManagerConfig,omitempty"`
}

// CertManagerConfig is for configuring cert-manager specifics.
// +kubebuilder:validation:XValidation:rule="!has(oldSelf.issuerRef) && !has(self.issuerRef) || has(oldSelf.issuerRef) && has(self.issuerRef)",message="issuerRef may only be configured during creation"
type CertManagerConfig struct {
	// enabled is for enabling the use of cert-manager for obtaining and renewing the
	// certificates used for webhook server, instead of built-in certificates.
	// Use `true` or `false` to indicate the preference.
	// +kubebuilder:default:="false"
	// +kubebuilder:validation:Enum:="true";"false"
	// +kubebuilder:validation:Required
	Enabled string `json:"enabled,omitempty"`

	// addInjectorAnnotations is for adding the `cert-manager.io/inject-ca-from` annotation to the
	// webhooks and CRDs to automatically setup webhook to the cert-manager CA. This requires
	// CA Injector to be enabled in cert-manager. Use `true` or `false` to indicate the preference.
	// +kubebuilder:default:="false"
	// +kubebuilder:validation:Enum:="true";"false"
	// +kubebuilder:validation:Optional
	AddInjectorAnnotations string `json:"addInjectorAnnotations,omitempty"`

	// issuerRef contains details to the referenced object used for
	// obtaining the certificates. It must exist in the external-secrets
	// namespace if not using a cluster-scoped cert-manager issuer.
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="issuerRef is immutable once set"
	// +kubebuilder:validation:Required
	IssuerRef ObjectReference `json:"issuerRef,omitempty"`

	// certificateDuration is the validity period of the webhook certificate.
	// +kubebuilder:default:="8760h"
	// +kubebuilder:validation:Optional
	CertificateDuration *metav1.Duration `json:"certificateDuration,omitempty"`

	// certificateRenewBefore is the ahead time to renew the webhook certificate
	// before expiry.
	// +kubebuilder:validation:Optional
	CertificateRenewBefore *metav1.Duration `json:"certificateRenewBefore,omitempty"`
}
