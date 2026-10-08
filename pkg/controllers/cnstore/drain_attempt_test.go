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
	"fmt"
	"testing"
	"time"

	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	"github.com/matrixorigin/matrixone-operator/pkg/controllers/common"
	"github.com/matrixorigin/matrixone-operator/pkg/mocli"
	"github.com/openkruise/kruise-api/apps/pub"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

type fakeLockMigrationClient struct {
	calls            []string
	setOK            bool
	canOK            bool
	setErr           error
	lostResponseOnce bool
	acceptedAttempt  string
	canErr           error
	beforeSet        func()
	beforeCan        func()
	proofMutate      func(*mocli.DrainProof)
}

func (f *fakeLockMigrationClient) BeginDrain(_ context.Context, serviceID, attemptID string) (mocli.DrainProof, error) {
	if f.beforeSet != nil {
		f.beforeSet()
	}
	f.calls = append(f.calls, "set")
	if f.lostResponseOnce {
		// The server accepted this attempt, but the client never received its
		// proof. The next reconcile must retry the same durable attempt.
		f.lostResponseOnce = false
		f.acceptedAttempt = attemptID
		return mocli.DrainProof{}, fmt.Errorf("accepted drain response lost")
	}
	if f.acceptedAttempt != "" && f.acceptedAttempt != attemptID {
		return mocli.DrainProof{}, fmt.Errorf("different attempt after lost response")
	}
	if f.setErr != nil || !f.setOK {
		if f.setErr != nil {
			return mocli.DrainProof{}, f.setErr
		}
		return mocli.DrainProof{}, fmt.Errorf("drain rejected")
	}
	proof := mocli.DrainProof{ServiceID: serviceID, AttemptID: attemptID, AllocatorID: "allocator", AllocatorVersion: 1}
	if f.proofMutate != nil {
		f.proofMutate(&proof)
	}
	return proof, nil
}

func (f *fakeLockMigrationClient) QueryDrain(_ context.Context, proof mocli.DrainProof) (bool, error) {
	if proof.ServiceID == "" || proof.AttemptID == "" || proof.AllocatorID == "" || proof.AllocatorVersion == 0 {
		return false, fmt.Errorf("incomplete drain proof")
	}
	if f.beforeCan != nil {
		f.beforeCan()
	}
	f.calls = append(f.calls, "can")
	return f.canOK, f.canErr
}

func (f *fakeLockMigrationClient) RemainTxnCount(context.Context, string) (int, error) {
	f.calls = append(f.calls, "remain")
	return 0, nil
}

func fakeLockIdentity(_ context.Context, _ *corev1.Pod, uid string, _ *mocli.ClientSet) (string, error) {
	return "instance-" + uid, nil
}

func runningPod(uid, containerID string, startedAt time.Time) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			UID:    types.UID(uid),
			Labels: map[string]string{},
			Annotations: map[string]string{
				common.SemanticVersionAnno: "2.0.1",
			},
		},
		Status: corev1.PodStatus{
			ContainerStatuses: []corev1.ContainerStatus{{
				Name:        v1alpha1.ContainerMain,
				ContainerID: containerID,
				State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{
					StartedAt: metav1.NewTime(startedAt),
				}},
			}},
		},
	}
}

func TestAdvanceLockDrainRequiresRequestBeforeCanRestart(t *testing.T) {
	fake := &fakeLockMigrationClient{setOK: true, canOK: true}
	attempt := &drainAttempt{PodUID: "pod", CNUUID: "cn", ContainerID: "container", ContainerStartedAt: "started", DrainStartedAt: "drain", AttemptID: "attempt", LockServiceID: "instance-cn", Phase: drainPhaseRequesting}

	safe, requested, err := advanceLockDrain(context.Background(), "cn", attempt, fake)
	if err != nil {
		t.Fatal(err)
	}
	if safe || !requested {
		t.Fatalf("first step = safe %v requested %v, want safe=false requested=true", safe, requested)
	}
	if got, want := fake.calls, []string{"set"}; len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("first calls = %v, want %v", got, want)
	}

	attempt.Phase = drainPhaseRequested
	attempt.RestartRequested = true
	safe, requested, err = advanceLockDrain(context.Background(), "cn", attempt, fake)
	if err != nil {
		t.Fatal(err)
	}
	if !safe || requested {
		t.Fatalf("second step = safe %v requested %v, want safe=true requested=false", safe, requested)
	}
	if got, want := fake.calls, []string{"set", "can"}; len(got) != len(want) || got[1] != want[1] {
		t.Fatalf("call order = %v, want %v", got, want)
	}
}

