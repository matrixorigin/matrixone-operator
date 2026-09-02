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

	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
)

func Test_syncPodTemplate(t *testing.T) {
	tests := []struct {
		name     string
		task     *SyncMOPodTask
		expectFn func(g Gomega, task *SyncMOPodTask)
	}{{
		name: "overlay sidecar",
		task: &SyncMOPodTask{
			PodSet: &v1alpha1.PodSet{
				MainContainer: v1alpha1.MainContainer{
					Image: "test:v1.2.3",
				},
				Overlay: &v1alpha1.Overlay{
					SidecarContainers: []corev1.Container{{
						Name:  "sidecar",
						Image: "sidecar",
					}},
				},
				Replicas: 2,
			},
			TargetTemplate: &corev1.PodTemplateSpec{},
			StorageProvider: &v1alpha1.SharedStorageProvider{
				S3: &v1alpha1.S3Provider{
					Path: "test",
				},
			},
		},
		expectFn: func(g Gomega, task *SyncMOPodTask) {
			g.Expect(task.TargetTemplate.Spec.Containers).To(HaveLen(2))
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewGomegaWithT(t)
			syncPodTemplate(tt.task)
			tt.expectFn(g, tt.task)
		})
	}
}
