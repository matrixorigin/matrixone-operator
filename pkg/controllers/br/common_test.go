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

package br

import (
	"reflect"
	"testing"

	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestBuildJobPreservesBRContainer(t *testing.T) {
	sidecars := []struct {
		name string
		sc   []corev1.Container
	}{
		{"nil", nil},
		{"empty", []corev1.Container{}},
		{"one", []corev1.Container{{Name: "side-a", Image: "side:a"}}},
		{"two", []corev1.Container{{Name: "side-a", Image: "side:a"}, {Name: "side-b", Image: "side:b"}}},
	}
	for _, kind := range []string{"BackupJob", "RestoreJob"} {
		for _, tt := range sidecars {
			t.Run(kind+"/"+tt.name, func(t *testing.T) {
				ov := overlayWithSidecars(tt.sc)
				before := ov.DeepCopy()
				o := newJobObject(kind, ov)
				inject := func(c *corev1.Container) {
					c.Env = []corev1.EnvVar{{Name: "INJECTED", Value: "1"}}
				}
				j1 := buildJob(o, "br:test", "do-work", inject)
				j2 := buildJob(o, "br:test", "do-work", inject)
				if !reflect.DeepEqual(ov, before) {
					t.Fatalf("overlay mutated: got %#v want %#v", ov, before)
				}
				if !reflect.DeepEqual(j1.Spec.Template.Spec, j2.Spec.Template.Spec) {
					t.Fatalf("repeated build changed pod spec")
				}
				assertBRJob(t, j1, ov)
			})
		}
	}
}

func newJobObject(kind string, ov *v1alpha1.Overlay) JobObject {
	meta := metav1.ObjectMeta{Name: "job", Namespace: "ns"}
	if kind == "RestoreJob" {
		return &v1alpha1.RestoreJob{ObjectMeta: meta, Overlay: ov}
	}
	return &v1alpha1.BackupJob{ObjectMeta: meta, Spec: v1alpha1.BackupJobSpec{Overlay: ov}}
}

func overlayWithSidecars(sc []corev1.Container) *v1alpha1.Overlay {
	return &v1alpha1.Overlay{
		MainContainerOverlay: v1alpha1.MainContainerOverlay{
			EnvFrom:      []corev1.EnvFromSource{{Prefix: "P_", ConfigMapRef: &corev1.ConfigMapEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: "br-env"}}}},
			Env:          []corev1.EnvVar{{Name: "FROM_OVERLAY", Value: "ov"}},
			VolumeMounts: []corev1.VolumeMount{{Name: "data", MountPath: "/data"}},
		},
		Volumes: []corev1.Volume{{
			Name:         "data",
			VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
		}},
		InitContainers:     []corev1.Container{{Name: "init", Image: "init:1"}},
		SidecarContainers:  sc,
		ServiceAccountName: "br-sa",
	}
}

func assertBRJob(t *testing.T, job *batchv1.Job, ov *v1alpha1.Overlay) {
	t.Helper()
	spec := job.Spec.Template.Spec
	wantBR := corev1.Container{
		Name:         "br",
		Image:        "br:test",
		Command:      []string{"/cmdrest", "--", "do-work"},
		EnvFrom:      ov.EnvFrom,
		Env:          []corev1.EnvVar{{Name: "INJECTED", Value: "1"}},
		VolumeMounts: ov.VolumeMounts,
	}
	wantC := append([]corev1.Container{wantBR}, ov.SidecarContainers...)
	if !reflect.DeepEqual(spec.Containers, wantC) {
		t.Errorf("containers: got %#v want %#v", spec.Containers, wantC)
	}
	if !reflect.DeepEqual(spec.InitContainers, ov.InitContainers) {
		t.Errorf("init: got %#v want %#v", spec.InitContainers, ov.InitContainers)
	}
	if !reflect.DeepEqual(spec.Volumes, ov.Volumes) {
		t.Errorf("volumes: got %#v want %#v", spec.Volumes, ov.Volumes)
	}
	if spec.ServiceAccountName != ov.ServiceAccountName {
		t.Errorf("sa: got %q want %q", spec.ServiceAccountName, ov.ServiceAccountName)
	}
}
func TestBuildJobWithoutOverlay(t *testing.T) {
	for _, kind := range []string{"BackupJob", "RestoreJob"} {
		t.Run(kind, func(t *testing.T) {
			job := buildJob(newJobObject(kind, nil), "br:test", "do-work", func(c *corev1.Container) {
				c.Env = []corev1.EnvVar{{Name: "INJECTED", Value: "1"}}
			})
			want := corev1.PodSpec{
				Containers: []corev1.Container{{
					Name: "br", Image: "br:test", Command: []string{"/cmdrest", "--", "do-work"},
					Env: []corev1.EnvVar{{Name: "INJECTED", Value: "1"}},
				}},
				RestartPolicy: corev1.RestartPolicyNever,
			}
			if !reflect.DeepEqual(job.Spec.Template.Spec, want) {
				t.Fatalf("pod without overlay: got %#v want %#v", job.Spec.Template.Spec, want)
			}
			if *job.Spec.Parallelism != 1 || *job.Spec.Completions != 1 || *job.Spec.BackoffLimit != 0 {
				t.Fatal("job execution policy changed")
			}
		})
	}
}
