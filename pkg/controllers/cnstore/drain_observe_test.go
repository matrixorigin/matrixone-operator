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

	"github.com/go-logr/logr"
	reconfake "github.com/matrixorigin/controller-runtime/pkg/fake"
	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	"github.com/matrixorigin/matrixone-operator/pkg/controllers/common"
	"github.com/matrixorigin/matrixone-operator/pkg/mocli"
	"github.com/matrixorigin/matrixone/pkg/logservice"
	logpb "github.com/matrixorigin/matrixone/pkg/pb/logservice"
	"github.com/openkruise/kruise-api/apps/pub"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type drainHAKeeper struct {
	logservice.ProxyHAKeeperClient
	uuid string
}

type observeFixture struct {
	cli  client.Client
	pod  *corev1.Pod
	c    *Controller
	lock *fakeLockMigrationClient
}

func newObserveFixture(t *testing.T) *observeFixture {
	t.Helper()
	pod := cnStoreTestPod(pub.LifecycleStatePreparingDelete, false)
	pod.Labels[common.ComponentLabelKey], pod.Labels[common.InstanceLabelKey] = "CNSet", "cnset"
	// v4 must use the instance-bound protocol; the legacy version gate is not proof.
	pod.Annotations[common.SemanticVersionAnno] = "4.2.0"
	delete(pod.Annotations, v1alpha1.StoreDrainingStartAnno)
	ls := &v1alpha1.LogSet{ObjectMeta: metav1.ObjectMeta{Name: "log", Namespace: "ns"}}
	ls.Status.Conditions = []metav1.Condition{{Type: "Ready", Status: metav1.ConditionTrue}}
	enabled, delay := true, int32(0)
	cn := &v1alpha1.CNSet{ObjectMeta: metav1.ObjectMeta{Name: "cnset", Namespace: "ns"}}
	cn.Deps.LogSet = ls
	cn.Spec.ScalingConfig.StoreDrainEnabled, cn.Spec.ScalingConfig.MinDelaySeconds = &enabled, &delay
	cli := cnStoreTestClient(t, pod, cn, ls)
	h := &drainHAKeeper{uuid: v1alpha1.GetCNPodUUID(pod)}
	cache := mocli.NewCNCache(h, time.Hour, logr.Discard())
	t.Cleanup(cache.Close)
	lock := &fakeLockMigrationClient{setOK: true, canOK: true}
	c := &Controller{apiReader: cli, queryCli: &fakeQueryClient{},
		clientMgr:  &drainClientProvider{set: &mocli.ClientSet{Client: h, StoreCache: cache}},
		lockClient: lock, now: func() time.Time { return time.Unix(20, 0) },
		lockIdentity: func(_ context.Context, _ *corev1.Pod, uid string, _ *mocli.ClientSet) (string, error) {
			return "instance-" + uid, nil
		}}
	return &observeFixture{cli: cli, pod: pod, c: c, lock: lock}
}

func (f *observeFixture) read(t *testing.T) *corev1.Pod {
	t.Helper()
	p := &corev1.Pod{}
	if err := f.cli.Get(context.Background(), client.ObjectKeyFromObject(f.pod), p); err != nil {
		t.Fatal(err)
	}
	return p
}

func (f *observeFixture) round(t *testing.T) *corev1.Pod {
	t.Helper()
	_, _ = f.c.Observe(reconfake.NewContext(f.read(t), f.cli, nil))
	return f.read(t)
}

func TestObserveDrainBlockedMatrix(t *testing.T) {
	for _, fault := range []string{"rejected", "response-lost", "remote-txn", "query-error", "missing-service", "timeout", "disabled", "malformed", "sidecar-only"} {
		t.Run(fault, func(t *testing.T) {
			f := newObserveFixture(t)
			f.round(t)
			f.round(t)
			switch fault {
			case "rejected":
				f.lock.setOK = false
			case "response-lost":
				f.lock.setErr = fmt.Errorf("response lost")
			case "remote-txn":
				f.lock.canOK = false
			case "query-error", "missing-service":
				f.lock.canErr = fmt.Errorf("does not exist")
			case "timeout":
				f.c.now = func() time.Time { return time.Unix(20, 0).Add(24 * time.Hour) }
				f.lock.canOK = false
			case "malformed", "sidecar-only":
				p := f.read(t)
				if fault == "malformed" {
					p.Annotations[drainAttemptAnno] = "not-json"
				}
				if fault == "sidecar-only" {
					p.Status.ContainerStatuses[0].Name = "sidecar"
					if err := f.cli.Status().Update(context.Background(), p); err != nil {
						t.Fatal(err)
					}
				} else if err := f.cli.Update(context.Background(), p); err != nil {
					t.Fatal(err)
				}
			case "disabled":
				cn := &v1alpha1.CNSet{}
				if err := f.cli.Get(context.Background(), client.ObjectKey{Name: "cnset", Namespace: "ns"}, cn); err != nil {
					t.Fatal(err)
				}
				disabled := false
				cn.Spec.ScalingConfig.StoreDrainEnabled = &disabled
				if err := f.cli.Update(context.Background(), cn); err != nil {
					t.Fatal(err)
				}
			}
			for round := 0; round < 3; round++ {
				p := f.round(t)
				if len(p.Finalizers) != 1 || p.Finalizers[0] != common.CNDrainingFinalizer || !p.DeletionTimestamp.IsZero() {
					t.Fatalf("fault %s released protection", fault)
				}
				if a, _ := readDrainAttempt(p); a != nil && a.Phase == drainPhaseCompleted {
					t.Fatal("fault authorized completion")
				}
			}
			if (fault == "disabled" || fault == "malformed" || fault == "sidecar-only") && len(f.lock.calls) != 0 {
				t.Fatalf("blocked admission called RPC: %v", f.lock.calls)
			}
		})
	}
}

