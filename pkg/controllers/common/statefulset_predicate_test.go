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

	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	kruisev1 "github.com/openkruise/kruise-api/apps/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

func TestLogSetReserveOrdinalsChangedPredicate(t *testing.T) {
	p := LogSetReserveOrdinalsChangedPredicate()
	logSet := &v1alpha1.LogSet{ObjectMeta: metav1.ObjectMeta{Namespace: "provider", Name: "log", UID: "log-uid"}}
	oldSts := &kruisev1.StatefulSet{ObjectMeta: metav1.ObjectMeta{
		Namespace: "provider",
		Name:      "log-log",
		OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(logSet,
			v1alpha1.GroupVersion.WithKind("LogSet"))},
	}}
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

	notLogSet := newSts.DeepCopy()
	notLogSet.OwnerReferences[0].Kind = "DNSet"
	if p.Update(event.UpdateEvent{ObjectOld: oldSts, ObjectNew: notLogSet}) {
		t.Fatal("StatefulSets not controlled by a LogSet must be ignored")
	}
}

func TestReferencesLogSet(t *testing.T) {
	key := client.ObjectKey{Namespace: "provider", Name: "log"}
	if !ReferencesLogSet(v1alpha1.LogSetRef{LogSet: &v1alpha1.LogSet{ObjectMeta: metav1.ObjectMeta{
		Namespace: "provider", Name: "log",
	}}}, "consumer", key) {
		t.Fatal("explicit cross-namespace reference must match")
	}
	if !ReferencesLogSet(v1alpha1.LogSetRef{LogSet: &v1alpha1.LogSet{ObjectMeta: metav1.ObjectMeta{
		Name: "log",
	}}}, "provider", key) {
		t.Fatal("empty namespace must fall back to the dependent namespace")
	}
	if ReferencesLogSet(v1alpha1.LogSetRef{LogSet: &v1alpha1.LogSet{ObjectMeta: metav1.ObjectMeta{
		Namespace: "other", Name: "log",
	}}}, "consumer", key) {
		t.Fatal("same-name LogSet in another namespace must not match")
	}
}
