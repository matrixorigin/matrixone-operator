// Copyright 2023 Matrix Origin
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

package cnset

import (
	"time"

	recon "github.com/matrixorigin/controller-runtime/pkg/reconciler"
	"github.com/matrixorigin/controller-runtime/pkg/util"
	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	"github.com/matrixorigin/matrixone-operator/pkg/controllers/common"
	kruise "github.com/openkruise/kruise-api/apps/v1beta1"
	"github.com/pkg/errors"
	"github.com/samber/lo"
	"go.uber.org/multierr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

// reconcile configuration
const (
	storeDownTimeOut = 1 * time.Minute
	reSyncAfter      = 10 * time.Second
)

type Actor struct{}

var _ recon.Actor[*v1alpha1.CNSet] = &Actor{}

type WithResources struct {
	*Actor
	sts *kruise.StatefulSet
	svc *corev1.Service
}

func (c *Actor) with(sts *kruise.StatefulSet, svc *corev1.Service) *WithResources {
	return &WithResources{Actor: c, sts: sts, svc: svc}
}

func (c *Actor) Observe(ctx *recon.Context[*v1alpha1.CNSet]) (recon.Action[*v1alpha1.CNSet], error) {
	cn := ctx.Obj

	svc := &corev1.Service{}
	err, foundSvc := util.IsFound(ctx.Get(client.ObjectKey{Namespace: cn.Namespace, Name: svcName(cn)}, svc))
	if err != nil {
		return nil, errors.Wrap(err, "get cn service")
	}

	sts := &kruise.StatefulSet{}
	err, foundSts := util.IsFound(ctx.Get(client.ObjectKey{Namespace: cn.Namespace, Name: stsName(cn)}, sts))
	if err != nil {
		return nil, errors.Wrap(err, "get cn statefulset")
	}

	if !foundSts || !foundSvc {
		return c.Create, nil
	}

	// update statefulset of cnset
	origin := sts.DeepCopy()
	if err := syncPods(ctx, sts); err != nil {
		return nil, err
	}
	if err = ctx.Update(sts, client.DryRunAll); err != nil {
		return nil, errors.Wrap(err, "dry run update cnset statefulset")
	}
	if !equality.Semantic.DeepEqual(origin, sts) {
		return c.with(sts, svc).Update, nil
	}

	// update service of cnset
	originSvc := svc.DeepCopy()
	syncService(ctx.Obj, svc)
	if !equality.Semantic.DeepEqual(originSvc, svc) {
		return c.with(sts, svc).SvcUpdate, nil
	}

	// collect cn status
	podList := &corev1.PodList{}
	err = ctx.List(podList, client.InNamespace(cn.Namespace), client.MatchingLabels(common.SubResourceLabels(cn)))
	if err != nil {
		return nil, errors.Wrap(err, "list cnset pods")
	}

	common.CollectStoreStatus(&cn.Status.FailoverStatus, podList.Items)

	if len(cn.Status.AvailableStores) >= int(cn.Spec.Replicas) {
		cn.Status.SetCondition(metav1.Condition{
			Type:    recon.ConditionTypeReady,
			Status:  metav1.ConditionTrue,
			Message: "cn stores ready",
		})

	} else {
		cn.Status.SetCondition(metav1.Condition{
			Type:    recon.ConditionTypeReady,
			Status:  metav1.ConditionFalse,
			Reason:  common.ReasonNoEnoughReadyStores,
			Message: "cn stores not ready",
		})
	}

	switch {
	case len(cn.Status.StoresFailedFor(storeDownTimeOut)) > 0:
		return c.with(sts, svc).Repair, nil
	case cn.Spec.Replicas != *sts.Spec.Replicas:
		return c.with(sts, svc).Scale, nil
	}

	if recon.IsReady(&cn.Status.ConditionalStatus) {
		return nil, nil
	}

	return nil, recon.ErrReSync("cnset is not ready", reSyncAfter)
}

func (c *WithResources) Scale(ctx *recon.Context[*v1alpha1.CNSet]) error {
	return ctx.Patch(c.sts, func() error {
		syncReplicas(ctx.Obj, c.sts)
		return nil
	})
}

func (c *WithResources) Update(ctx *recon.Context[*v1alpha1.CNSet]) error {
	return ctx.Update(c.sts)
}

func (c *WithResources) SvcUpdate(ctx *recon.Context[*v1alpha1.CNSet]) error {
	return ctx.Patch(c.svc, func() error {
		syncService(ctx.Obj, c.svc)
		return nil
	})
}

func (c *WithResources) Repair(ctx *recon.Context[*v1alpha1.CNSet]) error {
	toRepair := ctx.Obj.Status.FailoverStatus.StoresFailedFor(storeDownTimeOut)
	if len(toRepair) == 0 {
		return nil
	}

	// repair one at a time
	ordinal, err := util.PodOrdinal(toRepair[0].PodName)
	if err != nil {
		return errors.Wrapf(err, "error parse ordinal from pod name %s", toRepair[0].PodName)
	}
	c.sts.Spec.ReserveOrdinals = util.Upsert(c.sts.Spec.ReserveOrdinals, ordinal)

	return nil
}

func (c *Actor) Finalize(ctx *recon.Context[*v1alpha1.CNSet]) (bool, error) {
	cn := ctx.Obj

	objs := []client.Object{&corev1.Service{ObjectMeta: metav1.ObjectMeta{
		Name: headlessSvcName(cn),
	}}, &kruise.StatefulSet{ObjectMeta: metav1.ObjectMeta{
		Name: stsName(cn),
	}}, &corev1.Service{ObjectMeta: metav1.ObjectMeta{
		Name: svcName(cn),
	}}}
	for _, obj := range objs {
		obj.SetNamespace(cn.Namespace)
		if err := util.Ignore(apierrors.IsNotFound, ctx.Delete(obj)); err != nil {
			return false, err
		}
	}
	for _, obj := range objs {
		exist, err := ctx.Exist(client.ObjectKeyFromObject(obj), obj)
		if err != nil {
			return false, err
		}
		if exist {
			return false, nil
		}
	}

	return true, nil
}

func (c *Actor) Create(ctx *recon.Context[*v1alpha1.CNSet]) error {
	cn := ctx.Obj

	hSvc := buildHeadlessSvc(cn)
	cnSet := buildCNSet(cn)
	svc := buildSvc(cn)
	syncReplicas(cn, cnSet)
	syncPodMeta(cn, cnSet)
	syncPodSpec(cn, cnSet, ctx.Dep.Deps.LogSet.Spec.SharedStorage)
	syncPersistentVolumeClaim(cn, cnSet)

	configMap, err := buildCNSetConfigMap(cn, ctx.Dep.Deps.LogSet)
	if err != nil {
		return err
	}

	if err := common.SyncConfigMap(ctx, &cnSet.Spec.Template.Spec, configMap); err != nil {
		return err
	}

	// create all resources
	err = lo.Reduce[client.Object, error]([]client.Object{
		hSvc,
		svc,
		cnSet,
	}, func(errs error, o client.Object, _ int) error {
		err := ctx.CreateOwned(o)
		return multierr.Append(errs, util.Ignore(apierrors.IsAlreadyExists, err))
	}, nil)
	if err != nil {
		return errors.Wrap(err, "create cn service")
	}

	return nil
}

func (c *Actor) Reconcile(mgr manager.Manager) error {
	err := recon.Setup[*v1alpha1.CNSet](&v1alpha1.CNSet{}, "cnset", mgr, c,
		recon.WithBuildFn(func(b *builder.Builder) {
			b.Owns(&kruise.StatefulSet{}).
				Owns(&corev1.Service{})
		}))
	if err != nil {
		return err
	}

	return nil
}
func syncPods(ctx *recon.Context[*v1alpha1.CNSet], sts *kruise.StatefulSet) error {
	cm, err := buildCNSetConfigMap(ctx.Obj, ctx.Dep.Deps.LogSet)
	if err != nil {
		return err
	}

	syncPodMeta(ctx.Obj, sts)

	if ctx.Dep != nil {
		syncPodSpec(ctx.Obj, sts, ctx.Dep.Deps.LogSet.Spec.SharedStorage)
	}

	return common.SyncConfigMap(ctx, &sts.Spec.Template.Spec, cm)
}
