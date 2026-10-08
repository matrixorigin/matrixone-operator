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
	"strings"
	"testing"
	"time"

	reconfake "github.com/matrixorigin/controller-runtime/pkg/fake"
	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	"github.com/matrixorigin/matrixone-operator/pkg/controllers/common"
	"github.com/matrixorigin/matrixone-operator/pkg/mocli"
	"github.com/openkruise/kruise-api/apps/pub"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func authorizeTestPod(t *testing.T, pod *corev1.Pod) {
	t.Helper()
	lifecycle, _ := lifecycleForPod(pod)
	a, err := newDrainAttempt(pod, v1alpha1.GetCNPodUUID(pod), time.Unix(10, 0), lifecycle)
	if err != nil {
		t.Fatal(err)
	}
	a.Phase, a.RestartRequested = drainPhaseCompleted, true
	a.LockServiceID, a.AllocatorID, a.AllocatorVersion = "instance-"+a.CNUUID, "allocator", 1
	payload, err := json.Marshal(a)
	if err != nil {
		t.Fatal(err)
	}
	pod.Annotations[drainAttemptAnno] = string(payload)
}

type drainFaultClient struct {
	client.Client
	patchCount int
	failPatch  int
	failDelete bool
	failPhase  drainPhase
}

func (c *drainFaultClient) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	if pod, ok := obj.(*corev1.Pod); ok && c.failPhase != "" {
		if a, _ := readDrainAttempt(pod); a != nil && a.Phase == c.failPhase {
			return fmt.Errorf("injected phase write failure")
		}
	}
	c.patchCount++
	if c.patchCount == c.failPatch {
		return fmt.Errorf("injected write failure")
	}
	return c.Client.Patch(ctx, obj, patch, opts...)
}
func (c *drainFaultClient) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	if c.failDelete {
		return fmt.Errorf("injected delete failure")
	}
	return c.Client.Delete(ctx, obj, opts...)
}

func TestDrainRequestPersistenceFailures(t *testing.T) {
	for _, failAt := range []int{1, 2} {
		t.Run(fmt.Sprint(failAt), func(t *testing.T) {
			pod := cnStoreTestPod(pub.LifecycleStatePreparingDelete, false)
			a, err := newDrainAttempt(pod, v1alpha1.GetCNPodUUID(pod), time.Unix(10, 0), drainLifecycleDelete)
			if err != nil {
				t.Fatal(err)
			}
			pod.Annotations[drainAttemptAnno], err = marshalDrainAttempt(a)
			if err != nil {
				t.Fatal(err)
			}
			cli := &drainFaultClient{Client: cnStoreTestClient(t, pod), failPatch: failAt}
			lock := &fakeLockMigrationClient{setOK: true, canOK: true}
			wc := &withCNSet{Controller: &Controller{apiReader: cli, lockClient: lock, lockIdentity: fakeLockIdentity}}
			safe, err := wc.handleLockMigration(reconfake.NewContext(pod.DeepCopy(), cli, nil), a.CNUUID, context.Background(), &mocli.ClientSet{}, a)
			if err == nil || safe {
				t.Fatalf("write failure authorized completion: %v %v", safe, err)
			}
			if len(lock.calls) != failAt-1 {
				t.Fatalf("RPC occurred before durable intent: %v", lock.calls)
			}
			fresh := &corev1.Pod{}
			if err := cli.Get(context.Background(), client.ObjectKeyFromObject(pod), fresh); err != nil {
				t.Fatal(err)
			}
			stored, err := readDrainAttempt(fresh)
			if err != nil {
				t.Fatal(err)
			}
			want := drainPhasePrepared
			if failAt == 2 {
				want = drainPhaseRequesting
			}
			if stored.Phase != want {
				t.Fatalf("phase=%s want=%s", stored.Phase, want)
			}
		})
	}
}

