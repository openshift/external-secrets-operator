package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func init() {
	SchemeBuilder.Register(&ExternalSecretsOperator{}, &ExternalSecretsOperatorList{})
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
//+kubebuilder:object:root=true

// ExternalSecretsOperatorList is a list of ExternalSecretsOperator objects.
type ExternalSecretsOperatorList struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is the standard list's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	metav1.ListMeta `json:"metadata"`
	Items           []ExternalSecretsOperator `json:"items"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster

// ExternalSecretsOperator describes configuration and information about the managed external-secrets
// deployment. The name must be `cluster` to make ExternalSecretsOperator a singleton that is, to
// allow only one instance of ExternalSecretsOperator per cluster.
//
// ExternalSecretsOperator is mainly for configuring the global options and enabling the features, which
// serves as a common/centralized config for managing multiple controllers of the operator. The object
// will be created during the operator installation.
//
// +kubebuilder:validation:XValidation:rule="self.metadata.name == 'cluster'",message="ExternalSecretsOperator is a singleton, .metadata.name must be 'cluster'"
// +operator-sdk:csv:customresourcedefinitions:displayName="ExternalSecretsOperator"
type ExternalSecretsOperator struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is the standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// spec is the specification of the desired behavior of the ExternalSecretsOperator.
	// +kubebuilder:validation:Required
	Spec ExternalSecretsOperatorSpec `json:"spec,omitempty"`

	// status is the most recently observed status of the ExternalSecretsOperator.
	Status ExternalSecretsOperatorStatus `json:"status,omitempty"`
}

// ExternalSecretsOperatorSpec is the specification of the desired behavior of the ExternalSecretsOperator.
type ExternalSecretsOperatorSpec struct {
	// globalConfig is for configuring the external-secrets-operator behavior.
	// +kubebuilder:validation:Optional
	GlobalConfig *GlobalConfig `json:"globalConfig,omitempty"`

	// features is for enabling the optional features.
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
type Feature struct {
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
}

// ExternalSecretsOperatorStatus is the most recently observed status of the ExternalSecretsOperator.
type ExternalSecretsOperatorStatus struct {
	// conditions holds information of the current state of the external-secrets deployment.
	ConditionalStatus `json:",inline,omitempty"`
}
