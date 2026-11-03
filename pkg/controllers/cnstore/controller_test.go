// Copyright 2026 Matrix Origin
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cnstore

import (
	"testing"

	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	"github.com/matrixorigin/matrixone-operator/pkg/controllers/common"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

func TestAnnotationChangedExcludeStatsIgnoresControllerOwnedStats(t *testing.T) {
	tests := []struct {
		name string
		key  string
		want bool
	}{
		{name: "deletion cost", key: common.DeletionCostAnno, want: false},
		{name: "store connection", key: v1alpha1.StoreConnectionAnno, want: false},
		{name: "store score", key: v1alpha1.StoreScoreAnno, want: false},
		{name: "python worker status", key: v1alpha1.UDFWorkerStatusAnno, want: false},
		{name: "external annotation", key: "example.com/change", want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oldPod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{}}}
			newPod := oldPod.DeepCopy()
			newPod.Annotations[tt.key] = "updated"
			got := (annotationChangedExcludeStats{}).Update(event.UpdateEvent{
				ObjectOld: oldPod,
				ObjectNew: newPod,
			})
			if got != tt.want {
				t.Fatalf("predicate result = %v, want %v", got, tt.want)
			}
		})
	}
}
