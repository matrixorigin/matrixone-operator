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

package common

import (
	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"reflect"
	"sort"
	"testing"
)

// returns true when pod spec container image differs from pod status container image
func TestNeedUpdateImage_DifferentImages(t *testing.T) {
	pod := &corev1.Pod{
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{Name: "container1", Image: "image:v2"},
			},
		},
		Status: corev1.PodStatus{
			ContainerStatuses: []corev1.ContainerStatus{
				{Name: "container1", Image: "image:v1"},
			},
		},
	}

	if !NeedUpdateImage(pod) {
		t.Errorf("Expected true, got false")
	}
}

func TestGenDeletionCostSorting(t *testing.T) {
	type namedStoreScore struct {
		Name  string
		Score StoreScore
	}
	tests := []struct {
		name   string
		scores []namedStoreScore
		want   []string
	}{
		{
			name: "Basic sorting",
			scores: []namedStoreScore{
				{"A", StoreScore{SessionCount: 5, PipelineCount: 3, Phase: v1alpha1.CNPodPhaseBound}},
				{"B", StoreScore{SessionCount: 2, PipelineCount: 1, Phase: v1alpha1.CNPodPhaseIdle}},
				{"C", StoreScore{SessionCount: 10, PipelineCount: 5, Phase: v1alpha1.CNPodPhaseDraining}},
			},
			want: []string{"B", "C", "A"},
		},
		{
			name: "Sorting with same scores",
			scores: []namedStoreScore{
				{"A", StoreScore{SessionCount: 5, PipelineCount: 3, Phase: v1alpha1.CNPodPhaseBound}},
				{"B", StoreScore{SessionCount: 5, PipelineCount: 3, Phase: v1alpha1.CNPodPhaseBound}},
				{"C", StoreScore{SessionCount: 2, PipelineCount: 1, Phase: v1alpha1.CNPodPhaseIdle}},
			},
			want: []string{"C", "A", "B"},
		},
		{
			name: "Sorting with different phases",
			scores: []namedStoreScore{
				{"A", StoreScore{SessionCount: 5, PipelineCount: 3, Phase: v1alpha1.CNPodPhaseBound}},
				{"B", StoreScore{SessionCount: 5, PipelineCount: 3, Phase: v1alpha1.CNPodPhaseIdle}},
				{"C", StoreScore{SessionCount: 5, PipelineCount: 3, Phase: v1alpha1.CNPodPhaseDraining}},
				{"D", StoreScore{SessionCount: 5, PipelineCount: 3, Phase: v1alpha1.CNPodPhaseTerminating}},
			},
			want: []string{"B", "D", "C", "A"},
		},
		{
			name: "Sorting with different session count",
			scores: []namedStoreScore{
				{"A", StoreScore{SessionCount: 2, PipelineCount: 3, Phase: v1alpha1.CNPodPhaseBound}},
				{"B", StoreScore{SessionCount: 3, PipelineCount: 3, Phase: v1alpha1.CNPodPhaseBound}},
				{"C", StoreScore{SessionCount: 5, PipelineCount: 3, Phase: v1alpha1.CNPodPhaseBound}},
			},
			want: []string{"A", "B", "C"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sort.Slice(tt.scores, func(i, j int) bool {
				return tt.scores[i].Score.GenDeletionCost() < tt.scores[j].Score.GenDeletionCost()
			})

			got := make([]string, len(tt.scores))
			for i, score := range tt.scores {
				got[i] = score.Name
			}

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GenDeletionCost sorting = %v, want %v", got, tt.want)
			}
		})
	}
}
