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
	"strings"
	"testing"
	"time"

	reconfake "github.com/matrixorigin/controller-runtime/pkg/fake"
	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	"github.com/matrixorigin/matrixone-operator/pkg/controllers/common"
	"github.com/openkruise/kruise-api/apps/pub"
	kruise "github.com/openkruise/kruise-api/apps/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	clientfake "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func cnStoreTestClient(t *testing.T, objects ...client.Object) client.Client {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := v1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := kruise.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	return clientfake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()
}

func cnStoreTestPod(lifecycle pub.LifecycleStateType, direct bool) *corev1.Pod {
	labels := map[string]string{
		pub.LifecycleStateKey: string(lifecycle),
	}
	if direct {
		labels[v1alpha1.DirectPodLabel] = "true"
	}
	started := time.Unix(10, 0).UTC()
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "cn-0",
			Namespace:       "ns",
			UID:             types.UID("pod-uid-1"),
			ResourceVersion: "1",
			Finalizers:      []string{common.CNDrainingFinalizer},
			Labels:          labels,
			Annotations: map[string]string{
				common.SemanticVersionAnno:      "4.2.0",
				v1alpha1.StoreDrainingStartAnno: started.Format(time.RFC3339),
			},
		},
		Spec: corev1.PodSpec{Subdomain: "cnset"},
		Status: corev1.PodStatus{ContainerStatuses: []corev1.ContainerStatus{{
			Name:        v1alpha1.ContainerMain,
			ContainerID: "containerd://main-1",
			ImageID:     "sha256:old",
			Ready:       true,
			State:       corev1.ContainerState{Running: &corev1.ContainerStateRunning{StartedAt: metav1.NewTime(started)}},
		}}},
	}
}

type replacingReader struct {
	client.Reader
	replacement *corev1.Pod
	getCount    int
}

func (r *replacingReader) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	r.getCount++
	if r.getCount == 1 {
		r.replacement.DeepCopyInto(obj.(*corev1.Pod))
		return nil
	}
	return r.Reader.Get(ctx, key, obj, opts...)
}

func TestEnsureDrainAttemptAllowsPreparingUpdateReentry(t *testing.T) {
	pod := cnStoreTestPod(pub.LifecycleStatePreparingUpdate, false)
	cs := bindTestCloneSet(pod)
	cli := cnStoreTestClient(t, pod, cs)
	ctx := reconfake.NewContext(pod, cli, nil)
	wc := &withCNSet{Controller: &Controller{apiReader: cli}}
	start := time.Unix(20, 0).UTC()
	uid := v1alpha1.GetCNPodUUID(pod)

	if _, err := wc.ensureDrainAttempt(ctx, uid, start, drainLifecycleUpdate); err == nil {
		t.Fatal("first reconcile should persist the attempt and wait for the next reconcile")
	}
	stored := &corev1.Pod{}
	if err := cli.Get(context.Background(), clientObjectKey(pod), stored); err != nil {
		t.Fatal(err)
	}
	first, err := readDrainAttempt(stored)
	if err != nil {
		t.Fatal(err)
	}
	if first == nil || first.Lifecycle != drainLifecycleUpdate || first.Phase != drainPhasePrepared {
		t.Fatalf("stored attempt = %#v, want update/Prepared", first)
	}

	ctx.Obj = stored
	second, err := wc.ensureDrainAttempt(ctx, uid, start, drainLifecycleUpdate)
	if err != nil {
		t.Fatal(err)
	}
	if second == nil || second.AttemptID != first.AttemptID {
		t.Fatalf("reconcile did not continue the same attempt: first=%#v second=%#v", first, second)
	}
}

