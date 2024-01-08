// Copyright 2024 Matrix Origin
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

package cnclaim

import (
	"cmp"
	"context"
	recon "github.com/matrixorigin/controller-runtime/pkg/reconciler"
	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	"github.com/matrixorigin/matrixone-operator/pkg/controllers/common"
	"github.com/matrixorigin/matrixone-operator/pkg/hacli"
	logpb "github.com/matrixorigin/matrixone/pkg/pb/logservice"
	"github.com/matrixorigin/matrixone/pkg/pb/metadata"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"slices"
	"time"
)

// Actor reconciles CN Claim
type Actor struct {
	clientMgr *hacli.HAKeeperClientManager
}

func NewActor(mgr *hacli.HAKeeperClientManager) *Actor {
	return &Actor{clientMgr: mgr}
}

func (r *Actor) Observe(ctx *recon.Context[*v1alpha1.CNClaim]) (recon.Action[*v1alpha1.CNClaim], error) {
	if ctx.Obj.Spec.PodName == "" {
		return r.Bind, nil
	}
	return nil, r.Sync(ctx)
}

func (r *Actor) Bind(ctx *recon.Context[*v1alpha1.CNClaim]) error {
	c := ctx.Obj

	// collect orphan CNs left by former broken reconciliation
	orphanCNs, err := common.ListPods(ctx, client.InNamespace(c.Namespace), client.MatchingLabels{
		v1alpha1.CNPodPhaseLabel:   v1alpha1.CNPodPhaseBound,
		v1alpha1.PodClaimedByLabel: c.Name,
	})
	if err != nil && !apierrors.IsNotFound(err) {
		return errors.Wrap(err, "error get potential orphan CNs")
	}

	// claim CN
	claimedPod, err := r.claimCN(ctx, orphanCNs)
	if err != nil {
		return errors.Wrap(err, "error claim idle CN")
	}

	// no pod available, bound to a certain Pool (maybe we can loosen this constrain)
	if claimedPod == nil && c.Spec.PoolName == "" {
		poolList := &v1alpha1.CNPoolList{}
		if err := ctx.List(poolList, client.InNamespace(c.Namespace)); err != nil {
			return errors.Wrap(err, "error get list CN pools")
		}
		// TODO: multiple matching support (prioritize)
		var pool *v1alpha1.CNPool
		sl := common.MustAsSelector(c.Spec.Selector)
		for i := range poolList.Items {
			if sl.Matches(labels.Set(poolList.Items[i].Spec.PodLabels)) {
				pool = &poolList.Items[i]
				break
			}
		}
		if pool == nil {
			return errors.Wrapf(err, "no matching pool for claim %s/%s", c.Namespace, c.Name)
		}
	}
	// re-bound later
	// TODO: configurable
	return recon.ErrReSync("wait pod to bound", 5*time.Second)
}

func (r *Actor) claimCN(ctx *recon.Context[*v1alpha1.CNClaim], orphans []corev1.Pod) (*corev1.Pod, error) {
	c := ctx.Obj
	claimed, err := r.doClaimCN(ctx, orphans)
	if err != nil {
		return nil, errors.Wrap(err, "error claim CN")
	}
	// claim failed, wait
	if claimed == nil {
		return nil, nil
	}
	// alter CN label and working state
	store, err := r.patchStore(ctx, claimed, logpb.CNStateLabel{
		State:  metadata.WorkState_Working,
		Labels: common.ToStoreLabels(c.Spec.CNLabels),
	})
	if err != nil {
		return nil, errors.Wrapf(err, "error patch Store state of claimed CN %s/%s", claimed.Namespace, claimed.Name)
	}
	if err := r.bindPod(ctx, claimed, store); err != nil {
		return nil, errors.Wrap(err, "error bind pod")
	}
	return claimed, nil
}

func (r *Actor) doClaimCN(ctx *recon.Context[*v1alpha1.CNClaim], orphans []corev1.Pod) (*corev1.Pod, error) {
	c := ctx.Obj

	// bound orphan CN first
	if len(orphans) > 0 {
		if len(orphans) > 1 {
			ctx.Log.Info("multiple orphan CN bound to 1 claim", "count", len(orphans), "claimName", c.Name)
		}
		return &orphans[0], nil
	}

	// ordinary case: no orphans, try claim an idle CN
	baseSelector := common.MustAsSelector(c.Spec.Selector)
	podSelector := baseSelector.Add(common.MustEqual(v1alpha1.CNPodPhaseLabel, v1alpha1.CNPodPhaseIdle))
	if c.Spec.PoolName != "" {
		podSelector = podSelector.Add(common.MustEqual(v1alpha1.PoolNameLabel, c.Spec.PodName))
	}
	idleCNs, err := common.ListPods(ctx, client.InNamespace(c.Namespace), client.MatchingLabelsSelector{Selector: podSelector})
	if err != nil && !apierrors.IsNotFound(err) {
		return nil, errors.Wrap(err, "error list idle Pods")
	}

	slices.SortFunc(idleCNs, priorityFunc(c))
	for i := range idleCNs {
		pod := &idleCNs[i]
		pod.Labels[v1alpha1.CNPodPhaseLabel] = v1alpha1.CNPodPhaseBound
		pod.Labels[v1alpha1.PodClaimedByLabel] = c.Name
		if c.Spec.OwnerName != nil {
			pod.Labels[v1alpha1.ClaimOwnerNameLabel] = *c.Spec.OwnerName
		}
		// atomic operation with optimistic concurrency control, succeed means claimed
		if err := ctx.Update(pod); err != nil {
			if apierrors.IsConflict(err) {
				ctx.Log.Info("CN pod is not up to date, try next", "podName", pod.Name)
			} else {
				ctx.Log.Error(err, "error claim Pod", "podName", pod.Name)
			}
		} else {
			return pod, nil
		}
	}
	return nil, nil
}

