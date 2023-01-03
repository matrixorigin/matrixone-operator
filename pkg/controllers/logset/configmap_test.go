// Copyright 2023 Matrix Origin
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
	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	kruisev1 "github.com/openkruise/kruise-api/apps/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"reflect"
	"testing"
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