func TestDirectDeleteFailureRetainsAuthorization(t *testing.T) {
	pod := cnStoreTestPod(pub.LifecycleStatePreparingDelete, true)
	authorizeTestPod(t, pod)
	cli := &drainFaultClient{Client: cnStoreTestClient(t, pod), failDelete: true}
	wc := &withCNSet{Controller: &Controller{apiReader: cli}}
	if err := wc.completeDraining(reconfake.NewContext(pod.DeepCopy(), cli, nil), pod.DeepCopy()); err == nil {
		t.Fatal("expected delete error")
	}
	fresh := &corev1.Pod{}
	if err := cli.Get(context.Background(), client.ObjectKeyFromObject(pod), fresh); err != nil {
		t.Fatal(err)
	}
	if fresh.Annotations[drainAttemptAnno] != pod.Annotations[drainAttemptAnno] || len(fresh.Finalizers) != 1 || fresh.Finalizers[0] != common.CNDrainingFinalizer {
		t.Fatal("failed delete destroyed recovery state")
	}
	cli.failDelete = false
	// Reconstruct the controller to model restart, without any in-memory proof.
	wc = &withCNSet{Controller: &Controller{apiReader: cli}}
	if err := wc.completeDraining(reconfake.NewContext(fresh, cli, nil), fresh.DeepCopy()); err != nil {
		t.Fatal(err)
	}
}

func TestCompletionSnapshotRejectsChangedAttempt(t *testing.T) {
	for _, change := range []string{"cnUUID", "attempt", "phase"} {
		t.Run(change, func(t *testing.T) {
			pod := cnStoreTestPod(pub.LifecycleStatePreparingDelete, false)
			a, _ := newDrainAttempt(pod, v1alpha1.GetCNPodUUID(pod), time.Unix(10, 0), drainLifecycleDelete)
			a.Phase, a.RestartRequested = drainPhaseRequested, true
			a.LockServiceID, a.AllocatorID, a.AllocatorVersion = "instance-"+a.CNUUID, "allocator", 1
			unchanged := pod.DeepCopy()
			unchanged.Annotations[drainAttemptAnno], _ = marshalDrainAttempt(a)
			controlClient := cnStoreTestClient(t, unchanged)
			control := &withCNSet{Controller: &Controller{apiReader: controlClient}}
			if _, err := control.verifyDrainAttempt(reconfake.NewContext(unchanged, controlClient, nil), a); err != nil {
				t.Fatalf("unchanged requested attempt must be valid: %v", err)
			}
			stored := *a
			switch change {
			case "cnUUID":
				pod.Spec.Subdomain = "replacement"
			case "attempt":
				stored.DrainStartedAt = time.Unix(11, 0).UTC().Format(time.RFC3339Nano)
				stored.AttemptID = drainAttemptID(stored.PodUID, stored.CNUUID, stored.ContainerID, stored.ContainerStartedAt, stored.DrainStartedAt, stored.Lifecycle)
			case "phase":
				stored.Phase = drainPhaseRecovery
			}
			pod.Annotations[drainAttemptAnno], _ = marshalDrainAttempt(&stored)
			if _, err := readDrainAttempt(pod); err != nil {
				t.Fatalf("changed fixture must remain a valid drain attempt: %v", err)
			}
			cli := cnStoreTestClient(t, pod)
			wc := &withCNSet{Controller: &Controller{apiReader: cli}}
			want := "request snapshot changed"
			if change == "cnUUID" {
				want = "actual instance or lifecycle changed"
			}
			if _, err := wc.verifyDrainAttempt(reconfake.NewContext(pod, cli, nil), a); err == nil || !strings.Contains(err.Error(), want) {
				t.Fatalf("stale query did not fail the %q guard: %v", want, err)
			}
		})
	}
}

