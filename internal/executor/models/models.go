package models

import "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

type ResourceResult struct {
	// Resource identify a Kubernetes resource
	Resource *unstructured.Unstructured `json:"resource,omitempty"`

	// Message is an optional field.
	// +optional
	Message string `json:"message,omitempty"`
}
