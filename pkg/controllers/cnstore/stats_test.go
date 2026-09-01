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
	"context"
	"errors"
	"testing"
	"time"

	"github.com/blang/semver/v4"
	"github.com/go-logr/logr"
	"github.com/matrixorigin/matrixone-operator/pkg/controllers/common"
	querypb "github.com/matrixorigin/matrixone/pkg/pb/query"
)

type fakeQueryClient struct {
	pipelineCount int64
	replicaCount  int64
	sessionErr    error
	pipelineErr   error
	replicaErr    error

	pipelineCalls int
	replicaCalls  int
}

func (f *fakeQueryClient) ShowProcessList(context.Context, string) (*querypb.ShowProcessListResponse, error) {
	if f.sessionErr != nil {
		return nil, f.sessionErr
	}
	return &querypb.ShowProcessListResponse{}, nil
}

func (f *fakeQueryClient) GetPipelineInfo(context.Context, string) (*querypb.GetPipelineInfoResponse, error) {
	f.pipelineCalls++
	if f.pipelineErr != nil {
		return nil, f.pipelineErr
	}
	return &querypb.GetPipelineInfoResponse{Count: f.pipelineCount}, nil
}

func (f *fakeQueryClient) GetReplicaCount(context.Context, string) (querypb.GetReplicaCountResponse, error) {
	f.replicaCalls++
	if f.replicaErr != nil {
		return querypb.GetReplicaCountResponse{}, f.replicaErr
	}
	return querypb.GetReplicaCountResponse{Count: f.replicaCount}, nil
}

func TestCollectQueryStatsFailClosed(t *testing.T) {
	tests := []struct {
		name          string
		version       string
		query         *fakeQueryClient
		safe          bool
		pipelineCalls int
		replicaCalls  int
	}{
		{
			name:          "MO 4 complete zero",
			version:       "4.2.0",
			query:         &fakeQueryClient{},
			safe:          true,
			pipelineCalls: 1,
		},
		{
			name:          "MO 4 active AP pipeline",
			version:       "4.2.0",
			query:         &fakeQueryClient{pipelineCount: 1},
			pipelineCalls: 1,
		},
		{
			name:          "MO 4 pipeline query failure",
			version:       "4.2.0",
			query:         &fakeQueryClient{pipelineErr: errors.New("pipeline unavailable")},
			pipelineCalls: 1,
		},
		{
			name:          "session query failure",
			version:       "4.2.0",
			query:         &fakeQueryClient{sessionErr: errors.New("session unavailable")},
			pipelineCalls: 1,
		},
		{
			name:          "required replica query failure",
			version:       "2.0.0",
			query:         &fakeQueryClient{replicaErr: errors.New("replica unavailable")},
			pipelineCalls: 1,
			replicaCalls:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			started := time.Now()
			score := &common.StoreScore{StartedTime: &started}
			score.BeginObservation(&started)
			controller := &Controller{queryCli: tt.query}
			controller.collectQueryStats(score, "query-address", semver.MustParse(tt.version), &connectionDiagnosis{Logger: logr.Discard()})

			if got := score.IsSafeToReclaim(); got != tt.safe {
				t.Fatalf("IsSafeToReclaim() = %v, want %v; score=%+v", got, tt.safe, score)
			}
			if tt.query.pipelineCalls != tt.pipelineCalls {
				t.Fatalf("pipeline calls = %d, want %d", tt.query.pipelineCalls, tt.pipelineCalls)
			}
			if tt.query.replicaCalls != tt.replicaCalls {
				t.Fatalf("replica calls = %d, want %d", tt.query.replicaCalls, tt.replicaCalls)
			}
		})
	}
}
