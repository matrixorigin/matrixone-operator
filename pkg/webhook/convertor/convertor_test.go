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

package convertor

import (
	apiscorev1 "k8s.io/kubernetes/pkg/apis/core/v1"
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/kubernetes/pkg/apis/core"
)

func TestConvertSlice(t *testing.T) {
	tolerations := []corev1.Toleration{
		{
			Key:      "key1",
			Operator: corev1.TolerationOpEqual,
			Value:    "value1",
			Effect:   corev1.TaintEffectNoSchedule,
		},
		{
			Key:      "key2",
			Operator: corev1.TolerationOpExists,
			Effect:   corev1.TaintEffectNoExecute,
		},
	}
	want := []core.Toleration{
		{
			Key:      "key1",
			Operator: core.TolerationOpEqual,
			Value:    "value1",
			Effect:   core.TaintEffectNoSchedule,
		},
		{
			Key:      "key2",
			Operator: core.TolerationOpExists,
			Effect:   core.TaintEffectNoExecute,
		},
	}

	tests := []struct {
		name        string
		tolerations []corev1.Toleration
		want        []core.Toleration
		wantErr     bool
	}{
		{
			name:        "test-muti-tolerations",
			tolerations: tolerations,
			want:        want,
			wantErr:     false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ConvertSlice(tt.tolerations, apiscorev1.Convert_v1_Toleration_To_core_Toleration)
			if (err != nil) != tt.wantErr {
				t.Errorf("ConvertTolerations() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ConvertTolerations() got = %v, want %v", got, tt.want)
			}
		})
	}
}