func TestObserveDrainAcceptedResponseLostThenCompleted(t *testing.T) {
	f := newObserveFixture(t)
	f.round(t)
	f.round(t)
	f.lock.lostResponseOnce = true
	requesting := f.round(t)
	first, err := readDrainAttempt(requesting)
	if err != nil || first == nil || first.Phase != drainPhaseRequesting ||
		len(requesting.Finalizers) != 1 || len(f.lock.calls) != 1 {
		t.Fatalf("lost response released or lost the request: %#v %v, calls=%v", first, err, f.lock.calls)
	}
	requested := f.round(t)
	second, err := readDrainAttempt(requested)
	if err != nil || second == nil || second.Phase != drainPhaseRequested ||
		second.AttemptID != first.AttemptID || len(requested.Finalizers) != 1 {
		t.Fatalf("retry did not persist the same accepted attempt: %#v %v", second, err)
	}
	completed := f.round(t)
	third, err := readDrainAttempt(completed)
	if err != nil || third == nil || third.Phase != drainPhaseCompleted || len(completed.Finalizers) != 0 {
		t.Fatalf("query did not authorize the current attempt: %#v %v", third, err)
	}
	if len(f.lock.calls) != 3 ||
		f.lock.calls[0] != "set" || f.lock.calls[1] != "set" || f.lock.calls[2] != "can" {
		t.Fatalf("unexpected release or RPC sequence: finalizers=%v calls=%v", completed.Finalizers, f.lock.calls)
	}
}

func TestObserveDrainCompletesAfterDeadlineWithoutNewAttempt(t *testing.T) {
	f := newObserveFixture(t)
	f.round(t)           // persist original start time
	f.round(t)           // prepare the attempt
	f.lock.canOK = false // a remote transaction still holds a lock
	started := f.round(t)
	before, err := readDrainAttempt(started)
	if err != nil || before == nil || before.Phase != drainPhaseRequested {
		t.Fatalf("drain request was not persisted: %#v %v", before, err)
	}
	f.c.now = func() time.Time { return time.Unix(20, 0).Add(24 * time.Hour) }
	blocked := f.round(t)
	still, err := readDrainAttempt(blocked)
	if err != nil || still == nil || still.Phase != drainPhaseRequested ||
		still.AttemptID != before.AttemptID || len(blocked.Finalizers) != 1 {
		t.Fatalf("overdue drain lost its attempt or protection: %#v %v", still, err)
	}
	if len(f.lock.calls) != 3 || f.lock.calls[0] != "set" || f.lock.calls[1] != "can" || f.lock.calls[2] != "remain" {
		t.Fatalf("overdue drain stopped observing completion: %v", f.lock.calls)
	}

	f.lock.canOK = true // the held transaction commits after the deadline
	completed := f.round(t)
	after, err := readDrainAttempt(completed)
	if err != nil || after == nil || after.Phase != drainPhaseCompleted ||
		after.AttemptID != before.AttemptID || len(completed.Finalizers) != 0 {
		t.Fatalf("same overdue attempt did not complete safely: %#v %v", after, err)
	}
	if len(f.lock.calls) != 4 || f.lock.calls[3] != "can" {
		t.Fatalf("completion restarted drain instead of querying it: %v", f.lock.calls)
	}
}

