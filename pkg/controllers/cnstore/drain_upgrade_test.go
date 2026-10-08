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
	"fmt"
	"testing"
	"time"

	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	"github.com/matrixorigin/matrixone-operator/pkg/controllers/common"
	"github.com/openkruise/kruise-api/apps/pub"
	kruise "github.com/openkruise/kruise-api/apps/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type transientCloneSetReader struct {
	client.Reader
	fail bool
}

func (r *transientCloneSetReader) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	if _, ok := obj.(*kruise.CloneSet); ok && r.fail {
		return fmt.Errorf("temporary CloneSet API read failure")
	}
	return r.Reader.Get(ctx, key, obj, opts...)
}

func TestObserveUpgradeTransientOwnerReadCanRetry(t *testing.T) {
	for _, prepared := range []bool{false, true} {
		t.Run(fmt.Sprintf("prepared-%v", prepared), func(t *testing.T) {
			f := newObserveFixture(t)
			p := f.read(t)
			p.Labels[pub.LifecycleStateKey] = string(pub.LifecycleStatePreparingUpdate)
			cs := bindTestCloneSet(p)
			if err := f.cli.Create(context.Background(), cs); err != nil {
				t.Fatal(err)
			}
			if err := f.cli.Update(context.Background(), p); err != nil {
				t.Fatal(err)
			}
			f.round(t) // Persist the original drain start time.
			if prepared {
				f.round(t) // Persist Prepared, before any lock RPC.
			}
			reader := &transientCloneSetReader{Reader: f.cli, fail: true}
			f.c.apiReader = reader
			p = f.round(t)
			if p.Annotations[drainRecoveryAnno] != "" {
				t.Fatal("transient owner read became permanent recovery")
			}
			a, err := readDrainAttempt(p)
			if err != nil || (prepared && (a == nil || a.Phase != drainPhasePrepared)) {
				t.Fatalf("transient owner read damaged the prepared attempt: %#v %v", a, err)
			}
			if len(f.lock.calls) != 0 {
				t.Fatal("lock drain started without upgrade ownership proof")
			}
			reader.fail = false
			for i := 0; i < 4; i++ {
				p = f.round(t)
			}
			a, err = readDrainAttempt(p)
			if err != nil || a == nil || a.Phase != drainPhaseCompleted || len(f.lock.calls) != 2 {
				t.Fatalf("owner read recovery did not complete the same drain: %#v %v, calls=%v", a, err, f.lock.calls)
			}
		})
	}
}

func TestObserveUpgradeCompletionTransientOwnerReadCanRetry(t *testing.T) {
	f := completedUpgradeFixture(t)
	reader := &transientCloneSetReader{Reader: f.cli, fail: true}
	f.c.apiReader = reader
	p := f.round(t)
	a, err := readDrainAttempt(p)
	if err != nil || a == nil || a.Phase != drainPhaseCompleted || p.Annotations[drainRecoveryAnno] != "" {
		t.Fatalf("transient owner read destroyed completed proof: %#v %v", a, err)
	}
	if cond := common.GetReadinessCondition(p, common.CNStoreReadiness); cond == nil || cond.Status != corev1.ConditionFalse {
		t.Fatal("transient owner read admitted business")
	}
	reader.fail = false
	for i := 0; i < 3; i++ {
		p = f.round(t)
	}
	if a, err := readDrainAttempt(p); err != nil || a != nil {
		t.Fatalf("completed upgrade did not recover after owner read: %#v %v", a, err)
	}
	if len(f.lock.calls) != 2 || !controllerutil.ContainsFinalizer(p, common.CNDrainingFinalizer) {
		t.Fatal("owner read retry repeated lock handshake or lost protection")
	}
}

func TestObserveUpgradeRequestedTransientOwnerReadCanRetry(t *testing.T) {
	f := newObserveFixture(t)
	p := f.read(t)
	p.Labels[pub.LifecycleStateKey] = string(pub.LifecycleStatePreparingUpdate)
	cs := bindTestCloneSet(p)
	if err := f.cli.Create(context.Background(), cs); err != nil {
		t.Fatal(err)
	}
	if err := f.cli.Update(context.Background(), p); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		p = f.round(t)
	}
	a, err := readDrainAttempt(p)
	if err != nil || a == nil || a.Phase != drainPhaseRequested || len(f.lock.calls) != 1 {
		t.Fatalf("test did not reach a persisted request: %#v %v, calls=%v", a, err, f.lock.calls)
	}
	reader := &transientCloneSetReader{Reader: f.cli, fail: true}
	f.c.apiReader = reader
	p = f.round(t)
	a, err = readDrainAttempt(p)
	if err != nil || a == nil || a.Phase != drainPhaseRequested || len(f.lock.calls) != 1 {
		t.Fatalf("transient owner read lost requested attempt: %#v %v, calls=%v", a, err, f.lock.calls)
	}
	reader.fail = false
	p = f.round(t)
	a, err = readDrainAttempt(p)
	if err != nil || a == nil || a.Phase != drainPhaseCompleted || len(f.lock.calls) != 2 {
		t.Fatalf("requested drain did not resume after owner read: %#v %v, calls=%v", a, err, f.lock.calls)
	}
}

