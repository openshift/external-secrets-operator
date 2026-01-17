package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func init() {
	SchemeBuilder.Register(&ExternalSecretsManager{}, &ExternalSecretsManagerList{})
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true

// ExternalSecretsManagerList is a list of ExternalSecretsManager objects.
type ExternalSecretsManagerList struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is the standard list's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	metav1.ListMeta `json:"metadata"`

	Items []ExternalSecretsManager `json:"items"`
}

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=externalsecretsmanagers,scope=Cluster,categories={external-secrets-operator, external-secrets},shortName=esm;externalsecretsmanager;esmanager
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:metadata:labels={"app.kubernetes.io/name=externalsecretsmanager", "app.kubernetes.io/part-of=external-secrets-operator"}

// ExternalSecretsManager describes configuration and information about the deployments managed by the external-secrets-operator.
// The name must be `cluster` as this is a singleton object allowing only one instance of ExternalSecretsManager per cluster.
//
// It is mainly for configuring the global options and enabling optional features, which serves as a common/centralized config for managing multiple controllers of the operator.
// The object is automatically created during the operator installation.
//
// +kubebuilder:validation:XValidation:rule="self.metadata.name == 'cluster'",message="ExternalSecretsManager is a singleton, .metadata.name must be 'cluster'"
// +operator-sdk:csv:customresourcedefinitions:displayName="ExternalSecretsManager"
type ExternalSecretsManager struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is the standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// spec is the specification of the desired behavior
	// +optional
	Spec ExternalSecretsManagerSpec `json:"spec,omitempty"`

	// status is the most recently observed status of controllers used by External Secrets Operator.
	// +optional
	Status ExternalSecretsManagerStatus `json:"status,omitempty"`
}

// ExternalSecretsManagerSpec is the specification of the desired behavior of the ExternalSecretsManager.
type ExternalSecretsManagerSpec struct {
	// globalConfig is for configuring the behavior of deployments that are managed by external secrets-operator.
	// +optional
	GlobalConfig *GlobalConfig `json:"globalConfig,omitempty"`
}

// GlobalConfig is for configuring the external-secrets-operator behavior.
type GlobalConfig struct {
	// labels to apply to all resources created by the operator.
	// This field can have a maximum of 20 entries.
	// +mapType=granular
	// +kubebuilder:validation:MinProperties:=0
	// +kubebuilder:validation:MaxProperties:=20
	// +optional
	Labels map[string]string `json:"labels,omitempty"`

	CommonConfigs `json:",inline"`
}

// ExternalSecretsManagerStatus is the most recently observed status of the ExternalSecretsManager.
type ExternalSecretsManagerStatus struct {
	// controllerStatuses holds the observed conditions of the controllers part of the operator.
	// +patchMergeKey=name
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=name
	// +optional
	ControllerStatuses []ControllerStatus `json:"controllerStatuses,omitempty"`

	// lastTransitionTime is the last time the condition transitioned from one status to another.
	// +kubebuilder:validation:Type=string
	// +kubebuilder:validation:Format=date-time
	// +optional
	LastTransitionTime metav1.Time `json:"lastTransitionTime,omitempty"`
}

// ControllerStatus holds the observed conditions of the controllers part of the operator.
type ControllerStatus struct {
	// name of the controller for which the observed condition is recorded.
	// +required
	Name string `json:"name,omitempty"`

	// conditions holds information of the current state of the external-secrets-operator controllers.
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type
	Conditions []Condition `json:"conditions,omitempty"`

	// observedGeneration represents the .metadata.generation on the observed resource.
	// +kubebuilder:validation:Minimum=0
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

type Condition struct {
	// type of the condition
	// +required
	Type string `json:"type,omitempty"`

	// status of the condition
	Status metav1.ConditionStatus `json:"status"`

	// message provides details about the state.
	Message string `json:"message"`
}