func TestObserveInstanceBoundDrainProof(t *testing.T) {
	for _, fault := range []string{"identity-missing", "identity-changed", "proof-mismatch", "allocator-missing", "query-instance-changed"} {
		t.Run(fault, func(t *testing.T) {
			f := newObserveFixture(t)
			f.round(t)
			f.round(t)
			switch fault {
			case "identity-missing":
				f.c.lockIdentity = func(context.Context, *corev1.Pod, string, *mocli.ClientSet) (string, error) {
					return "", fmt.Errorf("identity unavailable")
				}
			case "identity-changed":
				f.lock.setErr = fmt.Errorf("response lost")
				f.round(t)
				f.c.lockIdentity = func(context.Context, *corev1.Pod, string, *mocli.ClientSet) (string, error) {
					return "other-instance", nil
				}
			case "proof-mismatch":
				f.lock.proofMutate = func(p *mocli.DrainProof) { p.ServiceID = "other-instance" }
			case "allocator-missing":
				f.lock.proofMutate = func(p *mocli.DrainProof) { p.AllocatorID = "" }
			}
			p := f.round(t)
			if fault == "proof-mismatch" || fault == "allocator-missing" {
				for round := 0; round < 2; round++ {
					p = f.round(t)
				}
				a, err := readDrainAttempt(p)
				if err != nil || a == nil || a.Phase != drainPhaseRequesting {
					t.Fatalf("%s must retain Requesting without an accepted proof: %#v %v", fault, a, err)
				}
				for _, call := range f.lock.calls {
					if call != "set" {
						t.Fatalf("%s queried completion before proof validation: %v", fault, f.lock.calls)
					}
				}
			}
			if fault == "query-instance-changed" {
				a, err := readDrainAttempt(p)
				if err != nil || a == nil || a.Phase != drainPhaseRequested {
					t.Fatalf("request not persisted: %#v %v", a, err)
				}
				f.c.lockIdentity = func(context.Context, *corev1.Pod, string, *mocli.ClientSet) (string, error) {
					return "replacement-instance", nil
				}
				p = f.round(t)
			}
			if len(p.Finalizers) != 1 || !p.DeletionTimestamp.IsZero() {
				t.Fatalf("%s released protected CN", fault)
			}
			a, _ := readDrainAttempt(p)
			if a != nil && a.Phase == drainPhaseCompleted {
				t.Fatalf("%s authorized stale instance", fault)
			}
		})
	}
}

func TestObserveRejectsStaleCompletionResponse(t *testing.T) {
	for _, change := range []string{"container", "pod", "cnUUID", "lifecycle", "attempt", "cancel"} {
		t.Run(change, func(t *testing.T) {
			f := newObserveFixture(t)
			f.round(t)
			f.round(t)
			f.round(t)
			f.lock.beforeCan = func() {
				p := f.read(t)
				switch change {
				case "container":
					p.Status.ContainerStatuses[0].ContainerID = "containerd://new"
					if err := f.cli.Status().Update(context.Background(), p); err != nil {
						t.Fatal(err)
					}
					return
				case "pod":
					p.UID = "replacement"
				case "cnUUID":
					p.Spec.Subdomain = "new-cnset"
				case "lifecycle":
					p.Labels[pub.LifecycleStateKey] = string(pub.LifecycleStatePreparingUpdate)
				case "attempt":
					delete(p.Annotations, drainAttemptAnno)
				case "cancel":
					a, _ := readDrainAttempt(p)
					a.Phase = drainPhaseRecovery
					p.Annotations[drainAttemptAnno], _ = marshalDrainAttempt(a)
				}
				if err := f.cli.Update(context.Background(), p); err != nil {
					t.Fatal(err)
				}
			}
			p := f.round(t)
			if len(p.Finalizers) != 1 || !p.DeletionTimestamp.IsZero() {
				t.Fatal("stale completion released protection")
			}
			if a, _ := readDrainAttempt(p); a != nil && a.Phase == drainPhaseCompleted {
				t.Fatal("stale completion persisted authorization")
			}
		})
	}
}

func TestObservePhasePersistenceFailure(t *testing.T) {
	for _, phase := range []drainPhase{drainPhaseRequesting, drainPhaseRequested, drainPhaseCompleted} {
		t.Run(string(phase), func(t *testing.T) {
			f := newObserveFixture(t)
			f.round(t)
			f.round(t)
			fault := &drainFaultClient{Client: f.cli, failPhase: phase}
			f.cli, f.c.apiReader = fault, fault
			f.round(t)
			if phase == drainPhaseCompleted {
				f.round(t)
			}
			p := f.read(t)
			if len(p.Finalizers) != 1 || !p.DeletionTimestamp.IsZero() {
				t.Fatal("failed persistence released protection")
			}
			if phase == drainPhaseRequesting && len(f.lock.calls) != 0 {
				t.Fatalf("RPC before intent: %v", f.lock.calls)
			}
			if phase == drainPhaseRequested && (len(f.lock.calls) != 1 || f.lock.calls[0] != "set") {
				t.Fatalf("query after failed request record: %v", f.lock.calls)
			}
			fault.failPhase = ""
			// New reconciles recover from the persisted phase, not a cached result.
			for i := 0; i < 3; i++ {
				f.round(t)
			}
			p = f.read(t)
			a, err := readDrainAttempt(p)
			if err != nil || a == nil || a.Phase != drainPhaseCompleted || len(p.Finalizers) != 0 {
				t.Fatalf("recovery failed: %#v %v", a, err)
			}
		})
	}
}