func TestPersistDrainPhaseRejectsStaleSource(t *testing.T) {
	for _, change := range []string{"container", "pod", "phase", "cnUUID"} {
		t.Run(change, func(t *testing.T) {
			pod := cnStoreTestPod(pub.LifecycleStatePreparingDelete, false)
			expected, err := newDrainAttempt(pod, v1alpha1.GetCNPodUUID(pod), time.Unix(10, 0), drainLifecycleDelete)
			if err != nil {
				t.Fatal(err)
			}
			expected.Phase = drainPhaseRequesting
			expected.LockServiceID = "instance-" + expected.CNUUID
			proof := mocli.DrainProof{ServiceID: expected.LockServiceID, AttemptID: expected.AttemptID, AllocatorID: "allocator", AllocatorVersion: 1}
			unchanged := pod.DeepCopy()
			unchanged.Annotations[drainAttemptAnno], err = marshalDrainAttempt(expected)
			if err != nil {
				t.Fatal(err)
			}
			controlClient := cnStoreTestClient(t, unchanged)
			control := &withCNSet{Controller: &Controller{apiReader: controlClient}}
			if err := control.persistDrainTransition(reconfake.NewContext(unchanged, controlClient, nil), expected, drainPhaseRequested, "", proof); err != nil {
				t.Fatalf("unchanged Requesting attempt must advance: %v", err)
			}
			advanced := &corev1.Pod{}
			if err := controlClient.Get(context.Background(), client.ObjectKeyFromObject(unchanged), advanced); err != nil {
				t.Fatal(err)
			}
			if got, err := readDrainAttempt(advanced); err != nil || got == nil || got.Phase != drainPhaseRequested {
				t.Fatalf("unchanged request did not persist Requested: %#v %v", got, err)
			}
			stored := *expected
			switch change {
			case "container":
				pod.Status.ContainerStatuses[0].ContainerID = "containerd://replacement"
			case "pod":
				pod.UID = "replacement"
			case "phase":
				stored.Phase = drainPhasePrepared
				stored.LockServiceID = ""
			case "cnUUID":
				pod.Spec.Subdomain = "replacement"
			}
			payload, err := json.Marshal(stored)
			if err != nil {
				t.Fatal(err)
			}
			pod.Annotations[drainAttemptAnno] = string(payload)
			if _, err := readDrainAttempt(pod); err != nil {
				t.Fatalf("changed fixture must remain a valid drain attempt: %v", err)
			}
			cli := cnStoreTestClient(t, pod)
			wc := &withCNSet{Controller: &Controller{apiReader: cli}}
			ctx := reconfake.NewContext(pod.DeepCopy(), cli, nil)
			want := "actual instance or lifecycle changed"
			if change == "phase" {
				want = "request snapshot changed"
			}
			if err := wc.persistDrainTransition(ctx, expected, drainPhaseRequested, "", proof); err == nil || !strings.Contains(err.Error(), want) {
				t.Fatalf("stale request response did not fail the %q guard: %v", want, err)
			}
			fresh := &corev1.Pod{}
			if err := cli.Get(context.Background(), client.ObjectKeyFromObject(pod), fresh); err != nil {
				t.Fatal(err)
			}
			if fresh.Annotations[drainAttemptAnno] != string(payload) {
				t.Fatal("rejected response mutated proof")
			}
		})
	}
}

func TestDrainRPCResponseCannotOverwriteCancellation(t *testing.T) {
	pod := cnStoreTestPod(pub.LifecycleStatePreparingDelete, false)
	a, err := newDrainAttempt(pod, v1alpha1.GetCNPodUUID(pod), time.Unix(10, 0), drainLifecycleDelete)
	if err != nil {
		t.Fatal(err)
	}
	pod.Annotations[drainAttemptAnno], err = marshalDrainAttempt(a)
	if err != nil {
		t.Fatal(err)
	}
	cli := cnStoreTestClient(t, pod)
	entered, release := make(chan struct{}), make(chan struct{})
	lock := &fakeLockMigrationClient{setOK: true, beforeSet: func() { close(entered); <-release }}
	wc := &withCNSet{Controller: &Controller{apiReader: cli, lockClient: lock, lockIdentity: fakeLockIdentity}}
	done := make(chan error, 1)
	go func() {
		_, err := wc.handleLockMigration(reconfake.NewContext(pod.DeepCopy(), cli, nil), a.CNUUID, context.Background(), &mocli.ClientSet{}, a)
		done <- err
	}()
	<-entered
	fresh := &corev1.Pod{}
	if err := cli.Get(context.Background(), client.ObjectKeyFromObject(pod), fresh); err != nil {
		close(release)
		t.Fatal(err)
	}
	current, err := readDrainAttempt(fresh)
	if err != nil {
		close(release)
		t.Fatal(err)
	}
	current.Phase = drainPhaseRecovery
	fresh.Annotations[drainAttemptAnno], err = marshalDrainAttempt(current)
	if err != nil {
		close(release)
		t.Fatal(err)
	}
	if err := cli.Update(context.Background(), fresh); err != nil {
		close(release)
		t.Fatal(err)
	}
	close(release)
	if err := <-done; err == nil {
		t.Fatal("late response overwrote recovery")
	}
	if err := cli.Get(context.Background(), client.ObjectKeyFromObject(pod), fresh); err != nil {
		t.Fatal(err)
	}
	current, err = readDrainAttempt(fresh)
	if err != nil || current.Phase != drainPhaseRecovery {
		t.Fatalf("recovery lost: %#v %v", current, err)
	}
}
