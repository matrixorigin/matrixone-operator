// Copyright 2023 Matrix Origin
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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type State string

const (
	// StatusInUse - a bucket still in use
	StatusInUse State = "InUse"
	// StatusReleased - bucket has been released, can be reused
	StatusReleased State = "Released"
	// StatusDeleting - bucket is deleting, data in share storage s3
	StatusDeleting State = "Deleting"
)

// +kubebuilder:object:root=true
// +kubebuilder:resource:shortName=bucket
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.state"
// +kubebuilder:printcolumn:name="Bind",type="string",JSONPath=".status.bindTo"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// A BucketClaim is a resource that represents the object storage bucket resource used by a mo cluster
type BucketClaim struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec is the desired state of BucketClaim
	Spec BucketClaimSpec `json:"spec"`

	// Status is the current state of BucketClaim
	Status BucketClaimStatus `json:"status,omitempty"`
}

type BucketClaimSpec struct {
	// S3 specifies an S3 bucket as the shared storage provider, mutual-exclusive with other providers.
	// +required
	S3 *S3Provider `json:"s3,omitempty"`

	// LogSetTemplate is a complete copy version of kruise statefulset PodTemplateSpec
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:Schemaless
	LogSetTemplate corev1.PodTemplateSpec `json:"logSetSpec"`
}

type BucketClaimStatus struct {
	// BindTo implies namespace and name of logset which BucketClaim bound to, in format of "namespace/name"
	BindTo string `json:"bindTo"`

	// +kubebuilder:validation:Enum=InUse;Released;Deleting
	State State `json:"state,omitempty"`

	// ConditionalStatus includes condition of deleting s3 resource progress
	ConditionalStatus `json:",inline"`
}

//+kubebuilder:object:root=true

// BucketClaimList contains a list of BucketClaim
type BucketClaimList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []BucketClaim `json:"items"`
}

func init() {
	SchemeBuilder.Register(&BucketClaim{}, &BucketClaimList{})
}
