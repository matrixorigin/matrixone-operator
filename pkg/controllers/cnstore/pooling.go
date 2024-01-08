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
	recon "github.com/matrixorigin/controller-runtime/pkg/reconciler"
	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	"github.com/matrixorigin/matrixone-operator/pkg/controllers/common"
	"github.com/matrixorigin/matrixone-operator/pkg/hacli"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
)

func (c *withCNSet) poolingCNReconcile(ctx *recon.Context[*corev1.Pod]) error {
	pod := ctx.Obj
	uid := v1alpha1.GetCNPodUUID(pod)

	var ready bool
	if err := c.withHAKeeperClient(ctx, func(ctx context.Context, h *hacli.Handler) error {
		_, ready = h.StoreCache.GetCN(uid)
		return nil
	}); err != nil {
		return errors.Wrap(err, "error call hakeeper")
	}

	switch pod.Labels[v1alpha1.CNPodPhaseLabel] {
	case v1alpha1.CNPodPhaseDraining:
		// recycle the pod
		// TODO(aylei): timeout
		count, err := common.GetStoreConnection(pod)
		if err != nil {
			return errors.Wrap(err, "error get store connection count")
		}
		if count == 0 {
			err := ctx.Patch(pod, func() error {
				pod.Labels[v1alpha1.CNPodPhaseLabel] = v1alpha1.CNPodPhaseIdle
				return nil
			})
			if err != nil {
				return errors.Wrap(err, "error patch CN phase idle")
			}
			return nil
		}
		return recon.ErrReSync("store is still draining", retryInterval)
	case v1alpha1.CNPodPhaseBound, v1alpha1.CNPodPhaseIdle:
		// noop
	case v1alpha1.CNPodPhaseUnknown:
		if ready {
			err := ctx.Patch(pod, func() error {
				pod.Labels[v1alpha1.CNPodPhaseLabel] = v1alpha1.CNPodPhaseIdle
				return nil
			})
			if err != nil {
				return errors.Wrap(err, "error patch CN phase idle")
			}
		}
	default:
		return errors.Errorf("unkown CN phase %s", pod.Labels[v1alpha1.CNPodPhaseLabel])
	}
	return nil
}
