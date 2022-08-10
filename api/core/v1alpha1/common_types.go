package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ConditionType string

const (
	// Whether the object is ready to act
	ConditionTypeReady = "Ready"
	// Whether the object is update to date
	ConditionTypeSynced = "Synced"
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

	// Config is the raw config for pods
	Config *TomlConfig `json:"config,omitempty"`
}

// MainContainers is the description of the main container of a Pod
type MainContainer struct {
	// Image is the docker image of the main container
	// +required
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

	// +optional
	VolumeMounts []corev1.VolumeMount `json:"volumeMounts,omitempty"`

	// +optional
	LivenessProbe *corev1.Probe `json:"livenessProbe,omitempty"`

	// +optional
	ReadinessProbe *corev1.Probe `json:"readinessProbe,omitempty"`

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
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

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
	Size resource.Quantity `json:"size"`

	// StorageClassName reference to the storageclass of the desired volume,
	// the default storageclass of the cluster would be used if no specified.
	// +optional
	StorageClassName *string `json:"storageClassName,omitempty"`
}

type SharedStorageProvider struct {
	// S3 specifies an S3 bucket as the shared storage provider,
	// mutual-exclusive with other providers.
	S3 *S3Provider `json:"s3,omitempty"`
}

type S3Provider struct {
	// Path is the s3 storage path in <bucket-name>/<folder> format, e.g. "my-bucket/my-folder"
	// +required
	Path string `json:"path"`
	// Region of the S3 bucket
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
