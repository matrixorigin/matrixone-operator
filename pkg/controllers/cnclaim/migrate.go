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
	"fmt"
	"github.com/go-errors/errors"
	recon "github.com/matrixorigin/controller-runtime/pkg/reconciler"
	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	"github.com/matrixorigin/matrixone-operator/pkg/controllers/common"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"time"
)

const (
	propagationDelay = 15 * time.Second

	migrationResyncInterval = 10 * time.Second
)

func (r *Actor) migrate(ctx *recon.Context[*v1alpha1.CNClaim]) error {
	c := ctx.Obj
	if c.Spec.SourcePod == nil {
		return r.completeMigration(ctx)
	}
	source := &corev1.Pod{}
	if err := ctx.Get(types.NamespacedName{Namespace: c.Namespace, Name: c.Spec.SourcePod.PodName}, source); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
	}
	if c.Status.Store.BoundTime == nil {
		return errors.New(fmt.Sprintf("claim store %s/%s bound time is nil", c.Namespace, c.Name))
	}
	if time.Now().Sub(c.Status.Store.BoundTime.Time) < propagationDelay {
		return recon.ErrReSync("target pod is not ready to accept traffic, delay", propagationDelay)
	}
	switch source.Labels[v1alpha1.CNPodPhaseLabel] {
	case v1alpha1.CNPodPhaseDraining:
		if err := r.reportProgress(ctx, source); err != nil {
			return err
		}
		return recon.ErrReSync("source pod is still draining, requeue", migrationResyncInterval)
	case v1alpha1.CNPodPhaseBound:
		// use connection migration to migrate workload from source to target pod
		if err := r.reclaimCN(ctx, source); err != nil {
			return err
		}
		if err := r.reportProgress(ctx, source); err != nil {
			return err
		}
		return recon.ErrReSync("source pod start draining, reqeue", migrationResyncInterval)
	}
	return r.completeMigration(ctx)
}

func (r *Actor) reportProgress(ctx *recon.Context[*v1alpha1.CNClaim], source *corev1.Pod) error {
	c := ctx.Obj
	score, err := common.GetStoreScore(source)
	if err != nil {
		return err
	}
	if err := ctx.PatchStatus(c, func() error {
		c.Status.Migrate = &v1alpha1.MigrateStatus{
			Source: v1alpha1.Workload{
				Connections: score.SessionCount,
				Pipelines:   score.PipelineCount,
			},
		}
		return nil
	}); err != nil {
		return err
	}
	return nil
}

func (r *Actor) completeMigration(ctx *recon.Context[*v1alpha1.CNClaim]) error {
	c := ctx.Obj
	if err := ctx.PatchStatus(c, func() error {
		c.Status.Migrate = nil
		return nil
	}); err != nil {
		return err
	}
	if err := ctx.Patch(c, func() error {
		c.Spec.SourcePod = nil
		return nil
	}); err != nil {
		return err
	}
	return nil
}
