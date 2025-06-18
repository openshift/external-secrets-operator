package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

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
	// +kubebuilder:validation:Required
	Name string `json:"name"`
	// Kind of the resource being referred to.
	// +kubebuilder:validation:Optional
	Kind string `json:"kind,omitempty"`
	// Group of the resource being referred to.
	// +kubebuilder:validation:Optional
	Group string `json:"group,omitempty"`
}

// SecretReference is a reference to the secret with the given name, which should exist
// in the same namespace where it will be utilized.
type SecretReference struct {
	// Name of the secret resource being referred to.
	// +kubebuilder:validation:Required
	Name string `json:"name"`
}