func TestControllerObserveUsesPreparingStopEntryPoint(t *testing.T) {
	pod := cnStoreTestPod(pub.LifecycleStatePreparingDelete, false)
	delete(pod.Annotations, v1alpha1.StoreDrainingStartAnno)
	pod.Annotations[common.SemanticVersionAnno] = "2.0.1"
	// Make syncStats return before it needs a live MORPC manager. The actual
	// Observe path must still resolve the CNSet and persist the first drain
	// timestamp before any lock RPC is possible.
	pod.Status.ContainerStatuses[0].State = corev1.ContainerState{}
	pod.Labels[common.ComponentLabelKey] = "CNSet"
	pod.Labels[common.InstanceLabelKey] = "cnset"
	enabled := true
	cnSet := &v1alpha1.CNSet{
		ObjectMeta: metav1.ObjectMeta{Name: "cnset", Namespace: "ns"},
		Spec:       v1alpha1.CNSetSpec{ScalingConfig: v1alpha1.ScalingConfig{StoreDrainEnabled: &enabled}},
	}
	cli := cnStoreTestClient(t, pod, cnSet)
	ctx := reconfake.NewContext(pod, cli, nil)

	if _, err := (&Controller{}).Observe(ctx); err == nil {
		t.Fatal("first preparing-stop reconcile should persist its start timestamp and resync")
	}
	stored := &corev1.Pod{}
	if err := cli.Get(context.Background(), clientObjectKey(pod), stored); err != nil {
		t.Fatal(err)
	}
	if stored.Annotations[v1alpha1.StoreDrainingStartAnno] == "" {
		t.Fatal("registered Observe path did not persist the drain start timestamp")
	}
	if _, err := readDrainAttempt(stored); err != nil {
		t.Fatal(err)
	}
}

func TestEnsureDrainAttemptRejectsLifecycleSwitch(t *testing.T) {
	pod := cnStoreTestPod(pub.LifecycleStatePreparingDelete, false)
	start := time.Unix(20, 0).UTC()
	uid := v1alpha1.GetCNPodUUID(pod)
	attempt, err := newDrainAttempt(pod, uid, start, drainLifecycleDelete)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := marshalDrainAttempt(attempt)
	if err != nil {
		t.Fatal(err)
	}
	pod.Annotations[drainAttemptAnno] = payload
	pod.Labels[pub.LifecycleStateKey] = string(pub.LifecycleStatePreparingUpdate)
	cli := cnStoreTestClient(t, pod)
	ctx := reconfake.NewContext(pod, cli, nil)
	wc := &withCNSet{Controller: &Controller{apiReader: cli}}

	if _, err := wc.ensureDrainAttempt(ctx, uid, start, drainLifecycleUpdate); err == nil {
		t.Fatal("lifecycle switch must invalidate the previous attempt")
	}
	stored := &corev1.Pod{}
	if err := cli.Get(context.Background(), clientObjectKey(pod), stored); err != nil {
		t.Fatal(err)
	}
	recovered, err := readDrainAttempt(stored)
	if err != nil {
		t.Fatal(err)
	}
	if recovered == nil || recovered.Phase != drainPhaseRecovery {
		t.Fatalf("lifecycle switch state = %#v, want RecoveryRequired", recovered)
	}
}

func TestEnsureDrainAttemptPreservesLegacyLockRestartMarker(t *testing.T) {
	pod := cnStoreTestPod(pub.LifecycleStatePreparingDelete, false)
	pod.Annotations[LockRestartSet] = "true"
	cli := cnStoreTestClient(t, pod)
	ctx := reconfake.NewContext(pod, cli, nil)
	wc := &withCNSet{Controller: &Controller{apiReader: cli}}
	uid := v1alpha1.GetCNPodUUID(pod)

	if _, err := wc.ensureDrainAttempt(ctx, uid, time.Unix(20, 0).UTC(), drainLifecycleDelete); err == nil {
		t.Fatal("legacy lock-restart marker must block creation of a new proof")
	}
	stored := &corev1.Pod{}
	if err := cli.Get(context.Background(), clientObjectKey(pod), stored); err != nil {
		t.Fatal(err)
	}
	if stored.Annotations[LockRestartSet] != "true" || stored.Annotations[drainAttemptAnno] != "" {
		t.Fatalf("legacy marker was rewritten: annotations=%v", stored.Annotations)
	}
}

func TestCompleteDrainingUsesFreshUIDAndResourceVersionForDirectPod(t *testing.T) {
	pod := cnStoreTestPod(pub.LifecycleStatePreparingDelete, true)
	authorizeTestPod(t, pod)
	cli := cnStoreTestClient(t, pod)
	ctx := reconfake.NewContext(pod, cli, nil)
	wc := &withCNSet{Controller: &Controller{apiReader: cli}}

	if err := wc.completeDraining(ctx, pod.DeepCopy()); err != nil {
		t.Fatal(err)
	}
	got := &corev1.Pod{}
	if err := cli.Get(context.Background(), clientObjectKey(pod), got); err != nil {
		t.Fatal(err)
	}
	if got.DeletionTimestamp.IsZero() || len(got.Finalizers) == 0 || got.Annotations[drainAttemptAnno] == "" {
		t.Fatal("accepted DELETE must retain proof and protection until OnDeleted cleanup")
	}
}

