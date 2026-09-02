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

package common

import (
	"testing"

	kruisev1 "github.com/openkruise/kruise-api/apps/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

func TestLogSetReserveOrdinalsChangedPredicate(t *testing.T) {
	p := LogSetReserveOrdinalsChangedPredicate()
	oldSts := &kruisev1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: "log"}}
	newSts := oldSts.DeepCopy()
	newSts.Spec.ReserveOrdinals = []int{1}

	if p.Create(event.CreateEvent{Object: oldSts}) {
		t.Fatal("StatefulSet creation must not change dependent lifecycle ordering")
	}
	if p.Delete(event.DeleteEvent{Object: oldSts}) {
		t.Fatal("StatefulSet deletion must not change dependent lifecycle ordering")
	}

	if !p.Update(event.UpdateEvent{ObjectOld: oldSts, ObjectNew: newSts}) {
		t.Fatal("reserveOrdinals change must trigger reconciliation")
	}

	statusOnly := newSts.DeepCopy()
	statusOnly.Status.ReadyReplicas = 1
	if p.Update(event.UpdateEvent{ObjectOld: newSts, ObjectNew: statusOnly}) {
		t.Fatal("status-only change must not trigger dependent reconciliation")
	}
}
