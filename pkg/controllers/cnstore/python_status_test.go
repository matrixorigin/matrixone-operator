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
	"testing"

	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	"github.com/matrixorigin/matrixone-operator/pkg/querycli"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestQueryPythonUDFStatusRequiresCurrentRuntimePolicy(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "cn-0", Namespace: "ns", UID: "pod-uid"},
		Spec:       corev1.PodSpec{Subdomain: "cn-headless"},
	}
	cn := &v1alpha1.CNSet{Spec: v1alpha1.CNSetSpec{
		ConfigThatChangeCNSpec: v1alpha1.ConfigThatChangeCNSpec{
			UDFWorker: &v1alpha1.UDFWorkerPolicy{Enabled: true},
		},
	}}
	expectedUID := v1alpha1.GetCNPodUUID(pod)
	pod.Annotations = map[string]string{
		v1alpha1.UDFWorkerGenerationAnno: v1alpha1.UDFWorkerPolicyGeneration(cn.Spec.UDFWorker),
	}
	tests := []struct {
		name       string
		status     *querycli.PythonUDFStatus
		generation string
		wantReady  bool
		wantError  string
		wantReason string
	}{
		{
			name: "enabled and unisolated",
			status: &querycli.PythonUDFStatus{
				CNUUID: expectedUID, Language: "python", Enabled: true,
				AllowUnisolated: true, Ready: true, LeaseEpoch: 1,
			},
			wantReady: true,
		},
		{
			name: "old rendered generation is rejected",
			status: &querycli.PythonUDFStatus{
				CNUUID: expectedUID, Language: "python", Enabled: true,
				AllowUnisolated: true, Ready: true, LeaseEpoch: 1,
			},
			generation: "old-generation",
			wantError:  v1alpha1.UDFWorkerStatusErrorIdentityMismatch,
			wantReason: v1alpha1.UDFWorkerStatusReasonIdentityMismatch,
		},
		{
			name: "runtime disabled",
			status: &querycli.PythonUDFStatus{
				CNUUID: expectedUID, Language: "python", Enabled: false,
				AllowUnisolated: true, Ready: true, LeaseEpoch: 1,
			},
			wantError:  v1alpha1.UDFWorkerStatusErrorPolicyMismatch,
			wantReason: v1alpha1.UDFWorkerStatusReasonPolicyMismatch,
		},
		{
			name: "unisolated gate denied",
			status: &querycli.PythonUDFStatus{
				CNUUID: expectedUID, Language: "python", Enabled: true,
				AllowUnisolated: false, Ready: true, LeaseEpoch: 1,
			},
			wantError:  v1alpha1.UDFWorkerStatusErrorPolicyMismatch,
			wantReason: v1alpha1.UDFWorkerStatusReasonPolicyMismatch,
		},
		{
			name: "oversized capability list is rejected",
			status: &querycli.PythonUDFStatus{
				CNUUID: expectedUID, Language: "python", Enabled: true,
				AllowUnisolated: true, Ready: true, LeaseEpoch: 1,
				Modes: make([]string, v1alpha1.UDFWorkerStatusMaxListItems+1),
			},
			wantError:  v1alpha1.UDFWorkerStatusErrorInvalid,
			wantReason: v1alpha1.UDFWorkerStatusReasonInvalid,
		},
		{
			name: "invalid error class is rejected",
			status: &querycli.PythonUDFStatus{
				CNUUID: expectedUID, Language: "python", Enabled: true,
				AllowUnisolated: true, ErrorClass: "bad reason/with spaces",
			},
			wantError:  v1alpha1.UDFWorkerStatusErrorInvalid,
			wantReason: v1alpha1.UDFWorkerStatusReasonInvalid,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testPod := pod.DeepCopy()
			if tt.generation != "" {
				testPod.Annotations[v1alpha1.UDFWorkerGenerationAnno] = tt.generation
			}
			c := &withCNSet{
				Controller: &Controller{queryCli: &fakeQueryClient{pythonStatus: tt.status}},
				cn:         cn,
			}
			got := c.queryPythonUDFStatus(context.Background(), testPod, "cn-0:6002")
			if got.Ready != tt.wantReady {
				t.Fatalf("ready = %v, want %v; status = %#v", got.Ready, tt.wantReady, got)
			}
			if got.ErrorClass != tt.wantError {
				t.Fatalf("error class = %q, want %q; status = %#v", got.ErrorClass, tt.wantError, got)
			}
			if got.Reason != tt.wantReason {
				t.Fatalf("reason = %q, want %q; status = %#v", got.Reason, tt.wantReason, got)
			}
		})
	}
}

func TestQueryPythonUDFStatusFailsClosedWhenWiringIsIncomplete(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "cn-0", Namespace: "ns", UID: "pod-uid"},
		Spec:       corev1.PodSpec{Subdomain: "cn-headless"},
	}
	cn := &v1alpha1.CNSet{Spec: v1alpha1.CNSetSpec{
		ConfigThatChangeCNSpec: v1alpha1.ConfigThatChangeCNSpec{
			UDFWorker: &v1alpha1.UDFWorkerPolicy{Enabled: true},
		},
	}}
	tests := []struct {
		name string
		cn   *v1alpha1.CNSet
		c    *withCNSet
		pod  *corev1.Pod
		want string
	}{
		{
			name: "nil query client",
			cn:   cn,
			c:    &withCNSet{Controller: &Controller{}, cn: cn},
			pod:  pod,
			want: v1alpha1.UDFWorkerStatusErrorQueryUnavailable,
		},
		{
			name: "nil cn set",
			c:    &withCNSet{Controller: &Controller{}},
			pod:  pod,
			want: "",
		},
		{
			name: "nil pod",
			cn:   cn,
			c:    &withCNSet{Controller: &Controller{}, cn: cn},
			want: v1alpha1.UDFWorkerStatusErrorInvalid,
		},
		{
			name: "nil receiver",
			cn:   cn,
			c:    nil,
			pod:  pod,
			want: v1alpha1.UDFWorkerStatusErrorQueryUnavailable,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.c.queryPythonUDFStatus(context.Background(), tt.pod, "cn-0:6002")
			if got.ErrorClass != tt.want {
				t.Fatalf("error class = %q, want %q; status = %#v", got.ErrorClass, tt.want, got)
			}
		})
	}
}