func TestObserveUpgradeUnobservedOwnerCanCatchUp(t *testing.T) {
	f := completedUpgradeFixture(t)
	p := f.read(t)
	cs := &kruise.CloneSet{}
	if err := f.cli.Get(context.Background(), client.ObjectKey{Namespace: p.Namespace, Name: "cloneset"}, cs); err != nil {
		t.Fatal(err)
	}
	cs.Generation++
	if err := f.cli.Update(context.Background(), cs); err != nil {
		t.Fatal(err)
	}
	p = f.round(t)
	a, err := readDrainAttempt(p)
	if err != nil || a == nil || a.Phase != drainPhaseCompleted {
		t.Fatalf("unobserved owner destroyed completed proof: %#v %v", a, err)
	}
	if err := f.cli.Get(context.Background(), client.ObjectKeyFromObject(cs), cs); err != nil {
		t.Fatal(err)
	}
	cs.Status.ObservedGeneration = cs.Generation
	if err := f.cli.Update(context.Background(), cs); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		p = f.round(t)
	}
	if a, err := readDrainAttempt(p); err != nil || a != nil || len(f.lock.calls) != 2 {
		t.Fatalf("observed owner did not resume authorized upgrade: %#v %v, calls=%v", a, err, f.lock.calls)
	}
}

func bindTestCloneSet(pod *corev1.Pod) *kruise.CloneSet {
	controller := true
	pod.OwnerReferences = []metav1.OwnerReference{{APIVersion: kruise.SchemeGroupVersion.String(), Kind: "CloneSet", Name: "cloneset", UID: "cs-uid", Controller: &controller}}
	pod.Labels[appsv1.ControllerRevisionHashLabelKey] = "cloneset-source"
	return &kruise.CloneSet{ObjectMeta: metav1.ObjectMeta{Name: "cloneset", Namespace: pod.Namespace, UID: "cs-uid"},
		Status: kruise.CloneSetStatus{UpdateRevision: "cloneset-target"}}
}

func completedUpgradeFixture(t *testing.T) *observeFixture {
	t.Helper()
	f := newObserveFixture(t)
	p := f.read(t)
	p.Labels[pub.LifecycleStateKey] = string(pub.LifecycleStatePreparingUpdate)
	cs := bindTestCloneSet(p)
	if err := f.cli.Create(context.Background(), cs); err != nil {
		t.Fatal(err)
	}
	if err := f.cli.Update(context.Background(), p); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 4; i++ {
		f.round(t)
	}
	p = f.read(t)
	a, err := readDrainAttempt(p)
	if err != nil || a == nil || a.Phase != drainPhaseCompleted {
		t.Fatalf("not completed: %#v %v", a, err)
	}
	p.Labels[pub.LifecycleStateKey] = string(pub.LifecycleStateUpdated)
	p.Labels[appsv1.ControllerRevisionHashLabelKey] = "cloneset-target"
	state := pub.InPlaceUpdateState{Revision: "cloneset-target", UpdateTimestamp: metav1.NewTime(time.Unix(30, 0)), LastContainerStatuses: map[string]pub.InPlaceUpdateContainerStatus{v1alpha1.ContainerMain: {ImageID: "sha256:old"}}}
	raw, _ := json.Marshal(state)
	p.Annotations[pub.InPlaceUpdateStateKey] = string(raw)
	if err := f.cli.Update(context.Background(), p); err != nil {
		t.Fatal(err)
	}
	p.Status.ContainerStatuses[0].ContainerID = "containerd://main-2"
	p.Status.ContainerStatuses[0].ImageID = "sha256:new"
	p.Status.ContainerStatuses[0].State.Running.StartedAt = metav1.NewTime(time.Unix(31, 0))
	p.Status.ContainerStatuses[0].RestartCount = 1
	p.Status.Conditions = append(p.Status.Conditions, corev1.PodCondition{Type: pub.InPlaceUpdateReady, Status: corev1.ConditionTrue})
	if err := f.cli.Status().Update(context.Background(), p); err != nil {
		t.Fatal(err)
	}
	return f
}

