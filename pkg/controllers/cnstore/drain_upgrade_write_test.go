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

	"github.com/matrixorigin/matrixone-operator/pkg/controllers/common"
	"github.com/openkruise/kruise-api/apps/pub"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type upgradeWriteFault struct {
	client.Client
	clear, accepted, fired bool
}

func (c *upgradeWriteFault) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	if pod, ok := obj.(*corev1.Pod); ok && !c.clear && !c.fired && controllerutil.ContainsFinalizer(pod, common.CNDrainingFinalizer) {
		c.fired = true
		if c.accepted {
			if err := c.Client.Update(ctx, obj, opts...); err != nil {
				return err
			}
		}
		return fmt.Errorf("injected finalizer write failure/lost response")
	}
	return c.Client.Update(ctx, obj, opts...)
}

func (c *upgradeWriteFault) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	if pod, ok := obj.(*corev1.Pod); ok && c.clear && !c.fired && pod.Annotations[drainAttemptAnno] == "" {
		c.fired = true
		if c.accepted {
			if err := c.Client.Patch(ctx, obj, patch, opts...); err != nil {
				return err
			}
		}
		return fmt.Errorf("injected proof cleanup failure/lost response")
	}
	return c.Client.Patch(ctx, obj, patch, opts...)
}

func TestObserveUpgradeWriteFailureRecovery(t *testing.T) {
	for _, clear := range []bool{false, true} {
		for _, accepted := range []bool{false, true} {
			t.Run(fmt.Sprintf("clear-%v-accepted-%v", clear, accepted), func(t *testing.T) {
				f := completedUpgradeFixture(t)
				if clear {
					f.round(t)
				}
				fault := &upgradeWriteFault{Client: f.cli, clear: clear, accepted: accepted}
				f.cli = fault
				f.c.apiReader = fault
				p := f.round(t)
				if !fault.fired {
					t.Fatal("did not reach intended write boundary")
				}
				a, err := readDrainAttempt(p)
				if err != nil {
					t.Fatal(err)
				}
				if (a == nil) != (clear && accepted) {
					t.Fatalf("wrong persisted proof after fault: %#v", a)
				}
				if controllerutil.ContainsFinalizer(p, common.CNDrainingFinalizer) != (clear || accepted) {
					t.Fatal("wrong persisted protection after fault")
				}
				if cond := common.GetReadinessCondition(p, common.CNStoreReadiness); cond == nil || cond.Status != corev1.ConditionFalse {
					t.Fatal("failure admitted business")
				}
				f.c = &Controller{apiReader: f.cli, queryCli: f.c.queryCli, clientMgr: f.c.clientMgr, lockClient: f.lock, now: f.c.now, lockIdentity: f.c.lockIdentity}
				for i := 0; i < 3; i++ {
					p = f.round(t)
				}
				if a, err := readDrainAttempt(p); err != nil || a != nil {
					t.Fatalf("proof not recoverable: %#v %v", a, err)
				}
				if !controllerutil.ContainsFinalizer(p, common.CNDrainingFinalizer) {
					t.Fatal("recovery lost protection")
				}
				p.Labels[pub.LifecycleStateKey] = string(pub.LifecycleStateNormal)
				if err := f.cli.Update(context.Background(), p); err != nil {
					t.Fatal(err)
				}
				p = f.round(t)
				if cond := common.GetReadinessCondition(p, common.CNStoreReadiness); cond == nil || cond.Status != corev1.ConditionTrue {
					t.Fatal("recovery never restored business readiness")
				}
				if len(f.lock.calls) != 2 {
					t.Fatalf("recovery repeated handshake: %v", f.lock.calls)
				}
			})
		}
	}
}
