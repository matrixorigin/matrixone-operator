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

package cnclaimset

import (
	"testing"
	"time"

	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	"github.com/matrixorigin/matrixone-operator/pkg/controllers/common"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "github.com/onsi/gomega"
)

func Test_sortClaimsToDelete(t *testing.T) {
	type args struct {
		cps []ClaimAndPod
	}
	now := time.Now()
	tests := []struct {
		name  string
		cps   []ClaimAndPod
		order []string
	}{{
		name: "basic",
		cps: []ClaimAndPod{
			{
				Claim: &v1alpha1.CNClaim{
					ObjectMeta: metav1.ObjectMeta{
						Name: "pod-old",
					},
					Status: v1alpha1.CNClaimStatus{
						Phase: v1alpha1.CNClaimPhaseBound,
					},
				},
				Pod: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:              "old-pod",
						CreationTimestamp: metav1.Time{Time: now.Add(-time.Hour)},
					},
					Spec: corev1.PodSpec{
						NodeName: "test",
					},
					Status: corev1.PodStatus{
						Phase: corev1.PodRunning,
					},
				},
			},
			{
				Claim: &v1alpha1.CNClaim{
					ObjectMeta: metav1.ObjectMeta{
						Name: "pod-high-deletion-cost",
					},
					Status: v1alpha1.CNClaimStatus{
						Phase: v1alpha1.CNClaimPhaseBound,
					},
				},
				Pod: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						CreationTimestamp: metav1.Time{Time: now},
						Annotations: map[string]string{
							common.DeletionCostAnno: "100",
						},
					},
					Spec: corev1.PodSpec{
						NodeName: "test",
					},
					Status: corev1.PodStatus{
						Phase: corev1.PodRunning,
					},
				},
			},
			{
				Claim: &v1alpha1.CNClaim{
					ObjectMeta: metav1.ObjectMeta{
						Name: "pod-pending",
					},
					Status: v1alpha1.CNClaimStatus{
						Phase: v1alpha1.CNClaimPhaseBound,
					},
				},
				Pod: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						CreationTimestamp: metav1.Time{Time: now},
					},
					Spec: corev1.PodSpec{
						NodeName: "test",
					},
					Status: corev1.PodStatus{
						Phase: corev1.PodPending,
					},
				},
			},
			{
				Claim: &v1alpha1.CNClaim{
					ObjectMeta: metav1.ObjectMeta{
						Name: "pod-middle-deletion-cost",
					},
					Status: v1alpha1.CNClaimStatus{
						Phase: v1alpha1.CNClaimPhaseBound,
					},
				},
				Pod: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						CreationTimestamp: metav1.Time{Time: now},
						Annotations: map[string]string{
							common.DeletionCostAnno: "10",
						},
					},
					Spec: corev1.PodSpec{
						NodeName: "test",
					},
					Status: corev1.PodStatus{
						Phase: corev1.PodRunning,
					},
				},
			},
			{
				Claim: &v1alpha1.CNClaim{
					ObjectMeta: metav1.ObjectMeta{
						Name: "pod-unscheduled",
					},
					Status: v1alpha1.CNClaimStatus{
						Phase: v1alpha1.CNClaimPhaseBound,
					},
				},
				Pod: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						CreationTimestamp: metav1.Time{Time: now},
					},
					Spec: corev1.PodSpec{
						NodeName: "",
					},
				},
			},
			{
				Claim: &v1alpha1.CNClaim{
					ObjectMeta: metav1.ObjectMeta{
						Name: "just-bind",
					},
					Status: v1alpha1.CNClaimStatus{
						Phase: v1alpha1.CNClaimPhaseBound,
					},
				},
				Pod: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:              "current-pod",
						CreationTimestamp: metav1.Time{Time: now},
					},
					Spec: corev1.PodSpec{
						NodeName: "test",
					},
					Status: corev1.PodStatus{
						Phase: corev1.PodRunning,
					},
				},
			},
			{
				Claim: &v1alpha1.CNClaim{
					ObjectMeta: metav1.ObjectMeta{
						Name: "claim-outdated",
					},
					Status: v1alpha1.CNClaimStatus{
						Phase: v1alpha1.CNClaimPhaseOutdated,
					},
				},
				Pod: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						CreationTimestamp: metav1.Time{Time: now},
					},
					Spec: corev1.PodSpec{
						NodeName: "test",
					},
					Status: corev1.PodStatus{
						Phase: corev1.PodRunning,
					},
				},
			},
			{
				Claim: &v1alpha1.CNClaim{
					ObjectMeta: metav1.ObjectMeta{
						Name: "claim-lost",
					},
					Status: v1alpha1.CNClaimStatus{
						Phase: v1alpha1.CNClaimPhaseLost,
					},
				},
				Pod: nil,
			},
			{
				Claim: &v1alpha1.CNClaim{
					ObjectMeta: metav1.ObjectMeta{
						Name:              "claim-pending",
						CreationTimestamp: metav1.NewTime(now),
					},
					Status: v1alpha1.CNClaimStatus{
						Phase: v1alpha1.CNClaimPhasePending,
					},
				},
				Pod: nil,
			},
			{
				Claim: &v1alpha1.CNClaim{
					ObjectMeta: metav1.ObjectMeta{
						Name:              "claim-pending-old",
						CreationTimestamp: metav1.NewTime(now.Add(-time.Hour)),
					},
					Status: v1alpha1.CNClaimStatus{
						Phase: v1alpha1.CNClaimPhasePending,
					},
				},
				Pod: nil,
			},
		},
		order: []string{
			"claim-pending",
			"claim-pending-old",
			"claim-lost",
			"claim-outdated",
			"pod-unscheduled",
			"pod-pending",
			"just-bind",
			"pod-old",
			"pod-middle-deletion-cost",
			"pod-high-deletion-cost",
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sortClaimsToDelete(tt.cps)
			g := NewGomegaWithT(t)
			var res []string
			for _, cp := range tt.cps {
				res = append(res, cp.Claim.Name)
			}
			g.Expect(res).To(Equal(tt.order))
		})
	}
}
