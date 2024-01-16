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

package cnclaim

import (
	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"math/rand"
	"testing"

	. "github.com/onsi/gomega"
)

func Test_sortCNByPriority(t *testing.T) {
	tests := []struct {
		name  string
		c     *v1alpha1.CNClaim
		pods  []corev1.Pod
		order []string
	}{{
		name: "basic",
		c: &v1alpha1.CNClaim{
			Spec: v1alpha1.CNClaimSpec{
				OwnerName: pointer.String("set1"),
			},
		},
		pods: []corev1.Pod{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "not-claimed-but-older",
					Labels: map[string]string{
						v1alpha1.CNPodPhaseLabel: v1alpha1.CNPodPhaseIdle,
					},
					CreationTimestamp: metav1.Unix(0, 0),
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "previously-claimed",
					Labels: map[string]string{
						v1alpha1.CNPodPhaseLabel:   v1alpha1.CNPodPhaseIdle,
						v1alpha1.PodOwnerNameLabel: "set1",
					},
					CreationTimestamp: metav1.Unix(10, 0),
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "previously-claimed-by-other-set",
					Labels: map[string]string{
						v1alpha1.CNPodPhaseLabel:   v1alpha1.CNPodPhaseIdle,
						v1alpha1.PodOwnerNameLabel: "set2",
					},
					CreationTimestamp: metav1.Unix(10, 0),
				},
			},
		},
		order: []string{
			"previously-claimed",
			"not-claimed-but-older",
			"previously-claimed-by-other-set",
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rand.Shuffle(len(tt.pods), func(i, j int) {
				tt.pods[i], tt.pods[j] = tt.pods[j], tt.pods[i]
			})
			sortCNByPriority(tt.c, tt.pods)
			g := NewGomegaWithT(t)
			var res []string
			for _, po := range tt.pods {
				res = append(res, po.Name)
			}
			g.Expect(res).To(Equal(tt.order))
		})
	}
}
