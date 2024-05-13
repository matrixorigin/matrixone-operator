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
	"github.com/go-errors/errors"
	recon "github.com/matrixorigin/controller-runtime/pkg/reconciler"
	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	"github.com/matrixorigin/matrixone-operator/pkg/controllers/common"
	"github.com/matrixorigin/matrixone-operator/pkg/mocli"
	logpb "github.com/matrixorigin/matrixone/pkg/pb/logservice"
	"github.com/matrixorigin/matrixone/pkg/pb/metadata"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"slices"
	"time"
)

const (
	waitCacheTimeout = 10 * time.Second

	requeueAfter = 5 * time.Second
)

// Actor reconciles CN Claim
type Actor struct {
	clientMgr *mocli.MORPCClientManager
}

func NewActor(mgr *mocli.MORPCClientManager) *Actor {
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
	c.Status.Phase = v1alpha1.CNClaimPhasePending
	ctx.Log.Info("start bind cn claim")

	// collect orphan CNs left by former broken reconciliation
	orphanCNs, err := common.ListPods(ctx, client.InNamespace(c.Namespace), client.MatchingLabels{
		v1alpha1.CNPodPhaseLabel:   v1alpha1.CNPodPhaseBound,
		v1alpha1.PodClaimedByLabel: c.Name,
	})
	if err != nil && !apierrors.IsNotFound(err) {
		return errors.WrapPrefix(err, "error get potential orphan CNs", 0)
	}

	// claim CN
	claimedPod, err := r.claimCN(ctx, orphanCNs)
	if err != nil {
		return errors.WrapPrefix(err, "error claim idle CN", 0)
	}

	// no pod available, bound to a certain Pool (maybe we can loosen this constrain)
	if claimedPod == nil && c.Spec.PoolName == "" {
		ctx.Log.Info("no idle CN available, try to find a matching pool")
		poolList := &v1alpha1.CNPoolList{}
		if err := ctx.List(poolList, client.InNamespace(c.Namespace)); err != nil {
			return errors.WrapPrefix(err, "error get list CN pools", 0)
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
			return recon.ErrReSync("cannot find matching pool, requeue", requeueAfter)
		}
		c.Spec.PoolName = pool.Name
		c.Labels[v1alpha1.PoolNameLabel] = c.Spec.PoolName
		if err := ctx.Update(c); err != nil {
			return errors.WrapPrefix(err, "error bind claim to pool", 0)
		}
	}
	// re-bound later
	// TODO: configurable
	return recon.ErrReSync("wait pod to bound", requeueAfter)
}

func (r *Actor) claimCN(ctx *recon.Context[*v1alpha1.CNClaim], orphans []corev1.Pod) (*corev1.Pod, error) {
	c := ctx.Obj
	claimed, err := r.doClaimCN(ctx, orphans)
	if err != nil {
		return nil, errors.WrapPrefix(err, "error claim CN", 0)
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
		return nil, errors.Wrap(err, 0)
	}
	if err := r.bindPod(ctx, claimed, store); err != nil {
		return nil, errors.WrapPrefix(err, "error bind pod", 0)
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
		podSelector = podSelector.Add(common.MustEqual(v1alpha1.PoolNameLabel, c.Spec.PoolName))
	}
	idleCNs, err := common.ListPods(ctx, client.InNamespace(c.Namespace), client.MatchingLabelsSelector{Selector: podSelector})
	if err != nil && !apierrors.IsNotFound(err) {
		return nil, errors.WrapPrefix(err, "error list idle Pods", 0)
	}

	sortCNByPriority(c, idleCNs)
	for i := range idleCNs {
		pod := &idleCNs[i]
		pod.Labels[v1alpha1.CNPodPhaseLabel] = v1alpha1.CNPodPhaseBound
		pod.Labels[v1alpha1.PodClaimedByLabel] = c.Name
		// pod belongs to a ClaimSet
		csName := c.Labels[v1alpha1.ClaimSetNameLabel]
		if csName != "" {
			pod.Labels[v1alpha1.ClaimSetNameLabel] = csName
		}
		if c.Spec.OwnerName != nil {
			pod.Labels[v1alpha1.PodOwnerNameLabel] = *c.Spec.OwnerName
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
		return errors.WrapPrefix(err, "error bound pod to claim", 0)
	}

	c.Status.Phase = v1alpha1.CNPodPhaseBound
	c.Status.Store = toStoreStatus(store)
	c.Status.BoundTime = &metav1.Time{Time: time.Now()}
	// if we failed to update status here, observe would help fulfill the status later
	if err := ctx.UpdateStatus(c); err != nil {
		return errors.WrapPrefix(err, "error update claim status", 0)
	}
	return nil
}

func (r *Actor) Sync(ctx *recon.Context[*v1alpha1.CNClaim]) error {
	c := ctx.Obj
	switch c.Status.Phase {
	case v1alpha1.CNClaimPhasePending:
		return errors.Errorf("CN Claim %s/%s is pending, should bind it first", c.Namespace, c.Name)
	case v1alpha1.CNClaimPhaseLost:
		return nil
	case v1alpha1.CNClaimPhaseBound, v1alpha1.CNClaimPhaseOutdated:
		// noop
	default:
		return errors.Errorf("CN Claim %s/%s is in unknown phase %s", c.Namespace, c.Name, c.Status.Phase)
	}
	pod := &corev1.Pod{}
	err := ctx.Get(types.NamespacedName{Namespace: c.Namespace, Name: c.Spec.PodName}, pod)
	if err != nil {
		if apierrors.IsNotFound(err) {
			if c.Status.BoundTime != nil && time.Since(c.Status.BoundTime.Time) < waitCacheTimeout {
				return recon.ErrReSync("pod status may be not update to date, wait", waitCacheTimeout)
			}
			c.Status.Phase = v1alpha1.CNClaimPhaseLost
			return nil
		}
	}
	if pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodUnknown {
		c.Status.Phase = v1alpha1.CNClaimPhaseLost
		return nil
	}
	return nil
}

func (r *Actor) Finalize(ctx *recon.Context[*v1alpha1.CNClaim]) (bool, error) {
	c := ctx.Obj
	ownedCNs, err := common.ListPods(ctx, client.InNamespace(c.Namespace), client.MatchingLabels{
		v1alpha1.CNPodPhaseLabel:   v1alpha1.CNPodPhaseBound,
		v1alpha1.PodClaimedByLabel: c.Name,
	})
	if err != nil && !apierrors.IsNotFound(err) {
		return false, errors.WrapPrefix(err, "error get owned CNs", 0)
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
			return false, errors.Wrap(err, 0)
		}
		// set the CN Pod to draining phase and let the draining process handle recycling
		if err := ctx.Patch(&cn, func() error {
			cn.Labels[v1alpha1.CNPodPhaseLabel] = v1alpha1.CNPodPhaseDraining
			delete(cn.Labels, v1alpha1.PodClaimedByLabel)
			if cn.Annotations == nil {
				cn.Annotations = map[string]string{}
			}
			cn.Annotations[common.ReclaimedAt] = time.Now().Format(time.RFC3339)
			return nil
		}); err != nil {
			return false, errors.Wrap(err, 0)
		}
	}
	return false, nil
}

