// Copyright 2021 Matrix Origin
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
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// MatrixoneClusterSpec defines the desired state of MatrixoneCluster
type MatrixoneClusterSpec struct {
	Replicas                      *int32                        `json:"replicas,omitempty"`
	Image                         string                        `json:"image,omitempty"`
	Command                       []string                      `json:"command,omitempty"`
	ImagePullSecrets              []corev1.LocalObjectReference `json:"imagePullSecrets,omitempty"`
	TerminationGracePeriodSeconds *int64                        `json:"terminationGracePeriodSeconds,omitempty"`

	// Optional: Default is true, will delete the sts pod if sts is set to ordered ready to ensure
	// issue: https://github.com/kubernetes/kubernetes/issues/67250
	// doc: https://kubernetes.io/docs/concepts/workloads/controllers/statefulset/#forced-rollback
	ForceDeleteStsPodOnError bool `json:"forceDeleteStsPodOnError,omitempty"`

	// Optional: Default is set to true, orphaned ( unmounted pvc's ) shall be cleaned up by the operator.
	// +optional
	DeleteOrphanPvc bool `json:"deleteOrphanPvc"`

	// Optional: Default is set to false, pvc shall be deleted on deletion of CR
	DisablePVCDeletionFinalizer bool `json:"disablePVCDeletionFinalizer,omitempty"`

	// Optional: dns policy
	DNSPolicy corev1.DNSPolicy `json:"dnsPolicy,omitempty"`

	// Optional: dns config
	DNSConfig *corev1.PodDNSConfig `json:"dnsConfig,omitempty"`

	RollingDeploy       bool                              `json:"rollingDeploy,omitempty"`
	ImagePullPolicy     corev1.PullPolicy                 `json:"imagePullPolicy,omitempty"`
	StorageClass        *string                           `json:"storageClass,omitempty"`
	PodAnnotations      map[string]string                 `json:"podAnnotations,omitempty"`
	LogVolCap           string                            `json:"logVolumeCap,omitempty"`
	DataVolCap          string                            `json:"dataVolumeCap,omitempty"`
	ServiceType         corev1.ServiceType                `json:"serviceType,omitempty"`
	PodName             corev1.EnvVar                     `json:"podName,omitempty"`
	LivenessProbe       *corev1.Probe                     `json:"livenessProbe,omitempty"`
	ReadinessProbe      *corev1.Probe                     `json:"readinessProbe,omitempty"`
	UpdateStrategy      *appsv1.StatefulSetUpdateStrategy `json:"updateStrategy,omitempty"`
	Requests            corev1.ResourceList               `json:"requests,omitempty"`
	Limits              corev1.ResourceList               `json:"limits,omitempty"`
	Affinity            *corev1.Affinity                  `json:"affinity,omitempty"`
	NodeSelector        map[string]string                 `json:"nodeSelector,omitempty"`
	Tolerations         []corev1.Toleration               `json:"tolerations,omitempty"`
	PodManagementPolicy appsv1.PodManagementPolicyType    `json:"podManagementPolicy,omitempty"`
}

// MatrixoneClusterStatus defines the observed state of MatrixoneCluster
type MatrixoneClusterStatus struct {
	StatefulSets          []string `json:"statefulSets,omitempty"`
	Services              []string `json:"service,omitempty"`
	Pods                  []string `json:"pods,omitempty"`
	PersistentVolumeClaim []string `json:"persistentVolumeClaims,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// MatrixoneCluster is the Schema for the matrixoneclusters API
type MatrixoneCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MatrixoneClusterSpec   `json:"spec,omitempty"`
	Status MatrixoneClusterStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// MatrixoneClusterList contains a list of MatrixoneCluster
type MatrixoneClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MatrixoneCluster `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MatrixoneCluster{}, &MatrixoneClusterList{})
}