func TestAdvanceLockDrainRejectsMismatchedBeginProof(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*mocli.DrainProof)
	}{
		{"service", func(p *mocli.DrainProof) { p.ServiceID = "other-instance" }},
		{"attempt", func(p *mocli.DrainProof) { p.AttemptID = "other-attempt" }},
		{"allocator", func(p *mocli.DrainProof) { p.AllocatorID = "" }},
		{"epoch", func(p *mocli.DrainProof) { p.AllocatorVersion = 0 }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			attempt := &drainAttempt{AttemptID: "attempt", LockServiceID: "instance-cn", Phase: drainPhaseRequesting}
			fake := &fakeLockMigrationClient{setOK: true, canOK: true, proofMutate: tc.mutate}
			safe, requested, err := advanceLockDrain(context.Background(), "cn", attempt, fake)
			if err == nil || safe || requested || attempt.AllocatorID != "" || attempt.AllocatorVersion != 0 {
				t.Fatalf("invalid proof advanced the attempt: safe=%v requested=%v attempt=%#v err=%v", safe, requested, attempt, err)
			}
			if len(fake.calls) != 1 || fake.calls[0] != "set" {
				t.Fatalf("invalid proof queried completion: %v", fake.calls)
			}
		})
	}
}

func TestAdvanceLockDrainDoesNotTreatCanRestartAsInitialProof(t *testing.T) {
	fake := &fakeLockMigrationClient{setOK: true, canOK: true}
	attempt := &drainAttempt{PodUID: "pod", CNUUID: "cn", ContainerID: "container", ContainerStartedAt: "started", DrainStartedAt: "drain", AttemptID: "attempt", LockServiceID: "instance-cn", Phase: drainPhasePrepared}

	_, _, err := advanceLockDrain(context.Background(), "cn", attempt, fake)
	if err == nil || len(fake.calls) != 0 {
		t.Fatalf("unpersisted intent called RPC: calls=%v err=%v", fake.calls, err)
	}
}

func TestAdvanceLockDrainBlocksOnSetFailure(t *testing.T) {
	attempt := &drainAttempt{PodUID: "pod", CNUUID: "cn", ContainerID: "container", ContainerStartedAt: "started", DrainStartedAt: "drain", AttemptID: "attempt", LockServiceID: "instance-cn", Phase: drainPhaseRequesting}
	fake := &fakeLockMigrationClient{setErr: context.Canceled}
	safe, requested, err := advanceLockDrain(context.Background(), "cn", attempt, fake)
	if err == nil || safe || requested {
		t.Fatalf("set failure = safe %v requested %v err %v, want blocked", safe, requested, err)
	}
	if len(fake.calls) != 1 || fake.calls[0] != "set" {
		t.Fatalf("calls after set failure = %v, want [set]", fake.calls)
	}
}

func TestAdvanceLockDrainBlocksOnCanError(t *testing.T) {
	attempt := &drainAttempt{
		PodUID: "pod", CNUUID: "cn", ContainerID: "container",
		ContainerStartedAt: "started", DrainStartedAt: "drain", AttemptID: "attempt", LockServiceID: "instance-cn", AllocatorID: "allocator", AllocatorVersion: 1, Phase: drainPhaseRequested, RestartRequested: true,
	}
	fake := &fakeLockMigrationClient{canErr: context.DeadlineExceeded}
	safe, requested, err := advanceLockDrain(context.Background(), "cn", attempt, fake)
	if err == nil || safe || requested {
		t.Fatalf("can failure = safe %v requested %v err %v, want blocked", safe, requested, err)
	}
	if len(fake.calls) != 1 || fake.calls[0] != "can" {
		t.Fatalf("calls after can failure = %v, want [can]", fake.calls)
	}
}

