// Copyright 2024 Matrix Origin
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
	fmt "fmt"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"time"
)

const (
	JobPhasePending   = "Pending"
	JobPhaseRunning   = "Running"
	JobPhaseCompleted = "Completed"
	JobPhaseFailed    = "Failed"
)

const (
	JobConditionTypeEnded = "Ended"
)

const (
	defaultTTL = 1 * time.Hour
)

// BackupJobSpec specifies the backup job
type BackupJobSpec struct {
	// ttl defines the time to live of the backup job after completed or failed
	TTL *metav1.Duration `json:"ttl,omitempty"`

	// source the backup source
	Source BackupSource `json:"source"`

	Target SharedStorageProvider `json:"target"`

	Overlay *Overlay `json:"overlay,omitempty"`
}

// BackupSource is the source of the backup job
type BackupSource struct {
	// clusterRef is the name of the cluster to back up, mutual exclusive with cnSetRef
	ClusterRef *string `json:"clusterRef,omitempty"`

	// cnSetRef is the name of the cnSet to back up, mutual exclusive with clusterRef
	CNSetRef *string `json:"cnSetRef,omitempty"`

	// optional, secretRef is the name of the secret to use for authentication
	SecretRef *string `json:"secretRef,omitempty"`
}

func (r *BackupJob) GetSourceRef() string {
	if r.Spec.Source.ClusterRef != nil {
		return fmt.Sprintf("matrixonecluster/%s/%s", r.Namespace, *r.Spec.Source.ClusterRef)
	}
	if r.Spec.Source.CNSetRef != nil {
		return fmt.Sprintf("cnset/%s/%s", r.Namespace, *r.Spec.Source.CNSetRef)
	}
	return ""
}

type BackupJobStatus struct {
	ConditionalStatus `json:",inline"`

	Phase string `json:"phase,omitempty"`

	Backup string `json:"backup,omitempty"`
}

// A BackupJob is a resource that represents an MO backup job
// +kubebuilder:object:root=true
// +kubebuilder:resource:scope="Namespaced"
// +kubebuilder:printcolumn:name="phase",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="Backup",type="string",JSONPath=".status.backup"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
type BackupJob struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec is the backupJobSpec
	Spec BackupJobSpec `json:"spec"`

	// Spec is the backupJobStatus
	Status BackupJobStatus `json:"status,omitempty"`
}

func (r *BackupJob) GetTTL() time.Duration {
	if r.Spec.TTL != nil {
		return r.Spec.TTL.Duration
	}
	return defaultTTL
}

// BackupMeta specifies the backup
type BackupMeta struct {
	// location is the data location of the backup
	Location SharedStorageProvider `json:"location"`

	// id uniquely identifies the backup
	ID string `json:"id"`

	// size is the backup data size
	Size *resource.Quantity `json:"size,omitempty"`

	// atTime is the backup start time
	AtTime metav1.Time `json:"atTime"`

	// completeTime the backup complete time
	CompleteTime metav1.Time `json:"completeTime"`

	// clusterRef is the reference to the cluster that produce this backup
	SourceRef string `json:"sourceRef"`

	Raw string `json:"raw"`
}

// A Backup is a resource that represents an MO physical backup
// +kubebuilder:object:root=true
// +kubebuilder:resource:scope="Cluster"
// +kubebuilder:printcolumn:name="ID",type="string",JSONPath=".meta.id"
// +kubebuilder:printcolumn:name="At",type="string",format="date-time",JSONPath=".meta.atTime"
// +kubebuilder:printcolumn:name="Source",type="string",JSONPath=".meta.sourceRef"
type Backup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Meta is the backupMeta
	Meta BackupMeta `json:"meta"`
}

type RestoreJobSpec struct {
	// ttl defines the time to live of the backup job after completed or failed
	TTL *metav1.Duration `json:"ttl,omitempty"`

	// backupName specifies the backup to restore, must be set UNLESS externalSource is set
	BackupName string `json:"backupName,omitempty"`

	// optional, restore from an external source, mutual exclusive with backupName
	ExternalSource *SharedStorageProvider `json:"externalSource,omitempty"`

	// target specifies the restore location
	Target SharedStorageProvider `json:"target"`
}

type RestoreJobStatus struct {
	ConditionalStatus `json:",inline"`

	Phase string `json:"phase"`
}

// A RestoreJob is a resource that represents an MO restore job
// +kubebuilder:object:root=true
// +kubebuilder:resource:scope="Namespaced"
// +kubebuilder:printcolumn:name="phase",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
type RestoreJob struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec is the restoreJobSpec
	Spec RestoreJobSpec `json:"spec"`

	// Spec is the restoreJobStatus
	Status RestoreJobStatus `json:"status,omitempty"`

	Overlay *Overlay `json:"overlay,omitempty"`
}

func (r *RestoreJob) GetTTL() time.Duration {
	if r.Spec.TTL != nil {
		return r.Spec.TTL.Duration
	}
	return defaultTTL
}

func (r *RestoreJob) GetOverlay() *Overlay {
	return r.Overlay
}

func (r *RestoreJob) SetCondition(condition metav1.Condition) {
	r.Status.SetCondition(condition)
}

func (r *RestoreJob) GetConditions() []metav1.Condition {
	return r.Status.GetConditions()
}

func (r *RestoreJob) GetPhase() string {
	return r.Status.Phase
}

func (r *RestoreJob) SetPhase(phase string) {
	r.Status.Phase = phase
}

func (r *BackupJob) GetOverlay() *Overlay {
	return r.Spec.Overlay
}

func (r *BackupJob) SetCondition(condition metav1.Condition) {
	r.Status.SetCondition(condition)
}

func (r *BackupJob) GetConditions() []metav1.Condition {
	return r.Status.GetConditions()
}

func (r *BackupJob) GetPhase() string {
	return r.Status.Phase
}

func (r *BackupJob) SetPhase(phase string) {
	r.Status.Phase = phase
}

// BackupJobList contains a list of BackupJob
// +kubebuilder:object:root=true
type BackupJobList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []BackupJob `json:"items"`
}

// BackupList contains a list of BackupJ
// +kubebuilder:object:root=true
type BackupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Backup `json:"items"`
}

// RestoreJobList contains a list of RestoreJob
// +kubebuilder:object:root=true
type RestoreJobList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RestoreJob `json:"items"`
}

func init() {
	SchemeBuilder.Register(&BackupJob{}, &BackupJobList{})
	SchemeBuilder.Register(&Backup{}, &BackupList{})
	SchemeBuilder.Register(&RestoreJob{}, &RestoreJobList{})
}
