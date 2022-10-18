// Copyright 2022 Matrix Origin
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type PVCRetentionPolicy string

const (
	PVCRetentionPolicyDelete PVCRetentionPolicy = "Delete"
	PVCRetentionPolicyRetain PVCRetentionPolicy = "Retain"
)

type S3ProviderType string

const (
	S3ProviderTypeAWS   S3ProviderType = "aws"
	S3ProviderTypeMinIO S3ProviderType = "minio"
)

const (
	ContainerMain = "main"
)

type ConditionalStatus struct {
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// PodSet is an auxiliary struct to describe a set of isomorphic pods.
type PodSet struct {
	MainContainer `json:",inline"`

	// Replicas is the desired number of pods of this set
	Replicas int32 `json:"replicas"`

	// TopologyEvenSpread specifies what topology domains the Pods in set should be
	// evenly spread in.

	// This will be overridden by .overlay.TopologySpreadConstraints
	// +optional
	TopologyEvenSpread []string `json:"topologySpread,omitempty"`

	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// Config is the raw config for pods
	Config *TomlConfig `json:"config,omitempty"`
}

// MainContainer is the description of the main container of a Pod
type MainContainer struct {
	// Image is the docker image of the main container
	// +optional
	Image string `json:"image,omitempty"`

	// Resources is the resource requirement of the main conainer
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`
}

type MainContainerOverlay struct {
	// +optional
	Command []string `json:"command,omitempty"`

	// +optional
	Args []string `json:"args,omitempty"`

	// +optional
	EnvFrom []corev1.EnvFromSource `json:"envFrom,omitempty"`

	// +optional
	Env []corev1.EnvVar `json:"env,omitempty"`

	// ImagePullPolicy is the pull policy of MatrixOne image. The default value is the same as the
	// default of Kubernetes.
	// +optional
	// +kubebuilder:default=IfNotPresent
	// +kubebuilder:validation:Enum=Always;Never;IfNotPresent
	ImagePullPolicy corev1.PullPolicy `json:"imagePullPolicy,omitempty"`

	// +optional
	VolumeMounts []corev1.VolumeMount `json:"volumeMounts,omitempty"`

	// +optional
	LivenessProbe *corev1.Probe `json:"livenessProbe,omitempty"`

	// +optional
	ReadinessProbe *corev1.Probe `json:"readinessProbe,omitempty"`

	// +optional
	StartupProbe *corev1.Probe `json:"startupProbe,omitempty"`

	// +optional
	Lifecycle *corev1.Lifecycle `json:"lifecycle,omitempty"`
}

// Overlay allows advanced customization of the pod spec in the set
type Overlay struct {
	MainContainerOverlay `json:",inline"`

	// +optional
	Volumes []corev1.Volume `json:"volumes,omitempty"`

	// +optional
	VolumeClaims []corev1.PersistentVolumeClaim `json:"volumeClaims,omitempty"`

	// +optional
	InitContainers []corev1.Container `json:"initContainers,omitempty"`

	// +optional
	SidecarContainers []corev1.Container `json:"sidecarContainers,omitempty"`

	// +optional
	ServiceAccountName string `json:"serviceAccountName,omitempty"`

	// +optional
	SecurityContext *corev1.PodSecurityContext `json:"securityContext,omitempty"`

	// +optional
	ImagePullSecrets []corev1.LocalObjectReference `json:"imagePullSecrets,omitempty"`

	// +optional
	Affinity *corev1.Affinity `json:"affinity,omitempty"`

	// +optional
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`

	// +optional
	PriorityClassName string `json:"priorityClassName,omitempty"`

	// +optional
	TerminationGracePeriodSeconds *int64 `json:"terminationGracePeriodSeconds,omitempty"`

	// +optional
	HostAliases []corev1.HostAlias `json:"hostAliases,omitempty"`

	// +optional
	TopologySpreadConstraints []corev1.TopologySpreadConstraint `json:"topologySpreadConstraints,omitempty"`

	// +optional
	RuntimeClassName *string `json:"runtimeClassName,omitempty"`

	// +optional
	DNSConfig *corev1.PodDNSConfig `json:"dnsConfig,omitempty"`

	// +optional
	PodLabels map[string]string `json:"podLabels,omitempty"`

	// +optional
	PodAnnotations map[string]string `json:"podAnnotations,omitempty"`
}

type Volume struct {
	// Size is the desired storage size of the volume
	// +required
	Size resource.Quantity `json:"size,omitempty"`

	// StorageClassName reference to the storageclass of the desired volume,
	// the default storageclass of the cluster would be used if no specified.
	// +optional
	StorageClassName *string `json:"storageClassName,omitempty"`
}

type SharedStorageProvider struct {
	// S3 specifies an S3 bucket as the shared storage provider,
	// mutual-exclusive with other providers.
	S3 *S3Provider `json:"s3,omitempty"`
	// FileSystem specified a fileSystem path as the shared storage provider,
	// it assumes a shared filesystem is mounted to this path and instances can
	// safely read-write this path in current manner.
	FileSystem *FileSystemProvider `json:"fileSystem,omitempty"`
}

type FileSystemProvider struct {
	// Path the path that the shared fileSystem mounted to
	// +required
	Path string `json:"path"`
}

type S3Provider struct {
	// Path is the s3 storage path in <bucket-name>/<folder> format, e.g. "my-bucket/my-folder"
	// +required
	Path string `json:"path"`
	// S3ProviderType is type of this s3 provider, options: [aws, minio]
	// default to aws
	// +optional
	Type *S3ProviderType `json:"type,omitempty"`
	// Region of the bucket
	// the default region will be inferred from the deployment environment
	// +optional
	Region string `json:"region,omitempty"`
	// Endpoint is the endpoint of the S3 compatible service
	// default to aws S3 well known endpoint
	// +optional
	Endpoint string `json:"endpoint,omitempty"`
	// Credentials for s3, the client will automatically discover credential sources
	// from the environment if not specified
	// +optional
	SecretRef *corev1.LocalObjectReference `json:"secretRef,omitempty"`
}

func (p *S3Provider) GetProviderType() S3ProviderType {
	if p.Type == nil {
		return S3ProviderTypeAWS
	}
	return *p.Type
}

// LogSetRef reference to an LogSet, either internal or external
type LogSetRef struct {
	// The LogSet it depends on, mutual exclusive with ExternalLogSet
	// +kubebuilder:validation:Schemaless
	// +kubebuilder:validation:Type=object
	// +kubebuilder:pruning:PreserveUnknownFields
	// +optional
	LogSet *LogSet `json:"logSet,omitempty"`

	// An external LogSet the CNSet should connected to,
	// mutual exclusive with LogSet
	// TODO: rethink the schema of ExternalLogSet
	// +optional
	ExternalLogSet *ExternalLogSet `json:"externalLogSet,omitempty"`
}

type ExternalLogSet struct {
	// HAKeeperEndpoint of the ExternalLogSet
	// +required
	HAKeeperEndpoint string `json:"haKeeperEndpoint,omitempty"`
}

type FailoverStatus struct {
	AvailableStores []Store `json:"availableStores,omitempty"`
	FailedStores    []Store `json:"failedStores,omitempty"`
}

type Store struct {
	PodName            string      `json:"podName,omitempty"`
	Phase              string      `json:"phase,omitempty"`
	LastTransitionTime metav1.Time `json:"lastTransition,omitempty"`
}