func TestAdvanceLockDrainDoesNotCallRPCForRecoveryAttempt(t *testing.T) {
	attempt := &drainAttempt{
		PodUID: "pod", CNUUID: "cn", ContainerID: "container",
		ContainerStartedAt: "started", DrainStartedAt: "drain", Phase: drainPhaseRecovery,
	}
	fake := &fakeLockMigrationClient{setOK: true, canOK: true}
	safe, requested, err := advanceLockDrain(context.Background(), "cn", attempt, fake)
	if err == nil || safe || requested {
		t.Fatalf("recovery attempt = safe %v requested %v err %v, want blocked", safe, requested, err)
	}
	if len(fake.calls) != 0 {
		t.Fatalf("recovery attempt must not call lock RPCs: calls=%v", fake.calls)
	}
}

func TestDrainAttemptBindsPodAndContainerIdentity(t *testing.T) {
	started := time.Unix(10, 0).UTC()
	pod := runningPod("pod-1", "containerd://one", started)
	pod.Labels[pub.LifecycleStateKey] = string(pub.LifecycleStatePreparingDelete)
	attempt, err := newDrainAttempt(pod, "cn-1", started.Add(time.Second), drainLifecycleDelete)
	if err != nil {
		t.Fatal(err)
	}
	if !attempt.matches(attempt) {
		t.Fatal("attempt should match its own identity")
	}

	replacement := runningPod("pod-1", "containerd://two", started.Add(time.Second))
	replacement.Labels[pub.LifecycleStateKey] = string(pub.LifecycleStatePreparingDelete)
	replacementAttempt, err := newDrainAttempt(replacement, "cn-1", started.Add(time.Second), drainLifecycleDelete)
	if err != nil {
		t.Fatal(err)
	}
	if attempt.matches(replacementAttempt) {
		t.Fatal("container replacement reused the previous drain proof")
	}
}

func TestDrainAttemptRequiresRunningContainer(t *testing.T) {
	pod := runningPod("pod-1", "containerd://one", time.Unix(10, 0))
	pod.Status.ContainerStatuses[0].State = corev1.ContainerState{}
	if _, err := newDrainAttempt(pod, "cn-1", time.Unix(11, 0), drainLifecycleDelete); err == nil {
		t.Fatal("expected missing running container identity to block drain")
	}
}

func TestDrainAttemptDoesNotUseSidecarAsMainIdentity(t *testing.T) {
	pod := runningPod("pod-1", "containerd://sidecar", time.Unix(10, 0))
	pod.Status.ContainerStatuses[0].Name = "metrics"
	if _, err := newDrainAttempt(pod, "cn-1", time.Unix(11, 0), drainLifecycleDelete); err == nil {
		t.Fatal("expected sidecar-only identity to block drain")
	}
}

func TestReadDrainAttemptRejectsMalformedIdentity(t *testing.T) {
	pod := runningPod("pod-1", "containerd://one", time.Unix(10, 0))
	pod.Labels[pub.LifecycleStateKey] = string(pub.LifecycleStatePreparingDelete)
	pod.Annotations[drainAttemptAnno] = `{"podUID":"pod-1"}`
	if _, err := readDrainAttempt(pod); err == nil {
		t.Fatal("expected incomplete drain identity to be rejected")
	}

	pod.Annotations[drainAttemptAnno] = "not-json"
	if _, err := readDrainAttempt(pod); err == nil {
		t.Fatal("expected malformed drain attempt to be rejected")
	}
}

func TestDrainAttemptIDIncludesLifecycleAndProcessIdentity(t *testing.T) {
	base := drainAttemptID("pod", "cn", "container", "started", "drain", drainLifecycleDelete)
	if base == drainAttemptID("pod", "cn", "container", "started", "drain", drainLifecycleUpdate) {
		t.Fatal("delete and update attempts must not share an attempt ID")
	}
	if base == drainAttemptID("pod", "cn", "replacement", "started", "drain", drainLifecycleDelete) {
		t.Fatal("replacement containers must not share an attempt ID")
	}
}