func (r *Actor) bindPod(ctx *recon.Context[*v1alpha1.CNClaim], pod *corev1.Pod, store *metadata.CNService) error {
	c := ctx.Obj
	c.Spec.PodName = pod.Name
	c.Spec.PoolName = pod.Labels[v1alpha1.PoolNameLabel]
	if c.Labels == nil {
		c.Labels = map[string]string{}
	}
	c.Labels[v1alpha1.PoolNameLabel] = c.Spec.PoolName
	if err := ctx.Update(c); err != nil {
		return errors.Wrap(err, "error bound pod to claim")
	}

	c.Status.Phase = v1alpha1.CNPodPhaseBound
	c.Status.Store = toStoreStatus(store)
	c.Status.BoundTime = &metav1.Time{Time: time.Now()}
	// if we failed to update status here, observe would help fulfill the status later
	if err := ctx.UpdateStatus(c); err != nil {
		return errors.Wrap(err, "error update claim status")
	}
	return nil
}

func (r *Actor) Sync(ctx *recon.Context[*v1alpha1.CNClaim]) error {
	// TODO: monitor pod health
	return nil
}

func (r *Actor) Finalize(ctx *recon.Context[*v1alpha1.CNClaim]) (bool, error) {
	c := ctx.Obj
	ownedCNs, err := common.ListPods(ctx, client.InNamespace(c.Namespace), client.MatchingLabels{
		v1alpha1.CNPodPhaseLabel:   v1alpha1.CNPodPhaseBound,
		v1alpha1.PodClaimedByLabel: c.Name,
	})
	if err != nil && !apierrors.IsNotFound(err) {
		return false, errors.Wrap(err, "error get owned CNs")
	}
	if len(ownedCNs) == 0 {
		return true, nil
	}
	for i := range ownedCNs {
		cn := ownedCNs[i]
		// TODO(aylei): this will overwrite all labels, keep the base label if necessary
		_, err := r.patchStore(ctx, &cn, logpb.CNStateLabel{
			State: metadata.WorkState_Draining,
			// FIXME(aylei): HAKeeper does not support patch labels to empty yet, use a dummy one
			Labels: map[string]metadata.LabelList{
				"dummy": {Labels: []string{"pool"}},
			},
		})
		if err != nil {
			return false, errors.Wrapf(err, "error drain CN %s/%s", cn.Namespace, cn.Name)
		}
		// set the CN Pod to draining phase and let the draining process handle recycling
		if err := ctx.Patch(&cn, func() error {
			cn.Labels[v1alpha1.CNPodPhaseLabel] = v1alpha1.CNPodPhaseDraining
			delete(cn.Labels, v1alpha1.ClaimOwnerNameLabel)
			return nil
		}); err != nil {
			return false, errors.Wrapf(err, "error reclaim CN %s/%s", cn.Namespace, cn.Name)
		}
	}
	return false, nil
}

func (r *Actor) patchStore(ctx *recon.Context[*v1alpha1.CNClaim], pod *corev1.Pod, req logpb.CNStateLabel) (*metadata.CNService, error) {
	cs, err := common.ResolveCNSet(ctx, pod)
	if err != nil {
		return nil, errors.Wrap(err, "error resolve CNSet")
	}
	ls, err := common.ResolveLogSet(ctx, cs)
	if err != nil {
		return nil, errors.Wrap(err, "error resolve LogSet")
	}
	hc, err := r.clientMgr.GetClient(ls)
	if err != nil {
		return nil, errors.Wrap(err, "error get HAKeeper client")
	}
	timeout, cancel := context.WithTimeout(ctx, hacli.HAKeeperTimeout)
	defer cancel()
	uid := v1alpha1.GetCNPodUUID(pod)
	req.UUID = uid
	err = hc.Client.PatchCNStore(timeout, req)
	if err != nil {
		return nil, errors.Wrap(err, "error patch CNStore")
	}
	cn, ok := hc.StoreCache.GetCN(uid)
	if !ok {
		return nil, errors.Errorf("store not found in cache: %s", uid)
	}
	// the cache may be stale, update it locally
	cn.Labels = req.Labels
	cn.WorkState = req.State
	return &cn, nil
}

func (r *Actor) Start(mgr manager.Manager) error {
	return recon.Setup[*v1alpha1.CNClaim](&v1alpha1.CNClaim{}, "cn-claim-manager", mgr, r)
}

func toStoreStatus(cn *metadata.CNService) v1alpha1.CNStoreStatus {
	var ls []v1alpha1.CNLabel
	for k, v := range cn.Labels {
		ls = append(ls, v1alpha1.CNLabel{
			Key:    k,
			Values: v.Labels,
		})
	}
	slices.SortFunc(ls, func(a, b v1alpha1.CNLabel) int {
		return cmp.Compare(a.Key, b.Key)
	})
	return v1alpha1.CNStoreStatus{
		ServiceID:              cn.ServiceID,
		LockServiceAddress:     cn.LockServiceAddress,
		PipelineServiceAddress: cn.PipelineServiceAddress,
		SQLAddress:             cn.SQLAddress,
		QueryAddress:           cn.QueryAddress,
		WorkState:              int32(cn.WorkState),
		Labels:                 ls,
	}
}

// TODO: label similarity
func priorityFunc(c *v1alpha1.CNClaim) func(a, b corev1.Pod) int {
	return func(a, b corev1.Pod) int {
		return getScore(c, a) - getScore(c, b)
	}
}

func getScore(c *v1alpha1.CNClaim, p corev1.Pod) int {
	if c.Labels[v1alpha1.PodClaimedByLabel] == p.Name {
		return -1
	}
	return 0
}