func TestCompleteDrainingKeepsManagedPodAfterFinalizerRemoval(t *testing.T) {
	pod := cnStoreTestPod(pub.LifecycleStatePreparingDelete, false)
	authorizeTestPod(t, pod)
	cli := cnStoreTestClient(t, pod)
	ctx := reconfake.NewContext(pod, cli, nil)
	wc := &withCNSet{Controller: &Controller{apiReader: cli}}

	if err := wc.completeDraining(ctx, pod.DeepCopy()); err != nil {
		t.Fatal(err)
	}
	got := &corev1.Pod{}
	if err := cli.Get(context.Background(), clientObjectKey(pod), got); err != nil {
		t.Fatal(err)
	}
	for _, finalizer := range got.Finalizers {
		if finalizer == common.CNDrainingFinalizer {
			t.Fatal("CN draining finalizer was not removed")
		}
	}
}

func TestCompleteDrainingRejectsReplacementBeforeRemovingProtection(t *testing.T) {
	pod := cnStoreTestPod(pub.LifecycleStatePreparingDelete, true)
	cli := cnStoreTestClient(t, pod)
	replacement := pod.DeepCopy()
	replacement.UID = types.UID("replacement-uid")
	replacement.ResourceVersion = "2"
	reader := &replacingReader{Reader: cli, replacement: replacement}
	ctx := reconfake.NewContext(pod, cli, nil)
	wc := &withCNSet{Controller: &Controller{apiReader: reader}}

	if err := wc.completeDraining(ctx, pod.DeepCopy()); err == nil {
		t.Fatal("replacement Pod must stop completion before finalizer removal")
	}
	stored := &corev1.Pod{}
	if err := cli.Get(context.Background(), clientObjectKey(pod), stored); err != nil {
		t.Fatal(err)
	}
	for _, finalizer := range stored.Finalizers {
		if finalizer == common.CNDrainingFinalizer {
			return
		}
	}
	t.Fatal("replacement detection must retain the original Pod finalizer")
}

func TestVerifyDrainAttemptRejectsReplacementReadFromAPI(t *testing.T) {
	start := time.Unix(20, 0).UTC()
	pod := cnStoreTestPod(pub.LifecycleStatePreparingDelete, false)
	uid := v1alpha1.GetCNPodUUID(pod)
	attempt, err := newDrainAttempt(pod, uid, start, drainLifecycleDelete)
	if err != nil {
		t.Fatal(err)
	}
	attempt.Phase = drainPhaseRequested
	attempt.RestartRequested = true
	attempt.LockServiceID = "instance-" + uid
	attempt.AllocatorID = "allocator"
	attempt.AllocatorVersion = 1
	payload, err := marshalDrainAttempt(attempt)
	if err != nil {
		t.Fatal(err)
	}
	pod.Annotations[drainAttemptAnno] = payload
	cli := cnStoreTestClient(t, pod)
	ctx := reconfake.NewContext(pod, cli, nil)
	wc := &withCNSet{Controller: &Controller{apiReader: cli}}
	if _, err := wc.verifyDrainAttempt(ctx, attempt); err != nil {
		t.Fatalf("unchanged requested attempt must be valid: %v", err)
	}

	replacement := pod.DeepCopy()
	replacement.Status.ContainerStatuses[0].ContainerID = "containerd://main-2"
	if err := cli.Status().Update(context.Background(), replacement); err != nil {
		t.Fatal(err)
	}
	if _, err := wc.verifyDrainAttempt(ctx, attempt); err == nil || !strings.Contains(err.Error(), "actual instance or lifecycle changed") {
		t.Fatalf("replacement main container must fail the actual identity check: %v", err)
	}
}

func clientObjectKey(pod *corev1.Pod) client.ObjectKey {
	return client.ObjectKeyFromObject(pod)
}

func marshalDrainAttempt(attempt *drainAttempt) (string, error) {
	data, err := json.Marshal(attempt)
	return string(data), err
}