func (h *drainHAKeeper) GetClusterDetails(context.Context) (logpb.ClusterDetails, error) {
	return logpb.ClusterDetails{CNStores: []logpb.CNStore{{UUID: h.uuid}}}, nil
}
func (h *drainHAKeeper) PatchCNStore(context.Context, logpb.CNStateLabel) error   { return nil }
func (h *drainHAKeeper) DeleteCNStore(context.Context, logpb.DeleteCNStore) error { return nil }

type drainClientProvider struct{ set *mocli.ClientSet }

func (p *drainClientProvider) GetClient(*v1alpha1.LogSet) (*mocli.ClientSet, error) {
	return p.set, nil
}

func TestObserveFullDrainHandshake(t *testing.T) {
	for _, lifecycle := range []pub.LifecycleStateType{pub.LifecycleStatePreparingDelete, pub.LifecycleStatePreparingUpdate} {
		t.Run(string(lifecycle), func(t *testing.T) {
			pod := cnStoreTestPod(lifecycle, false)
			pod.Labels[common.ComponentLabelKey], pod.Labels[common.InstanceLabelKey] = "CNSet", "cnset"
			pod.Annotations[common.SemanticVersionAnno] = "2.0.1"
			delete(pod.Annotations, v1alpha1.StoreDrainingStartAnno)
			ls := &v1alpha1.LogSet{ObjectMeta: metav1.ObjectMeta{Name: "log", Namespace: "ns"}}
			ls.Status.Conditions = []metav1.Condition{{Type: "Ready", Status: metav1.ConditionTrue}}
			enabled, delay := true, int32(0)
			cn := &v1alpha1.CNSet{ObjectMeta: metav1.ObjectMeta{Name: "cnset", Namespace: "ns"}}
			cn.Deps.LogSet = ls
			cn.Spec.ScalingConfig.StoreDrainEnabled, cn.Spec.ScalingConfig.MinDelaySeconds = &enabled, &delay
			cs := bindTestCloneSet(pod)
			cli := cnStoreTestClient(t, pod, cn, ls, cs)
			h := &drainHAKeeper{uuid: v1alpha1.GetCNPodUUID(pod)}
			cache := mocli.NewCNCache(h, time.Hour, logr.Discard())
			defer cache.Close()
			lock := &fakeLockMigrationClient{setOK: true, canOK: true}
			lock.beforeSet = func() {
				persisted := &corev1.Pod{}
				if err := cli.Get(context.Background(), client.ObjectKeyFromObject(pod), persisted); err != nil {
					t.Fatal(err)
				}
				a, err := readDrainAttempt(persisted)
				if err != nil || a == nil || a.Phase != drainPhaseRequesting {
					t.Fatalf("RPC before durable Requesting: %#v %v", a, err)
				}
			}
			c := &Controller{apiReader: cli, queryCli: &fakeQueryClient{},
				clientMgr:  &drainClientProvider{set: &mocli.ClientSet{Client: h, StoreCache: cache}},
				lockClient: lock, now: func() time.Time { return time.Unix(20, 0) },
				lockIdentity: func(_ context.Context, _ *corev1.Pod, uid string, _ *mocli.ClientSet) (string, error) {
					return "instance-" + uid, nil
				}}
			for round, phase := range []drainPhase{"", drainPhasePrepared, drainPhaseRequested, drainPhaseCompleted, drainPhaseCompleted} {
				fresh := &corev1.Pod{}
				if err := cli.Get(context.Background(), client.ObjectKeyFromObject(pod), fresh); err != nil {
					t.Fatal(err)
				}
				_, _ = c.Observe(reconfake.NewContext(fresh, cli, nil))
				if err := cli.Get(context.Background(), client.ObjectKeyFromObject(pod), fresh); err != nil {
					t.Fatal(err)
				}
				a, err := readDrainAttempt(fresh)
				if err != nil {
					t.Fatal(err)
				}
				if phase == "" && a != nil || phase != "" && (a == nil || a.Phase != phase) {
					t.Fatalf("round %d: want phase %s, got %#v", round, phase, a)
				}
				protected := false
				for _, f := range fresh.Finalizers {
					protected = protected || f == common.CNDrainingFinalizer
				}
				if protected != (phase != drainPhaseCompleted) {
					t.Fatalf("round %d: protection=%v", round, protected)
				}
			}
			if len(lock.calls) != 2 || lock.calls[0] != "set" || lock.calls[1] != "can" {
				t.Fatalf("RPC order: %v", lock.calls)
			}
		})
	}
}
