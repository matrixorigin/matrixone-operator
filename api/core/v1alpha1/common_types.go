// Copyright 2024 Matrix Origin
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
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
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

	EnvGoMemLimit  = "GOMEMLIMIT"
	EnvGoDebug     = "GODEBUG"
	DefaultGODebug = "madvdontneed=1,gctrace=2"
)

type PromDiscoveryScheme string

const (
	PromDiscoverySchemePod     PromDiscoveryScheme = "Pod"
	PromDiscoverySchemeService PromDiscoveryScheme = "Service"
)

type ConditionalStatus struct {
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// PodSet is an auxiliary struct to describe a set of isomorphic pods.
type PodSet struct {
	MainContainer `json:",inline"`

	// +optional
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:Schemaless
	Overlay *Overlay `json:"overlay,omitempty"`

	// Replicas is the desired number of pods of this set
	// +optional
	Replicas int32 `json:"replicas,omitempty"`

	// TopologyEvenSpread specifies what topology domains the Pods in set should be
	// evenly spread in.
	// This will be overridden by .overlay.TopologySpreadConstraints
	// +optional
	TopologyEvenSpread []string `json:"topologySpread,omitempty"`

	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// Config is the raw config for pods
	Config *TomlConfig `json:"config,omitempty"`

	// If enabled, use the Pod dns name as the Pod identity
	// Deprecated: DNSBasedIdentity is barely for keeping backward compatibility
	DNSBasedIdentity *bool `json:"dnsBasedIdentity,omitempty"`

	// ClusterDomain is the cluster-domain of current kubernetes cluster,
	// refer https://kubernetes.io/docs/concepts/services-networking/dns-pod-service/ for details
	ClusterDomain string `json:"clusterDomain,omitempty"`

	// ServiceArgs define command line options for process, used by logset/cnset/dnset service.
	// NOTE: user should not define "-cfg" argument in this field, which is defined default by controller
	// +optional
	ServiceArgs []string `json:"serviceArgs,omitempty"`

	// MemoryLimitPercent is percent used to set GOMEMLIMIT env, its value must be in interval (0, 100].
	// GOMEMLIMIT = limits.memory * MemoryLimitPercent / 100
	// +optional
	MemoryLimitPercent *int `json:"memoryLimitPercent,omitempty"`

	// ExportToPrometheus enables the pod to be discovered scraped by Prometheus
	ExportToPrometheus *bool `json:"exportToPrometheus,omitempty"`

	// PromDiscoveryScheme indicates how the Pod will be discovered by prometheus, options:
	// - Pod: the pod will be discovered via will-known labels on the Pod
	// - Service: the pod will be discovered via will-known annotations in the service which expose endpoints to the pods
	// default to Service
	PromDiscoveryScheme *PromDiscoveryScheme `json:"promDiscoveryScheme,omitempty"`

	// SemanticVersion override the semantic version of CN if set,
	// the semantic version of CN will be default to the image tag,
	// if the semantic version is not set, nor the image tag is a valid semantic version,
	// operator will treat the MO as unknown version and will not apply any version-specific
	// reconciliations
	// +optional
	SemanticVersion *string `json:"semanticVersion,omitempty"`
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
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:Schemaless
	EnvFrom []corev1.EnvFromSource `json:"envFrom,omitempty"`

	// +optional
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:Schemaless
	Env []corev1.EnvVar `json:"env,omitempty"`

	// ImagePullPolicy is the pull policy of MatrixOne image. The default value is the same as the
	// default of Kubernetes.
	// +optional
	// +kubebuilder:default=IfNotPresent
	// +kubebuilder:validation:Enum=Always;Never;IfNotPresent
	ImagePullPolicy *corev1.PullPolicy `json:"imagePullPolicy,omitempty"`

	// +optional
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:Schemaless
	VolumeMounts []corev1.VolumeMount `json:"volumeMounts,omitempty"`

	// +optional
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:Schemaless
	LivenessProbe *corev1.Probe `json:"livenessProbe,omitempty"`

	// +optional
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:Schemaless
	ReadinessProbe *corev1.Probe `json:"readinessProbe,omitempty"`

	// +optional
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:Schemaless
	StartupProbe *corev1.Probe `json:"startupProbe,omitempty"`

	// +optional
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:Schemaless
	Lifecycle *corev1.Lifecycle `json:"lifecycle,omitempty"`

	// +optional
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:Schemaless
	SecurityContext *corev1.SecurityContext `json:"mainContainerSecurityContext,omitempty"`
}

// Overlay allows advanced customization of the pod spec in the set
type Overlay struct {
	MainContainerOverlay `json:",inline"`

	// +optional
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:Schemaless
	Volumes []corev1.Volume `json:"volumes,omitempty"`

	// +optional
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:Schemaless
	VolumeClaims []corev1.PersistentVolumeClaim `json:"volumeClaims,omitempty"`

	// +optional
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:Schemaless
	InitContainers []corev1.Container `json:"initContainers,omitempty"`

	// +optional
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:Schemaless
	SidecarContainers []corev1.Container `json:"sidecarContainers,omitempty"`

	// +optional
	ServiceAccountName string `json:"serviceAccountName,omitempty"`

	// +optional
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:Schemaless
	SecurityContext *corev1.PodSecurityContext `json:"securityContext,omitempty"`

	// +optional
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:Schemaless
	ImagePullSecrets []corev1.LocalObjectReference `json:"imagePullSecrets,omitempty"`

	// +optional
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:Schemaless
	Affinity *corev1.Affinity `json:"affinity,omitempty"`

	// +optional
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:Schemaless
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`

	// +optional
	PriorityClassName string `json:"priorityClassName,omitempty"`

	// +optional
	TerminationGracePeriodSeconds *int64 `json:"terminationGracePeriodSeconds,omitempty"`

	// +optional
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:Schemaless
	HostAliases []corev1.HostAlias `json:"hostAliases,omitempty"`

	// +optional
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:Schemaless
	TopologySpreadConstraints []corev1.TopologySpreadConstraint `json:"topologySpreadConstraints,omitempty"`

	// +optional
	RuntimeClassName *string `json:"runtimeClassName,omitempty"`

	// +optional
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:Schemaless
	DNSConfig *corev1.PodDNSConfig `json:"dnsConfig,omitempty"`

	// +optional
	PodLabels map[string]string `json:"podLabels,omitempty"`

	// +optional
	PodAnnotations map[string]string `json:"podAnnotations,omitempty"`

	// +optional
	ShareProcessNamespace *bool `json:"shareProcessNamespace,omitempty"`
}

type Volume struct {
	// Size is the desired storage size of the volume
	// +required
	Size resource.Quantity `json:"size,omitempty"`

	// StorageClassName reference to the storageclass of the desired volume,
	// the default storageclass of the cluster would be used if no specified.
	// +optional
	StorageClassName *string `json:"storageClassName,omitempty"`

	// Deprecated: use SharedStorageCache instead
	MemoryCacheSize *resource.Quantity `json:"memoryCacheSize,omitempty"`
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

type SharedStorageCache struct {
	// MemoryCacheSize specifies how much memory would be used to cache the object in shared storage,
	// the default size would be 50% of the container memory request
	// MemoryCache cannot be completely disabled due to MO limitation currently, you can set MemoryCacheSize
	// to 1B to achieve an effect similar to disabling
	MemoryCacheSize *resource.Quantity `json:"memoryCacheSize,omitempty"`

	// DiskCacheSize specifies how much disk space can be used to cache the object in shared storage,
	// the default size would be 90% of the cacheVolume size to reserve some space to the filesystem metadata
	// and avoid disk space exhaustion
	// DiskCache would be disabled if CacheVolume is not set for DN/CN, and if DiskCacheSize is set while the CacheVolume
	// is not set for DN/CN, an error would be raised to indicate the misconfiguration.
	// NOTE: Unless there is a specific reason not to set this field, it is usually more reasonable to let the operator
	// set the available disk cache size according to the actual size of the cacheVolume.
	DiskCacheSize *resource.Quantity `json:"diskCacheSize,omitempty"`
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
	// CertificateRef allow specifies custom CA certificate for the object storage
	CertificateRef *CertificateRef `json:"certificateRef,omitempty"`
	// +optional
	// S3RetentionPolicy defines the retention policy of orphaned S3 bucket storage
	// +kubebuilder:validation:Enum=Delete;Retain
	S3RetentionPolicy *PVCRetentionPolicy `json:"s3RetentionPolicy,omitempty"`
}

type CertificateRef struct {
	// secret name
	// +required
	Name string `json:"name"`

	// cert files in the secret
	// +required
	Files []string `json:"files"`
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

type ObjectRef struct {
	// +optional
	Namespace string `json:"namespace,omitempty"`
	// +required
	Name string `json:"name"`
}

type RollingUpdateStrategy struct {
	// MaxSurge is an optional field that specifies the maximum number of Pods that
	// can be created over the desired number of Pods.
	// +optional
	MaxSurge *intstr.IntOrString `json:"maxSurge,omitempty"`

	// MaxUnavailable an optional field that specifies the maximum number of Pods that
	// can be unavailable during the update process.
	// +optional
	MaxUnavailable *intstr.IntOrString `json:"maxUnavailable,omitempty"`
}

func (o *ObjectRef) NamespacedName() types.NamespacedName {
	return types.NamespacedName{
		Namespace: o.Namespace,
		Name:      o.Name,
	}
}
