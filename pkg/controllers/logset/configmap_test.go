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
package logset

import (
	"reflect"
	"testing"

	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	kruisev1 "github.com/openkruise/kruise-api/apps/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_gossipSeeds(t *testing.T) {
	ls := &v1alpha1.LogSet{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "test",
		},
	}
	tests := []struct {
		name             string
		replicas         int32
		reservedOrdinals []int
		want             []string
	}{
		{
			name:             "basic",
			replicas:         3,
			reservedOrdinals: nil,
			want: []string{
				"test-log-0.test-log-headless.default.svc:32002",
				"test-log-1.test-log-headless.default.svc:32002",
				"test-log-2.test-log-headless.default.svc:32002",
			},
		},
		{
			name:             "failover",
			replicas:         3,
			reservedOrdinals: []int{1},
			want: []string{
				"test-log-0.test-log-headless.default.svc:32002",
				"test-log-2.test-log-headless.default.svc:32002",
				"test-log-3.test-log-headless.default.svc:32002",
			},
		},
		{
			name:             "irrelevant reservation",
			replicas:         3,
			reservedOrdinals: []int{3},
			want: []string{
				"test-log-0.test-log-headless.default.svc:32002",
				"test-log-1.test-log-headless.default.svc:32002",
				"test-log-2.test-log-headless.default.svc:32002",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sts := &kruisev1.StatefulSet{
				Spec: kruisev1.StatefulSetSpec{
					Replicas:        &tt.replicas,
					ReserveOrdinals: tt.reservedOrdinals,
				},
			}
			if got := gossipSeeds(ls, sts); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("gossipSeeds() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_HaKeeperSvcAddrs(t *testing.T) {
	ls := &v1alpha1.LogSet{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "test",
		},
		Spec: v1alpha1.LogSetSpec{
			PodSet: v1alpha1.PodSet{Replicas: 3},
		},
	}
	tests := []struct {
		name             string
		reservedOrdinals []int
		want             []string
	}{
		{
			name:             "basic, no failover",
			reservedOrdinals: nil,
			want: []string{
				"test-log-0.test-log-headless.default.svc:32001",
				"test-log-1.test-log-headless.default.svc:32001",
				"test-log-2.test-log-headless.default.svc:32001",
			},
		},
		{
			// regression test for #596: HaKeeperAdds() previously ignored
			// ReserveOrdinals entirely and always returned [log-0, log-1, log-2],
			// pointing at the dead log-1 and missing the newly created log-3.
			name:             "log-1 failover creates log-3, hole must be skipped",
			reservedOrdinals: []int{1},
			want: []string{
				"test-log-0.test-log-headless.default.svc:32001",
				"test-log-2.test-log-headless.default.svc:32001",
				"test-log-3.test-log-headless.default.svc:32001",
			},
		},
		{
			name:             "log-0 failover creates log-3, hole must be skipped",
			reservedOrdinals: []int{0},
			want: []string{
				"test-log-1.test-log-headless.default.svc:32001",
				"test-log-2.test-log-headless.default.svc:32001",
				"test-log-3.test-log-headless.default.svc:32001",
			},
		},
		{
			name:             "reservation outside current window is a no-op",
			reservedOrdinals: []int{3},
			want: []string{
				"test-log-0.test-log-headless.default.svc:32001",
				"test-log-1.test-log-headless.default.svc:32001",
				"test-log-2.test-log-headless.default.svc:32001",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HaKeeperSvcAddrs(ls, tt.reservedOrdinals); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("HaKeeperSvcAddrs() = %v, want %v", got, tt.want)
			}
		})
	}
}
