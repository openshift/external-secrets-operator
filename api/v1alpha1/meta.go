package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ConditionalStatus holds information of the current state of the external-secrets deployment
// indicated through defined conditions.
type ConditionalStatus struct {
	// conditions holds information of the current state of deployment.
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// ObjectReference is a reference to an object with a given name, kind and group.
type ObjectReference struct {
	// Name of the resource being referred to.
	// +kubebuilder:validation:MinLength:=1
	// +kubebuilder:validation:MaxLength:=253
	// +kubebuilder:validation:Required
	Name string `json:"name"`
	// Kind of the resource being referred to.
	// +kubebuilder:validation:MinLength:=1
	// +kubebuilder:validation:MaxLength:=253
	// +kubebuilder:validation:Optional
	Kind string `json:"kind,omitempty"`
	// Group of the resource being referred to.
	// +kubebuilder:validation:MinLength:=1
	// +kubebuilder:validation:MaxLength:=253
	// +kubebuilder:validation:Optional
	Group string `json:"group,omitempty"`
}

// SecretReference is a reference to the secret with the given name, which should exist
// in the same namespace where it will be utilized.
type SecretReference struct {
	// Name of the secret resource being referred to.
	// +kubebuilder:validation:MinLength:=1
	// +kubebuilder:validation:MaxLength:=253
	// +kubebuilder:validation:Required
	Name string `json:"name"`
}

// CommonConfigs are the common configurations available for all the operands managed
// by the operator.
type CommonConfigs struct {
	// logLevel supports value range as per [kubernetes logging guidelines](https://github.com/kubernetes/community/blob/master/contributors/devel/sig-instrumentation/logging.md#what-method-to-use).
	// +kubebuilder:default:=1
	// +kubebuilder:validation:Minimum:=1
	// +kubebuilder:validation:Maximum:=5
	// +kubebuilder:validation:Optional
	LogLevel int32 `json:"logLevel,omitempty"`

	// labels to apply to all resources created by the operator.
	// +mapType=granular
	// +kubebuilder:validation:MinProperties:=0
	// +kubebuilder:validation:MaxProperties:=20
	// +kubebuilder:validation:Optional
	Labels map[string]string `json:"labels,omitempty"`

	// resources is for defining the resource requirements.
	// Cannot be updated.
	// ref: https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/
	// +kubebuilder:validation:Optional
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`

	// affinity is for setting scheduling affinity rules.
	// ref: https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/
	// +kubebuilder:validation:Optional
	Affinity *corev1.Affinity `json:"affinity,omitempty"`

	// tolerations is for setting the pod tolerations.
	// ref: https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/
	// +kubebuilder:validation:Optional
	// +listType=atomic
	// +kubebuilder:validation:MinItems:=0
	// +kubebuilder:validation:MaxItems:=10
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`

	// nodeSelector is for defining the scheduling criteria using node labels.
	// ref: https://kubernetes.io/docs/concepts/configuration/assign-pod-node/
	// +kubebuilder:validation:Optional
	// +mapType=atomic
	// +kubebuilder:validation:MinProperties:=0
	// +kubebuilder:validation:MaxProperties:=10
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// proxy is for setting the proxy configurations which will be made available
	// in operand containers managed by the operator as environment variables.
	// +kubebuilder:validation:Optional
	Proxy *ProxyConfig `json:"proxy,omitempty"`
}

// ProxyConfig is for setting the proxy configurations which will be made available
// in operand containers managed by the operator as environment variables.
type ProxyConfig struct {
	// httpProxy is the URL of the proxy for HTTP requests.
	// +kubebuilder:validation:MinLength:=1
	// +kubebuilder:validation:MaxLength:=4096
	// +kubebuilder:validation:Optional
	HTTPProxy string `json:"httpProxy,omitempty"`

	// httpsProxy is the URL of the proxy for HTTPS requests.
	// +kubebuilder:validation:MinLength:=1
	// +kubebuilder:validation:MaxLength:=4096
	// +kubebuilder:validation:Optional
	HTTPSProxy string `json:"httpsProxy,omitempty"`

	// noProxy is a comma-separated list of hostnames and/or CIDRs and/or IPs for which the proxy should not be used.
	// +kubebuilder:validation:MinLength:=1
	// +kubebuilder:validation:MaxLength:=4096
	// +kubebuilder:validation:Optional
	NoProxy string `json:"noProxy,omitempty"`
}