func TestObserveUpgradeRestoresProtectionBeforeClearingProof(t *testing.T) {
	f := completedUpgradeFixture(t)
	p := f.round(t)
	a, err := readDrainAttempt(p)
	if err != nil || a == nil || a.Phase != drainPhaseCompleted || !controllerutil.ContainsFinalizer(p, common.CNDrainingFinalizer) {
		t.Fatalf("proof/protection not retained: %#v %v", a, err)
	}
	p = f.round(t)
	a, err = readDrainAttempt(p)
	if err != nil || a != nil || !controllerutil.ContainsFinalizer(p, common.CNDrainingFinalizer) {
		t.Fatalf("proof not cleared with protection: %#v %v", a, err)
	}
	if len(f.lock.calls) != 2 {
		t.Fatalf("duplicate lock handshake: %v", f.lock.calls)
	}
	// Updated is not yet business-admissible even after proof cleanup.
	p = f.round(t)
	if condition := common.GetReadinessCondition(p, common.CNStoreReadiness); condition == nil || condition.Status != corev1.ConditionFalse {
		t.Fatal("admitted business before Kruise Normal")
	}
	p.Labels[pub.LifecycleStateKey] = string(pub.LifecycleStateNormal)
	if err := f.cli.Update(context.Background(), p); err != nil {
		t.Fatal(err)
	}
	p = f.round(t)
	if condition := common.GetReadinessCondition(p, common.CNStoreReadiness); condition == nil || condition.Status != corev1.ConditionTrue {
		t.Fatalf("valid upgrade never restored business readiness: %#v", condition)
	}
}

func TestObserveUpgradeShortRevisionLabel(t *testing.T) {
	f := completedUpgradeFixture(t)
	p := f.read(t)
	p.Labels[appsv1.ControllerRevisionHashLabelKey] = "target"
	if err := f.cli.Update(context.Background(), p); err != nil {
		t.Fatal(err)
	}
	f.round(t)
	p = f.round(t)
	if a, err := readDrainAttempt(p); err != nil || a != nil {
		t.Fatalf("short revision completion blocked: %#v %v", a, err)
	}
}

func TestObserveUpgradeRejectsUnrelatedCompletion(t *testing.T) {
	for _, fault := range []string{"revision", "record", "uuid", "container", "owner", "unhealthy"} {
		t.Run(fault, func(t *testing.T) {
			f := completedUpgradeFixture(t)
			p := f.read(t)
			switch fault {
			case "revision":
				p.Labels[appsv1.ControllerRevisionHashLabelKey] = "other"
			case "record":
				delete(p.Annotations, pub.InPlaceUpdateStateKey)
			case "uuid":
				p.Spec.Subdomain = "another"
			case "owner":
				p.OwnerReferences[0].UID = "other"
			case "container":
				p.Status.ContainerStatuses[0].ContainerID = "containerd://main-1"
			case "unhealthy":
				p.Status.ContainerStatuses[0].Ready = false
			}
			if fault == "container" || fault == "unhealthy" {
				if err := f.cli.Status().Update(context.Background(), p); err != nil {
					t.Fatal(err)
				}
			} else if err := f.cli.Update(context.Background(), p); err != nil {
				t.Fatal(err)
			}
			p = f.round(t)
			p = f.round(t)
			a, err := readDrainAttempt(p)
			if err != nil || a == nil {
				t.Fatalf("bad completion lost proof: %#v %v", a, err)
			}
			if fault != "unhealthy" && a.Phase != drainPhaseRecovery {
				t.Fatalf("bad completion not isolated: %#v", a)
			}
			if !controllerutil.ContainsFinalizer(p, common.CNDrainingFinalizer) {
				t.Fatal("invalid upgrade lost deletion protection")
			}
			if condition := common.GetReadinessCondition(p, common.CNStoreReadiness); condition == nil || condition.Status != corev1.ConditionFalse {
				t.Fatal("invalid upgrade restored business readiness")
			}
			if len(f.lock.calls) != 2 {
				t.Fatalf("restarted handshake: %v", f.lock.calls)
			}
		})
	}
}
