// Copyright 2023 Matrix Origin
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

package v1alpha1

import (
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"
)

func TestGetCNPodUUID(t *testing.T) {
	pod := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "default-cn-0",
			Namespace: "test",
		},
		Spec: corev1.PodSpec{
			Subdomain: "default-cn-headless",
		},
	}
	tests := []struct {
		name     string
		dnsBased bool
		pod      *corev1.Pod
		want     string
		wantErr  bool
	}{{
		name:     "DNS Based",
		dnsBased: true,
		pod:      &pod,
		want:     "64396564-3061-3238-3164-363835623561",
	}, {
		name:     "Ordinal Based",
		dnsBased: false,
		pod:      &pod,
		want:     "00000000-0000-0000-0000-200000000000",
	}}
	g := NewGomegaWithT(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetCNPodUUID(tt.pod, tt.dnsBased)
			if !tt.wantErr {
				g.Expect(err).To(Succeed())
			}
			g.Expect(got).To(Equal(tt.want))
		})
	}
}
