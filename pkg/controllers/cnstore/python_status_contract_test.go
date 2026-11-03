// Copyright 2026 Matrix Origin
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

package cnstore

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	"github.com/matrixorigin/matrixone-operator/pkg/querycli"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestMOFailureStatusesRemainDiagnosable(t *testing.T) {
	cn := &v1alpha1.CNSet{Spec: v1alpha1.CNSetSpec{ConfigThatChangeCNSpec: v1alpha1.ConfigThatChangeCNSpec{
		UDFWorker: &v1alpha1.UDFWorkerPolicy{Enabled: true},
	}}}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "cn-0", Namespace: "mo", UID: "pod-uid"},
		Spec:       corev1.PodSpec{Subdomain: "cn-headless"},
	}
	pod.Annotations = map[string]string{
		v1alpha1.UDFWorkerGenerationAnno: v1alpha1.UDFWorkerPolicyGeneration(cn.Spec.UDFWorker),
	}
	// Values are the current MO runtime contract at feb3849849, not legacy worker codes.

	raw, err := os.ReadFile("../../querycli/testdata/python_status_wire.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures []struct{ Class, Reason, Wire string }
	if err := json.Unmarshal(raw, &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, input := range fixtures {
		t.Run(input.Reason, func(t *testing.T) {
			observed := &querycli.PythonUDFStatus{
				CNUUID: v1alpha1.GetCNPodUUID(pod), Language: "python",
				Enabled: input.Class != "DISABLED", AllowUnisolated: input.Class != "NOT_ALLOWED", Ready: input.Class == "", LeaseEpoch: 1,
				ErrorClass: input.Class, Reason: input.Reason,
			}
			c := &withCNSet{Controller: &Controller{queryCli: &fakeQueryClient{pythonStatus: observed}}, cn: cn}
			actual := c.queryPythonUDFStatus(context.Background(), pod, "cn-0:6004")

			if input.Class == "DISABLED" || input.Class == "NOT_ALLOWED" {
				if actual.ErrorClass != v1alpha1.UDFWorkerStatusErrorPolicyMismatch {
					t.Fatalf("wrong policy failure: %#v", actual)
				}
				return
			}
			encoded, err := json.Marshal(actual)
			if err != nil {
				t.Fatal(err)
			}
			var annotation v1alpha1.UDFWorkerPodStatus
			if err := json.Unmarshal(encoded, &annotation); err != nil {
				t.Fatal(err)
			}
			if err := v1alpha1.ValidateUDFWorkerPodStatus(&annotation); err != nil {
				t.Fatal(err)
			}
			if actual.ErrorClass != input.Class || actual.Reason != input.Reason {
				t.Fatalf("current MO contract lost: input=%s/%s output=%s/%s", input.Class, input.Reason, actual.ErrorClass, actual.Reason)
			}
		})
	}
}

func TestCNStatusCannotImpersonateObservationFailure(t *testing.T) {
	for _, class := range []string{"QUERY_UNAVAILABLE", "IDENTITY_MISMATCH", "POLICY_MISMATCH", "INVALID_STATUS", "STALE_STATUS", "READY", "future"} {
		if err := validatePythonUDFStatusBounds(&querycli.PythonUDFStatus{ErrorClass: class}); err == nil {
			t.Errorf("accepted %s from runtime", class)
		}
	}
}
