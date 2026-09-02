// Copyright 2025-2026 Matrix Origin
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
	"context"
	"testing"
	"time"

	"github.com/matrixorigin/controller-runtime/pkg/fake"
	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	"github.com/matrixorigin/matrixone-operator/pkg/controllers/common"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/onsi/gomega"
)

func Test_scaleIn_skipsMigratingClaims(t *testing.T) {
	g := NewGomegaWithT(t)
	now := time.Now()

	oc := &ownedClaims{
		owned: []v1alpha1.CNClaim{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "claim-migrating",
					Namespace: "ns",
				},
				Spec: v1alpha1.CNClaimSpec{
					ClaimPodRef: v1alpha1.ClaimPodRef{PodName: "pod-target"},
					SourcePod:   &v1alpha1.ClaimPodRef{PodName: "pod-source"},
				},
				Status: v1alpha1.CNClaimStatus{
					Phase: v1alpha1.CNClaimPhaseBound,
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "claim-normal",
					Namespace:         "ns",
					CreationTimestamp: metav1.NewTime(now),
				},
				Spec: v1alpha1.CNClaimSpec{
					ClaimPodRef: v1alpha1.ClaimPodRef{PodName: ""},
				},
				Status: v1alpha1.CNClaimStatus{
					Phase: v1alpha1.CNClaimPhasePending,
				},
			},
		},
	}

	scheme := runtime.NewScheme()
	g.Expect(v1alpha1.AddToScheme(scheme)).To(Succeed())
	cli := fake.KubeClientBuilder().
		WithScheme(scheme).
		WithObjects(&oc.owned[0], &oc.owned[1]).
		Build()
	ctx := fake.NewContext(&v1alpha1.CNClaimSet{
		ObjectMeta: metav1.ObjectMeta{Name: "claimset", Namespace: "ns"},
	}, cli, nil)

	g.Expect((&Actor{}).scaleIn(ctx, oc, 1)).To(Succeed())
	g.Expect(oc.owned).To(HaveLen(1))
	g.Expect(oc.owned[0].Name).To(Equal("claim-migrating"))

	stored := &v1alpha1.CNClaim{}
	g.Expect(cli.Get(context.Background(), client.ObjectKey{Namespace: "ns", Name: "claim-migrating"}, stored)).To(Succeed())
	err := cli.Get(context.Background(), client.ObjectKey{Namespace: "ns", Name: "claim-normal"}, &v1alpha1.CNClaim{})
	g.Expect(apierrors.IsNotFound(err)).To(BeTrue())
}

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
