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
	"os"
	"testing"
	"time"

	reconfake "github.com/matrixorigin/controller-runtime/pkg/fake"
	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	"github.com/matrixorigin/matrixone-operator/pkg/mocli"
	"github.com/openkruise/kruise-api/apps/pub"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

type afterReadClient struct {
	client.Client
	afterRead func()
}

func (c *afterReadClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	if err := c.Client.Get(ctx, key, obj, opts...); err != nil {
		return err
	}
	if c.afterRead != nil {
		fn := c.afterRead
		c.afterRead = nil
		fn()
	}
	return nil
}

func TestDrainRealAPIConcurrency(t *testing.T) {
	if os.Getenv("KUBEBUILDER_ASSETS") == "" {
		t.Skip("NOT_RUN: KUBEBUILDER_ASSETS is required")
	}
	env := &envtest.Environment{}
	cfg, err := env.Start()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := env.Stop(); err != nil {
			t.Error(err)
		}
	})
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	cli, err := client.New(cfg, client.Options{Scheme: scheme})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := cli.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "ns"}}); err != nil {
		t.Fatal(err)
	}
	pod := cnStoreTestPod(pub.LifecycleStatePreparingDelete, true)
	pod.UID, pod.ResourceVersion = "", ""
	pod.Spec.Containers = []corev1.Container{{Name: v1alpha1.ContainerMain, Image: "example.invalid/cn:test"}}
	status := pod.Status
	if err := cli.Create(ctx, pod); err != nil {
		t.Fatal(err)
	}
	pod.Status = status
	if err := cli.Status().Update(ctx, pod); err != nil {
		t.Fatal(err)
	}
	a, err := newDrainAttempt(pod, v1alpha1.GetCNPodUUID(pod), time.Unix(10, 0), drainLifecycleDelete)
	if err != nil {
		t.Fatal(err)
	}
	pod.Annotations[drainAttemptAnno], err = marshalDrainAttempt(a)
	if err != nil {
		t.Fatal(err)
	}
	if err := cli.Update(ctx, pod); err != nil {
		t.Fatal(err)
	}
	reader := &afterReadClient{Client: cli, afterRead: func() {
		concurrent := &corev1.Pod{}
		if err := cli.Get(ctx, client.ObjectKeyFromObject(pod), concurrent); err != nil {
			t.Fatal(err)
		}
		concurrent.Annotations["test/concurrent"] = "change"
		if err := cli.Update(ctx, concurrent); err != nil {
			t.Fatal(err)
		}
	}}
	wc := &withCNSet{Controller: &Controller{apiReader: reader}}
	err = wc.persistDrainTransition(reconfake.NewContext(pod.DeepCopy(), cli, nil), a, drainPhaseRequesting, "instance-"+a.CNUUID, mocli.DrainProof{})
	if !apierrors.IsConflict(err) {
		t.Fatalf("want real resourceVersion conflict, got %v", err)
	}
	fresh := &corev1.Pod{}
	if err := cli.Get(ctx, client.ObjectKeyFromObject(pod), fresh); err != nil {
		t.Fatal(err)
	}
	stored, err := readDrainAttempt(fresh)
	if err != nil || stored.Phase != drainPhasePrepared {
		t.Fatalf("conflict advanced phase: %v %#v", err, stored)
	}
	wrongUID := types.UID("same-name-old-instance")
	if err := cli.Delete(ctx, fresh, client.Preconditions{UID: &wrongUID}); !apierrors.IsConflict(err) {
		t.Fatalf("UID precondition not enforced: %v", err)
	}
	oldRV := pod.ResourceVersion
	if err := cli.Delete(ctx, fresh, client.Preconditions{ResourceVersion: &oldRV}); !apierrors.IsConflict(err) {
		t.Fatalf("RV precondition not enforced: %v", err)
	}
	// A successful completion proof must also lose to a concurrent metadata
	// change after its last read, instead of retrying with the old RPC result.
	authorizeTestPod(t, fresh)
	delete(fresh.Labels, v1alpha1.DirectPodLabel)
	if err := cli.Update(ctx, fresh); err != nil {
		t.Fatal(err)
	}
	verified := fresh.DeepCopy()
	reader.afterRead = func() {
		concurrent := &corev1.Pod{}
		if err := cli.Get(ctx, client.ObjectKeyFromObject(pod), concurrent); err != nil {
			t.Fatal(err)
		}
		concurrent.Annotations["test/concurrent"] = "after-completion-read"
		if err := cli.Update(ctx, concurrent); err != nil {
			t.Fatal(err)
		}
	}
	err = wc.completeDraining(reconfake.NewContext(verified.DeepCopy(), cli, nil), verified)
	if !apierrors.IsConflict(err) {
		t.Fatalf("completion should conflict, got %v", err)
	}
	if err := cli.Get(ctx, client.ObjectKeyFromObject(pod), fresh); err != nil {
		t.Fatal(err)
	}
	if len(fresh.Finalizers) != 1 {
		t.Fatal("conflicted completion removed protection")
	}
}
