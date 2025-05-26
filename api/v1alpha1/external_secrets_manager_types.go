package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func init() {
	SchemeBuilder.Register(&ExternalSecretsManager{}, &ExternalSecretsManagerList{})
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
//+kubebuilder:object:root=true

// ExternalSecretsManagerList is a list of ExternalSecretsManager objects.
type ExternalSecretsManagerList struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is the standard list's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	metav1.ListMeta `json:"metadata"`
	Items           []ExternalSecretsManager `json:"items"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster

// ExternalSecretsManager describes configuration and information about the deployments managed by
// the external-secrets-operator. The name must be `cluster` as this is a singleton object allowing
// only one instance of ExternalSecretsManager per cluster.
//
// It is mainly for configuring the global options and enabling optional features, which
// serves as a common/centralized config for managing multiple controllers of the operator. The object
// is automatically created during the operator installation.
//
// +kubebuilder:validation:XValidation:rule="self.metadata.name == 'cluster'",message="ExternalSecretsManager is a singleton, .metadata.name must be 'cluster'"
// +operator-sdk:csv:customresourcedefinitions:displayName="ExternalSecretsManager"
type ExternalSecretsManager struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is the standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// spec is the specification of the desired behavior
	// +kubebuilder:validation:Required
	Spec ExternalSecretsManagerSpec `json:"spec,omitempty"`

	// status is the most recently observed status of controllers used by
	// External Secrets Operator.
	Status ExternalSecretsManagerStatus `json:"status,omitempty"`
}

// ExternalSecretsManagerSpec is the specification of the desired behavior of the ExternalSecretsManager.
type ExternalSecretsManagerSpec struct {
	// globalConfig is for configuring the behavior of deployments that are managed
	// by external secrets-operator.
	// +kubebuilder:validation:Optional
	GlobalConfig *GlobalConfig `json:"globalConfig,omitempty"`

	// features is for enabling the optional operator features.
	Features []Feature `json:"features,omitempty"`
}

// GlobalConfig is for configuring the external-secrets-operator behavior.
type GlobalConfig struct {
	// logLevel supports value range as per [kubernetes logging guidelines](https://github.com/kubernetes/community/blob/master/contributors/devel/sig-instrumentation/logging.md#what-method-to-use).
	// +kubebuilder:default:=1
	// +kubebuilder:validation:Minimum:=1
	// +kubebuilder:validation:Maximum:=5
	// +kubebuilder:validation:Optional
	LogLevel int32 `json:"logLevel,omitempty"`

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

	// labels to apply to all resources created for external-secrets deployment.
	// +mapType=granular
	// +kubebuilder:validation:Optional
	Labels map[string]string `json:"labels,omitempty"`
}

// Feature is for enabling the optional features.
// Feature is for enabling the optional features.
type Feature struct {
	// name of the optional feature.
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// enabled determines if feature should be turned on.
	// +kubebuilder:validation:Required
	Enabled bool `json:"enabled"`
}

// ExternalSecretsManagerStatus is the most recently observed status of the ExternalSecretsManager.
type ExternalSecretsManagerStatus struct {
	// conditions holds information of the current state of the external-secrets-operator controllers.
	ConditionalStatus `json:",inline,omitempty"`
}