func (r *Actor) patchStore(ctx *recon.Context[*v1alpha1.CNClaim], pod *corev1.Pod, req logpb.CNStateLabel) (*metadata.CNService, error) {
	cs, err := common.ResolveCNSet(ctx, pod)
	if err != nil {
		return nil, errors.WrapPrefix(err, "error resolve CNSet", 0)
	}
	ls, err := common.ResolveLogSet(ctx, cs)
	if err != nil {
		return nil, errors.WrapPrefix(err, "error resolve LogSet", 0)
	}
	hc, err := r.clientMgr.GetClient(ls)
	if err != nil {
		return nil, errors.WrapPrefix(err, "error get HAKeeper client", 0)
	}
	timeout, cancel := context.WithTimeout(ctx, mocli.DefaultRPCTimeout)
	defer cancel()
	uid := v1alpha1.GetCNPodUUID(pod)
	req.UUID = uid
	err = hc.Client.PatchCNStore(timeout, req)
	if err != nil {
		return nil, errors.WrapPrefix(err, "error patch CNStore", 0)
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
	return recon.Setup[*v1alpha1.CNClaim](&v1alpha1.CNClaim{}, "cn-claim-manager", mgr, r, recon.WithBuildFn(func(b *builder.Builder) {
		b.Watches(&corev1.Pod{}, handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, object client.Object) []reconcile.Request {
			pod, ok := object.(*corev1.Pod)
			if !ok {
				return nil
			}
			claimName, ok := pod.Labels[v1alpha1.PodClaimedByLabel]
			if !ok {
				return nil
			}
			return []reconcile.Request{{
				NamespacedName: types.NamespacedName{
					Namespace: pod.Namespace,
					Name:      claimName,
				},
			}}
		}))
	}))
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

func sortCNByPriority(c *v1alpha1.CNClaim, pods []corev1.Pod) {
	slices.SortFunc(pods, priorityFunc(c))
}

func priorityFunc(c *v1alpha1.CNClaim) func(a, b corev1.Pod) int {
	return func(a, b corev1.Pod) int {
		// 1. claim the previously used pod first
		ownedA := previouslyOwned(c, a)
		ownedB := previouslyOwned(c, b)
		if ownedA != ownedB {
			return ownedA - ownedB
		}

		// 2. then we prefer older pod
		return a.CreationTimestamp.Second() - b.CreationTimestamp.Second()
	}
}

func previouslyOwned(c *v1alpha1.CNClaim, p corev1.Pod) int {
	if c.Spec.OwnerName != nil && p.Labels[v1alpha1.PodOwnerNameLabel] == *c.Spec.OwnerName {
		return 0
	}
	return 1
}
