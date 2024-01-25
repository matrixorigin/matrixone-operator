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

package cnpool

import (
	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"math/rand"
	"testing"
	"time"

	. "github.com/onsi/gomega"
)

func Test_sortPodByDeletionOrder(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name  string
		pods  []*corev1.Pod
		order []string
	}{{
		name: "basic",
		pods: []*corev1.Pod{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "idle-new",
					Labels: map[string]string{
						v1alpha1.CNPodPhaseLabel: v1alpha1.CNPodPhaseIdle,
					},
					CreationTimestamp: metav1.NewTime(now),
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "unknown",
					Labels: map[string]string{
						v1alpha1.CNPodPhaseLabel: v1alpha1.CNPodPhaseUnknown,
					},
					CreationTimestamp: metav1.NewTime(now),
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "idle-old",
					Labels: map[string]string{
						v1alpha1.CNPodPhaseLabel: v1alpha1.CNPodPhaseIdle,
					},
					CreationTimestamp: metav1.NewTime(now.Add(-time.Hour)),
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "terminating",
					Labels: map[string]string{
						v1alpha1.CNPodPhaseLabel: v1alpha1.CNPodPhaseTerminating,
					},
					CreationTimestamp: metav1.NewTime(now),
				},
			},
		},
		order: []string{
			"terminating",
			"unknown",
			"idle-new",
			"idle-old",
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rand.Shuffle(len(tt.pods), func(i, j int) {
				tt.pods[i], tt.pods[j] = tt.pods[j], tt.pods[i]
			})
			sortPodByDeletionOrder(tt.pods)
			g := NewGomegaWithT(t)
			var res []string
			for _, po := range tt.pods {
				res = append(res, po.Name)
			}
			g.Expect(res).To(Equal(tt.order))
		})
	}
}
