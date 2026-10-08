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

	reconfake "github.com/matrixorigin/controller-runtime/pkg/fake"
	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	"github.com/matrixorigin/matrixone-operator/pkg/controllers/common"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type deleteObservationClient struct {
	client.Client
	accepted bool
	fail     bool
	calls    int
}

func (c *deleteObservationClient) Delete(ctx context.Context, obj client.Object, options ...client.DeleteOption) error {
	c.calls++
	opts := &client.DeleteOptions{}
	for _, option := range options {
		option.ApplyToDelete(opts)
	}
	if opts.Preconditions == nil || opts.Preconditions.UID == nil || *opts.Preconditions.UID != obj.GetUID() ||
		opts.Preconditions.ResourceVersion == nil || *opts.Preconditions.ResourceVersion != obj.GetResourceVersion() {
		return fmt.Errorf("delete without verified preconditions")
	}
	if !c.fail || c.accepted {
		if err := c.Client.Delete(ctx, obj, options...); err != nil {
			return err
		}
	}
	if c.fail {
		return fmt.Errorf("injected lost delete response")
	}
	return nil
}

func TestObserveDirectDeleteResumesAfterRestart(t *testing.T) {
	for _, accepted := range []bool{false, true} {
		t.Run(fmt.Sprintf("accepted-%v", accepted), func(t *testing.T) {
			f := newObserveFixture(t)
			p := f.read(t)
			p.Labels[v1alpha1.DirectPodLabel] = "true"
			if err := f.cli.Update(context.Background(), p); err != nil {
				t.Fatal(err)
			}
			fault := &deleteObservationClient{Client: f.cli, fail: true, accepted: accepted}
			f.cli = fault
			f.c.apiReader = fault
			for i := 0; i < 4; i++ {
				f.round(t)
			}
			p = f.read(t)
			a, err := readDrainAttempt(p)
			if err != nil || a == nil || a.Phase != drainPhaseCompleted || !controllerutil.ContainsFinalizer(p, common.CNDrainingFinalizer) {
				t.Fatalf("delete failure lost authorization/protection: %#v %v", a, err)
			}
			if accepted != !p.DeletionTimestamp.IsZero() {
				t.Fatal("wrong acceptance state")
			}
			// Reconstruct the controller, keeping only injected external clients.
			f.c = &Controller{apiReader: fault, queryCli: f.c.queryCli, clientMgr: f.c.clientMgr, lockClient: f.lock, now: f.c.now, lockIdentity: f.c.lockIdentity}
			fault.fail = false
			if !accepted {
				f.round(t)
			}
			_, _ = f.c.Observe(reconfake.NewContext(f.read(t), f.cli, nil))
			err = f.cli.Get(context.Background(), client.ObjectKeyFromObject(f.pod), &corev1.Pod{})
			if !apierrors.IsNotFound(err) {
				t.Fatalf("terminal deletion not reconciled: %v", err)
			}
			if len(f.lock.calls) != 2 {
				t.Fatalf("restart repeated lock requests: %v", f.lock.calls)
			}
			want := 2
			if accepted {
				want = 1
			}
			if fault.calls != want {
				t.Fatalf("delete calls: %d want %d", fault.calls, want)
			}
		})
	}
}
