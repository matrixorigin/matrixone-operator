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

package cnstore

import (
	"context"
	"github.com/go-errors/errors"
	recon "github.com/matrixorigin/controller-runtime/pkg/reconciler"
	"github.com/matrixorigin/controller-runtime/pkg/util"
	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	"github.com/matrixorigin/matrixone-operator/pkg/controllers/common"
	"github.com/matrixorigin/matrixone-operator/pkg/mocli"
	kruisev1alpha1 "github.com/openkruise/kruise-api/apps/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"time"
)

func (c *withCNSet) poolingCNReconcile(ctx *recon.Context[*corev1.Pod]) error {
	pod := ctx.Obj
	uid := v1alpha1.GetCNPodUUID(pod)

	var ready bool
	if err := c.withMOClientSet(ctx, func(ctx context.Context, h *mocli.ClientSet) error {
		_, ready = h.StoreCache.GetCN(uid)
		return nil
	}); err != nil {
		return errors.WrapPrefix(err, "error call hakeeper", 0)
	}

	switch pod.Labels[v1alpha1.CNPodPhaseLabel] {
	case v1alpha1.CNPodPhaseDraining:
		// recycle the pod
		timeStr, ok := pod.Annotations[common.ReclaimedAt]
		if !ok {
			// legacy pod, simply return it to the pool
			return c.patchPhase(ctx, v1alpha1.CNPodPhaseIdle)
		}
		parsed, err := time.Parse(time.RFC3339, timeStr)
		if err != nil {
			return errors.Wrap(err, 0)
		}
		if time.Since(parsed) > c.cn.Spec.ScalingConfig.GetStoreDrainTimeout() {
			ctx.Log.Info("drain pool pod timeout, delete it to avoid workload intervention")
			// handle graceful logic in OnPreparingStop()
			return evictPodGracefully(ctx, pod)
		}
		if time.Since(parsed) < c.cn.Spec.ScalingConfig.GetMinDelayDuration() {
			return recon.ErrReSync("wait min delay duration", retryInterval)
		}
		storeConnection, err := common.GetStoreScore(pod)
		if err != nil {
			return errors.WrapPrefix(err, "error get store connection count", 0)
		}
		if storeConnection.IsSafeToReclaim() {
			if _, ok := pod.Annotations[v1alpha1.DeleteOnReclaimAnno]; ok {
				ctx.Log.Info("delete pod since deleteOnReclaim is set")
				return util.Ignore(apierrors.IsNotFound, ctx.Delete(pod))
			}
			return ctx.Patch(pod, func() error {
				delete(pod.Annotations, common.ReclaimedAt)
				pod.Labels[v1alpha1.CNPodPhaseLabel] = v1alpha1.CNPodPhaseIdle
				return nil
			})
		}
		return recon.ErrReSync("store is still draining", retryInterval)
	case v1alpha1.CNPodPhaseBound, v1alpha1.CNPodPhaseIdle:
		return c.patchCNReadiness(ctx, corev1.ConditionTrue, messageCNStoreReady)
	case v1alpha1.CNPodPhaseUnknown:
		if ready {
			err := ctx.Patch(pod, func() error {
				pod.Labels[v1alpha1.CNPodPhaseLabel] = v1alpha1.CNPodPhaseIdle
				return nil
			})
			if err != nil {
				return errors.WrapPrefix(err, "error patch CN phase idle", 0)
			}
			return c.patchCNReadiness(ctx, corev1.ConditionTrue, messageCNStoreReady)
		}
		ctx.Log.V(4).Info("CN Pod not ready")
		return c.patchCNReadiness(ctx, corev1.ConditionFalse, messageCNStoreNotRegistered)
	case v1alpha1.CNPodPhaseTerminating:
		return nil
	default:
		return errors.Errorf("unkown CN phase %s", pod.Labels[v1alpha1.CNPodPhaseLabel])
	}
}

func (c *withCNSet) patchPhase(ctx *recon.Context[*corev1.Pod], phase string) error {
	err := ctx.Patch(ctx.Obj, func() error {
		ctx.Obj.Labels[v1alpha1.CNPodPhaseLabel] = phase
		return nil
	})
	if err != nil {
		return errors.WrapPrefix(err, "error patch CN phase idle", 0)
	}
	return nil
}

func evictPodGracefully(ctx *recon.Context[*corev1.Pod], pod *corev1.Pod) error {
	p := pod.DeepCopy()
	if err := ctx.Patch(p, func() error {
		if p.Labels == nil {
			p.Labels = map[string]string{}
		}
		p.Labels[kruisev1alpha1.SpecifiedDeleteKey] = "true"

		return nil
	}); err != nil {
		return errors.Wrap(err, 0)
	}
	return nil
}
