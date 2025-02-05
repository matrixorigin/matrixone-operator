// Copyright 2025 Matrix Origin
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package v1alpha1

import (
	"time"

	recon "github.com/matrixorigin/controller-runtime/pkg/reconciler"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type CNRole string

const (
	CNRoleTP CNRole = "TP"
	CNRoleAP CNRole = "AP"
)

const (
	CNStoreStateUnknown  string = "Unknown"
	CNStoreStateDraining string = "Draining"
	CNStoreStateUp       string = "Up"

	defaultMinDelaySeconds = 15
)

const (
	ContainerPythonUdf             string = "python-udf"
	ContainerPythonUdfDefaultPort  int    = 50051
	ContainerPythonUdfDefaultImage string = "composer000/mo-python-udf-server:latest" // TODO change it
)

type CNSetTerminationPolicy string

const (
	CNSetTerminationPolicyDelete CNSetTerminationPolicy = "Delete"
	CNSetTerminationPolicyDrain  CNSetTerminationPolicy = "Drain"
)

type CNSetSpec struct {
	PodSet                 `json:",inline"`
	ConfigThatChangeCNSpec `json:",inline"`

	// ServiceType is the service type of cn service
	// +optional
	// +kubebuilder:default=ClusterIP
	// +kubebuilder:validation:Enum=ClusterIP;NodePort;LoadBalancer
	ServiceType corev1.ServiceType `json:"serviceType,omitempty"`

	// ServiceAnnotations are the annotations for the cn service
	// +optional
	ServiceAnnotations map[string]string `json:"serviceAnnotations,omitempty"`

	// NodePort specifies the node port to use when ServiceType is NodePort or LoadBalancer,
	// reconciling will fail if the node port is not available.
	// +optional
	NodePort *int32 `json:"nodePort,omitempty"`

	// [TP, AP], default to TP
	// +optional
	// Deprecated: use labels instead
	Role CNRole `json:"role,omitempty"`

	// Labels are the CN labels for all the CN stores managed by this CNSet
	Labels []CNLabel `json:"cnLabels,omitempty"`

	// ScalingConfig declares the CN scaling behavior
	ScalingConfig ScalingConfig `json:"scalingConfig,omitempty"`

	// UpdateStrategy is the rolling-update strategy of CN
	UpdateStrategy RollingUpdateStrategy `json:"updateStrategy,omitempty"`

	TerminationPolicy *CNSetTerminationPolicy `json:"terminationPolicy,omitempty"`

	// PodManagementPolicy is the pod management policy of the Pod in this Set
	PodManagementPolicy *string `json:"podManagementPolicy,omitempty"`

	// PodsToDelete are the Pods to delete in the CNSet
	PodsToDelete []string `json:"podsToDelete,omitempty"`

	// PauseUpdate means the CNSet should pause rolling-update
	PauseUpdate bool `json:"pauseUpdate,omitempty"`

	// ReusePVC means whether CNSet should reuse PVC
	ReusePVC *bool `json:"reusePVC,omitempty"`
}

// ConfigThatChangeCNSpec is an auxiliary struct to hold the config that can change CN spec
type ConfigThatChangeCNSpec struct {
	// CacheVolume is the desired local cache volume for CNSet,
	// node storage will be used if not specified
	// +optional
	CacheVolume *Volume `json:"cacheVolume,omitempty"`

	// SharedStorageCache is the configuration of the S3 sharedStorageCache
	SharedStorageCache SharedStorageCache `json:"sharedStorageCache,omitempty"`

	// PythonUdfSidecar is the python udf server in CN
	PythonUdfSidecar PythonUdfSidecar `json:"pythonUdfSidecar,omitempty"`
}

func (s *CNSetSpec) GetReusePVC() bool {
	if s.ReusePVC == nil {
		return false
	}
	return *s.ReusePVC
}

func (s *CNSetSpec) GetTerminationPolicy() CNSetTerminationPolicy {
	if s.TerminationPolicy == nil {
		return CNSetTerminationPolicyDelete
	}
	return *s.TerminationPolicy
}

type ScalingConfig struct {
	// StoreDrainEnabled is the flag to enable store draining
	StoreDrainEnabled *bool `json:"storeDrainEnabled,omitempty"`
	// StoreDrainTimeout is the timeout for draining a CN store
	StoreDrainTimeout *metav1.Duration `json:"storeDrainTimeout,omitempty"`
	// minDelaySeconds is the minimum delay when drain CN store, usually
	// be used to waiting for CN draining be propagated to the whole cluster
	MinDelaySeconds *int32 `json:"minDelaySeconds,omitempty"`
}

func (s *ScalingConfig) GetStoreDrainEnabled() bool {
	if s.StoreDrainEnabled == nil {
		return false
	}
	return *s.StoreDrainEnabled
}

func (s *ScalingConfig) GetStoreDrainTimeout() time.Duration {
	if s.StoreDrainTimeout == nil {
		return 0
	}
	return s.StoreDrainTimeout.Duration
}

func (s *ScalingConfig) GetMinDelayDuration() time.Duration {
	if s.MinDelaySeconds == nil {
		return time.Duration(defaultMinDelaySeconds) * time.Second
	}
	return time.Duration(*s.MinDelaySeconds) * time.Second
}

type PythonUdfSidecar struct {
	Enabled bool `json:"enabled,omitempty"`

	Port int `json:"port,omitempty"`

	// Image is the docker image of the python udf server
	Image string `json:"image,omitempty"`

	// Resources is the resource requirement of the python udf server
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`

	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:Schemaless
	Overlay *MainContainerOverlay `json:"overlay,omitempty"`
}

type CNLabel struct {
	// Key is the store label key
	Key string `json:"key,omitempty"`
	// Values are the store label values
	Values []string `json:"values,omitempty"`
}

// CNSetStatus Figure out what status should be exposed
type CNSetStatus struct {
	ConditionalStatus `json:",inline"`

	Stores []CNStore `json:"stores,omitempty"`

	Replicas      int32  `json:"replicas,omitempty"`
	ReadyReplicas int32  `json:"readyReplicas,omitempty"`
	LabelSelector string `json:"labelSelector,omitempty"`

	Host string `json:"host,omitempty"`
	Port int    `json:"port,omitempty"`
}

type CNStore struct {
	UUID    string `json:"uuid,omitempty"`
	PodName string `json:"podName,omitempty"`
	State   string `json:"state,omitempty"`
}

type CNSetDeps struct {
	LogSetRef `json:",inline"`
	// The DNSet it depends on
	// +kubebuilder:validation:Schemaless
	// +kubebuilder:validation:Type=object
	// +kubebuilder:pruning:PreserveUnknownFields
	// +optional
	DNSet *DNSet `json:"dnSet,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:printcolumn:name="Image",type="string",JSONPath=".spec.image"
// +kubebuilder:printcolumn:name="Replicas",type="integer",JSONPath=".spec.replicas"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// A CNSet is a resource that represents a set of MO's CN instances
// +kubebuilder:subresource:status
// +kubebuilder:subresource:scale:specpath=.spec.replicas,statuspath=.status.replicas,selectorpath=.status.labelSelector
type CNSet struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec is the desired state of CNSet
	Spec CNSetSpec `json:"spec"`
	// Deps is the dependencies of CNSet
	Deps CNSetDeps `json:"deps,omitempty"`

	Status CNSetStatus `json:"status,omitempty"`
}

func (s *CNSet) GetServiceType() corev1.ServiceType {
	if s.Spec.ServiceType == "" {
		return corev1.ServiceTypeClusterIP
	}
	return s.Spec.ServiceType
}

func (s *CNSet) GetDependencies() []recon.Dependency {
	var deps []recon.Dependency
	if s.Deps.LogSet != nil {
		deps = append(deps, &recon.ObjectDependency[*LogSet]{
			ObjectRef: s.Deps.LogSet,
			ReadyFunc: func(l *LogSet) bool {
				return recon.IsReady(&l.Status) && recon.IsSyncedWithLatestGeneration(&l.Status, l.Generation)
			},
		})
	}
	if s.Deps.DNSet != nil {
		deps = append(deps, &recon.ObjectDependency[*DNSet]{
			ObjectRef: s.Deps.DNSet,
			ReadyFunc: func(d *DNSet) bool {
				return recon.IsReady(&d.Status) && recon.IsSyncedWithLatestGeneration(&d.Status, d.Generation)
			},
		})
	}
	return deps
}

func (s *CNSet) SetCondition(condition metav1.Condition) {
	s.Status.SetCondition(condition)
}

func (s *CNSet) GetConditions() []metav1.Condition {
	return s.Status.GetConditions()
}

//+kubebuilder:object:root=true

// CNSetList contains a list of CNSet
type CNSetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CNSet `json:"items"`
}

func init() {
	SchemeBuilder.Register(&CNSet{}, &CNSetList{})
}
