package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ConditionalStatus holds information of the current state of the external-secrets deployment indicated through defined conditions.
type ConditionalStatus struct {
	// conditions holds information of the current state of deployment.
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
}

// ObjectReference is a reference to an object with a given name, kind and group.
type ObjectReference struct {
	// name of the resource being referred to.
	// +kubebuilder:validation:MinLength:=1
	// +kubebuilder:validation:MaxLength:=253
	// +required
	Name string `json:"name,omitempty"`

	// kind of the resource being referred to.
	// +kubebuilder:validation:MinLength:=1
	// +kubebuilder:validation:MaxLength:=253
	// +optional
	Kind string `json:"kind,omitempty"`

	// group of the resource being referred to.
	// +kubebuilder:validation:MinLength:=1
	// +kubebuilder:validation:MaxLength:=253
	// +optional
	Group string `json:"group,omitempty"`
}

// SecretReference is a reference to the secret with the given name, which should exist in the same namespace where it will be utilized.
type SecretReference struct {
	// name of the secret resource being referred to.
	// +kubebuilder:validation:MinLength:=1
	// +kubebuilder:validation:MaxLength:=253
	// +required
	Name string `json:"name,omitempty"`
}

// CommonConfigs are the common configurations available for all the operands managed by the operator.
type CommonConfigs struct {
	// logLevel supports value range as per [Kubernetes logging guidelines](https://github.com/kubernetes/community/blob/master/contributors/devel/sig-instrumentation/logging.md#what-method-to-use).
	// +kubebuilder:default:=1
	// +kubebuilder:validation:Minimum:=1
	// +kubebuilder:validation:Maximum:=5
	// +optional
	LogLevel int32 `json:"logLevel,omitempty"`

	// resources is for defining the resource requirements.
	// Cannot be updated.
	// ref: https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/
	// +optional
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`

	// affinity is for setting scheduling affinity rules.
	// ref: https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/
	// +optional
	Affinity *corev1.Affinity `json:"affinity,omitempty"`

	// tolerations is for setting the pod tolerations.
	// ref: https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/
	// This field can have a maximum of 50 entries.
	// +listType=atomic
	// +kubebuilder:validation:MinItems:=0
	// +kubebuilder:validation:MaxItems:=50
	// +optional
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`

	// nodeSelector is for defining the scheduling criteria using node labels.
	// ref: https://kubernetes.io/docs/concepts/configuration/assign-pod-node/
	// This field can have a maximum of 50 entries.
	// +mapType=atomic
	// +kubebuilder:validation:MinProperties:=0
	// +kubebuilder:validation:MaxProperties:=50
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// proxy is for setting the proxy configurations which will be made available in operand containers managed by the operator as environment variables.
	// +optional
	Proxy *ProxyConfig `json:"proxy,omitempty"`
}

// ProxyConfig is for setting the proxy configurations which will be made available in operand containers managed by the operator as environment variables.
type ProxyConfig struct {
	// httpProxy is the URL of the proxy for HTTP requests.
	// This field can have a maximum of 2048 characters.
	// +kubebuilder:validation:MinLength:=0
	// +kubebuilder:validation:MaxLength:=2048
	// +optional
	HTTPProxy string `json:"httpProxy,omitempty"`

	// httpsProxy is the URL of the proxy for HTTPS requests.
	// This field can have a maximum of 2048 characters.
	// +kubebuilder:validation:MinLength:=0
	// +kubebuilder:validation:MaxLength:=2048
	// +optional
	HTTPSProxy string `json:"httpsProxy,omitempty"`

	// noProxy is a comma-separated list of hostnames and/or CIDRs and/or IPs for which the proxy should not be used.
	// This field can have a maximum of 4096 characters.
	// +kubebuilder:validation:MinLength:=0
	// +kubebuilder:validation:MaxLength:=4096
	// +optional
	NoProxy string `json:"noProxy,omitempty"`
}

// Mode indicates the operational state of the optional features.
type Mode string

const (
	// Enabled indicates the optional configuration is enabled.
	Enabled Mode = "Enabled"

	// Disabled indicates the optional configuration is disabled.
	Disabled Mode = "Disabled"
)
