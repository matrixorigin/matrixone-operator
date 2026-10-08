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
	"errors"
	"testing"
	"time"

	"github.com/matrixorigin/matrixone-operator/pkg/controllers/common"
	"github.com/openkruise/kruise-api/apps/pub"
	kruise "github.com/openkruise/kruise-api/apps/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func TestObserveUpgradeCompletionFaultMatrix(t *testing.T) {
	for _, fault := range []string{"malformed-record", "old-timestamp", "wrong-record-revision", "wrong-old-image", "next-batch", "pending-precheck", "extra-restart", "old-start", "main-exited", "missing-image", "not-update-ready", "updating", "target-changed", "owner-unobserved", "owner-missing"} {
		t.Run(fault, func(t *testing.T) {
			f := completedUpgradeFixture(t)
			p := f.read(t)
			var state pub.InPlaceUpdateState
			if err := json.Unmarshal([]byte(p.Annotations[pub.InPlaceUpdateStateKey]), &state); err != nil {
				t.Fatal(err)
			}
			waiting := false
			switch fault {
			case "old-timestamp":
				state.UpdateTimestamp = metav1.NewTime(time.Unix(1, 0))
			case "wrong-record-revision":
				state.Revision = "unrelated"
			case "wrong-old-image":
				state.LastContainerStatuses = nil
			case "next-batch":
				state.NextContainerImages = map[string]string{"main": "new"}
			case "pending-precheck":
				state.PreCheckBeforeNext = &pub.InPlaceUpdatePreCheckBeforeNext{}
			case "extra-restart":
				p.Status.ContainerStatuses[0].RestartCount++
			case "old-start":
				p.Status.ContainerStatuses[0].State.Running.StartedAt = metav1.NewTime(time.Unix(29, 0))
			case "main-exited":
				p.Status.ContainerStatuses[0].State.Running = nil
			case "missing-image":
				p.Status.ContainerStatuses[0].ImageID = ""
				waiting = true
			case "not-update-ready":
				p.Status.Conditions[len(p.Status.Conditions)-1].Status = corev1.ConditionFalse
				waiting = true
			case "updating":
				p.Labels[pub.LifecycleStateKey] = string(pub.LifecycleStateUpdating)
				waiting = true
			case "target-changed", "owner-unobserved", "owner-missing":
				cs := &kruise.CloneSet{}
				if err := f.cli.Get(context.Background(), client.ObjectKey{Namespace: p.Namespace, Name: "cloneset"}, cs); err != nil {
					t.Fatal(err)
				}
				if fault == "owner-missing" {
					if err := f.cli.Delete(context.Background(), cs); err != nil {
						t.Fatal(err)
					}
				} else {
					if fault == "target-changed" {
						cs.Status.UpdateRevision = "other"
					} else {
						cs.Generation++
						waiting = true
					}
					if err := f.cli.Update(context.Background(), cs); err != nil {
						t.Fatal(err)
					}
				}
			}
			raw, err := json.Marshal(state)
			if err != nil {
				t.Fatal(err)
			}
			p.Annotations[pub.InPlaceUpdateStateKey] = string(raw)
			if fault == "malformed-record" {
				p.Annotations[pub.InPlaceUpdateStateKey] = "{"
			}
			status := p.Status.DeepCopy()
			if err := f.cli.Update(context.Background(), p); err != nil {
				t.Fatal(err)
			}
			p.Status = *status
			if err := f.cli.Status().Update(context.Background(), p); err != nil {
				t.Fatal(err)
			}
			for i := 0; i < 3; i++ {
				p = f.round(t)
			}
			a, err := readDrainAttempt(p)
			want := drainPhaseRecovery
			if waiting {
				want = drainPhaseCompleted
			}
			if err != nil || a == nil || a.Phase != want {
				t.Fatalf("phase: %#v %v; want %s", a, err, want)
			}
			if !controllerutil.ContainsFinalizer(p, common.CNDrainingFinalizer) {
				t.Fatal("lost protection")
			}
			if cond := common.GetReadinessCondition(p, common.CNStoreReadiness); cond == nil || cond.Status != corev1.ConditionFalse {
				t.Fatal("restored business admission")
			}
			if len(f.lock.calls) != 2 {
				t.Fatalf("repeated RPC: %v", f.lock.calls)
			}
		})
	}
}

func TestObserveUpgradeControllerRestartAndHealthyRetry(t *testing.T) {
	f := completedUpgradeFixture(t)
	p := f.read(t)
	p.Status.ContainerStatuses[0].Ready = false
	if err := f.cli.Status().Update(context.Background(), p); err != nil {
		t.Fatal(err)
	}
	f.round(t)
	f.round(t)
	// Reconstruct the controller with only persistent/external dependencies.
	f.c = &Controller{apiReader: f.cli, queryCli: f.c.queryCli, clientMgr: f.c.clientMgr, lockClient: f.lock, now: f.c.now, lockIdentity: f.c.lockIdentity}
	p = f.read(t)
	p.Status.ContainerStatuses[0].Ready = true
	if err := f.cli.Status().Update(context.Background(), p); err != nil {
		t.Fatal(err)
	}
	p = f.round(t)
	if a, err := readDrainAttempt(p); err != nil || a != nil {
		t.Fatalf("retry did not finish: %#v %v", a, err)
	}
	if !controllerutil.ContainsFinalizer(p, common.CNDrainingFinalizer) || len(f.lock.calls) != 2 {
		t.Fatal("retry lost protection or repeated handshake")
	}
}

func TestObserveUpgradeFailedHealthRefreshCannotReuseOldScore(t *testing.T) {
	f := completedUpgradeFixture(t)
	f.round(t) // Establish protection and an initially successful health round.
	query := f.c.queryCli.(*fakeQueryClient)
	query.sessionErr = errors.New("health query unavailable")
	p := f.round(t)
	a, err := readDrainAttempt(p)
	if err != nil || a == nil || a.Phase != drainPhaseCompleted {
		t.Fatalf("failed health refresh cleared proof: %#v %v", a, err)
	}
	score, err := common.GetStoreScore(p)
	if err != nil || score.SessionObserved {
		t.Fatalf("old successful observation was reused: %#v %v", score, err)
	}
	if cond := common.GetReadinessCondition(p, common.CNStoreReadiness); cond == nil || cond.Status != corev1.ConditionFalse {
		t.Fatal("failed refresh admitted business")
	}
	query.sessionErr = nil
	p = f.round(t)
	if a, err := readDrainAttempt(p); err != nil || a != nil {
		t.Fatalf("successful refresh did not recover: %#v %v", a, err)
	}
	if len(f.lock.calls) != 2 {
		t.Fatal("health retry repeated lock handshake")
	}
}
