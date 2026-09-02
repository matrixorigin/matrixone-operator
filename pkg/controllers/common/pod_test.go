// Copyright 2025 Matrix Origin
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package common

import (
	"testing"

	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

func TestPodStatusChangedPredicate_Update(t *testing.T) {
	tests := []struct {
		name  string
		event event.UpdateEvent
		want  bool
	}{
		{
			name: "pod status changed from Pending to Running",
			event: event.UpdateEvent{
				ObjectOld: &v1.Pod{Status: v1.PodStatus{Phase: v1.PodPending}},
				ObjectNew: &v1.Pod{Status: v1.PodStatus{Phase: v1.PodRunning}},
			},
			want: true,
		},
		{
			name: "pod status unchanged",
			event: event.UpdateEvent{
				ObjectOld: &v1.Pod{Status: v1.PodStatus{Phase: v1.PodRunning}},
				ObjectNew: &v1.Pod{Status: v1.PodStatus{Phase: v1.PodRunning}},
			},
			want: false,
		},
		{
			name: "nil old object",
			event: event.UpdateEvent{
				ObjectOld: nil,
				ObjectNew: &v1.Pod{Status: v1.PodStatus{Phase: v1.PodRunning}},
			},
			want: false,
		},
		{
			name: "nil new object",
			event: event.UpdateEvent{
				ObjectOld: &v1.Pod{Status: v1.PodStatus{Phase: v1.PodRunning}},
				ObjectNew: nil,
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := PodStatusChangedPredicate{}
			got := p.Update(tt.event)
			if got != tt.want {
				t.Errorf("Update() = %v, want %v", got, tt.want)
			}
		})
	}
}
