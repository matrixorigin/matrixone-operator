package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
)

type KubernetesConfig struct {
	Image                  string                         `json:"image"`
	ImagePullPolicy        corev1.PullPolicy              `json:"imagePullPolicy,omitempty"`
	Resources              *corev1.ResourceRequirements   `json:"resources,omitempty"`
	ExistingPasswordSecret *ExistingPasswordSecret        `json:"matrixoneSecret,omitempty"`
	ImagePullSecrets       *[]corev1.LocalObjectReference `json:"imagePullSecrets,omitempty"`
}

type MatrixoneConfig struct {
	AdditionalMatrixoneConfig *string `json:"additionalMatrixoneConfig,omitempty"`
}

type ExistingPasswordSecret struct {
	Name *string `json:"name,omitempty"`
	Key  *string `json:"key,omitempty"`
}

type Stroage struct {
	VolumeClaimTemplate corev1.PersistentVolumeClaim `json:"volumeClaimTemplate,omitempty"`
}

type MatrixoneExporter struct {
	Enabled         bool                         `json:"enabled,omitempty"`
	Image           string                       `json:"image"`
	Resources       *corev1.ResourceRequirements `json:"resources,omitempty"`
	ImagePullPolicy corev1.PullPolicy            `json:"imagePullPolicy,omitempty"`
	EnvVars         *[]corev1.EnvVar             `json:"env,omitempty"`
}
