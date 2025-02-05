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

package v1alpha1

import (
	"testing"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	}{{
		name:     "DNS Based",
		dnsBased: true,
		pod:      &pod,
		want:     "64396564-3061-3238-3164-363835623561",
	}}
	g := NewGomegaWithT(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetCNPodUUID(tt.pod)
			g.Expect(got).To(Equal(tt.want))
		})
	}
}

func TestGetImageTag(t *testing.T) {
	tests := []struct {
		name  string
		image string
		want  string
	}{
		{
			name:  "Regular image with tag",
			image: "matrixorigin/matrixone:1.1.1",
			want:  "1.1.1",
		},
		{
			name:  "Image with registry and tag",
			image: "registry.example.com/matrixorigin/matrixone:latest",
			want:  "latest",
		},
		{
			name:  "Image without tag",
			image: "matrixorigin/matrixone",
			want:  "",
		},
		{
			name:  "Image with multiple colons",
			image: "registry.example.com:5000/matrixorigin/matrixone:v1.0.0",
			want:  "v1.0.0",
		},
		{
			name:  "Empty string",
			image: "",
			want:  "",
		},
	}

	g := NewGomegaWithT(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getImageTag(tt.image)
			g.Expect(got).To(Equal(tt.want))
		})
	}
}
