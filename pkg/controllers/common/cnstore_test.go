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

package common

import (
	"testing"
	"time"

	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestStoreScoreIsSafeToReclaim(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name  string
		score StoreScore
		safe  bool
	}{
		{
			name: "complete zero observation",
			score: StoreScore{
				SessionObserved: true, PipelineObserved: true, ReplicaObserved: true,
				StartedTime: &now,
			},
			safe: true,
		},
		{
			name: "active pipeline without ordinary sessions",
			score: StoreScore{
				PipelineCount:   1,
				SessionObserved: true, PipelineObserved: true, ReplicaObserved: true,
				StartedTime: &now,
			},
		},
		{
			name: "active session",
			score: StoreScore{
				SessionCount:    1,
				SessionObserved: true, PipelineObserved: true, ReplicaObserved: true,
				StartedTime: &now,
			},
		},
		{
			name: "active replica",
			score: StoreScore{
				ReplicaCount:    1,
				SessionObserved: true, PipelineObserved: true, ReplicaObserved: true,
				StartedTime: &now,
			},
		},
		{
			name:  "legacy zero without observation evidence",
			score: StoreScore{StartedTime: &now},
		},
		{
			name: "pipeline query failed",
			score: StoreScore{
				SessionObserved: true, ReplicaObserved: true,
				StartedTime: &now,
			},
		},
		{
			name: "session query failed",
			score: StoreScore{
				PipelineObserved: true, ReplicaObserved: true,
				StartedTime: &now,
			},
		},
		{
			name: "replica query failed",
			score: StoreScore{
				SessionObserved: true, PipelineObserved: true,
				StartedTime: &now,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.score.IsSafeToReclaim(); got != tt.safe {
				t.Fatalf("IsSafeToReclaim() = %v, want %v", got, tt.safe)
			}
		})
	}
}

func TestGetStoreScoreLegacyAnnotationsAreUnsafe(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		{name: "legacy JSON zero", value: `{"sessionCount":0,"pipelineCount":0,"replicaCount":0}`},
		{name: "legacy integer zero", value: "0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{v1alpha1.StoreScoreAnno: tt.value}}}
			score, err := GetStoreScore(pod)
			if err != nil {
				t.Fatalf("GetStoreScore() error = %v", err)
			}
			if score.IsSafeToReclaim() {
				t.Fatal("legacy score without observation evidence must not authorize reclaim")
			}
		})
	}
}

func TestStoreScoreBeginObservationInvalidatesStaleZero(t *testing.T) {
	started := time.Now()
	score := StoreScore{
		SessionObserved:  true,
		PipelineObserved: true,
		ReplicaObserved:  true,
		StartedTime:      &started,
	}
	if !score.IsSafeToReclaim() {
		t.Fatal("precondition: complete zero observation should be safe")
	}

	score.BeginObservation(&started)
	if score.IsSafeToReclaim() {
		t.Fatal("a prior zero must not authorize reclaim in a new observation round")
	}
}

func TestStoreScoreBeginObservationResetsRestartedCN(t *testing.T) {
	previousStart := time.Now().Add(-time.Minute)
	currentStart := time.Now()
	score := StoreScore{
		SessionCount:     3,
		PipelineCount:    2,
		ReplicaCount:     1,
		SessionObserved:  true,
		PipelineObserved: true,
		ReplicaObserved:  true,
		StartedTime:      &previousStart,
	}

	score.BeginObservation(&currentStart)
	if score.SessionCount != 0 || score.PipelineCount != 0 || score.ReplicaCount != 0 {
		t.Fatalf("restart did not reset counts: %+v", score)
	}
	if score.SessionObserved || score.PipelineObserved || score.ReplicaObserved {
		t.Fatalf("restart retained observation validity: %+v", score)
	}
	if score.StartedTime == nil || !score.StartedTime.Equal(currentStart) {
		t.Fatalf("StartedTime = %v, want %v", score.StartedTime, currentStart)
	}
}

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
