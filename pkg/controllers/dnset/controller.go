// Copyright 2022 Matrix Origin
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

package dnset

import (
	"fmt"
	"time"

	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	"github.com/matrixorigin/matrixone-operator/pkg/controllers/common"
	recon "github.com/matrixorigin/matrixone-operator/runtime/pkg/reconciler"
	"github.com/matrixorigin/matrixone-operator/runtime/pkg/util"
	kruise "github.com/openkruise/kruise-api/apps/v1beta1"
	"github.com/pkg/errors"
	"github.com/samber/lo"
	"go.uber.org/multierr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

const (
	storeDownTimeout = 1 * time.Minute
	reSyncAfter      = 10 * time.Second
)

type Actor struct{}

var _ recon.Actor[*v1alpha1.DNSet] = &Actor{}

type WithResources struct {
	*Actor
	sts *kruise.StatefulSet
	svc *corev1.Service
}

func (d *Actor) with(sts *kruise.StatefulSet, svc *corev1.Service) *WithResources {
	return &WithResources{Actor: d, sts: sts, svc: svc}
}

func (d *Actor) Observe(ctx *recon.Context[*v1alpha1.DNSet]) (recon.Action[*v1alpha1.DNSet], error) {
	dn := ctx.Obj

	svc := &corev1.Service{}
	err, foundSvc := util.IsFound(ctx.Get(client.ObjectKey{Namespace: dn.Namespace, Name: headlessSvcName(dn)}, svc))
	if err != nil {
		return nil, errors.Wrap(err, "get dn service")
	}

	sts := &kruise.StatefulSet{}
	err, foundSts := util.IsFound(ctx.Get(client.ObjectKey{
		Namespace: dn.Namespace,
		Name:      stsName(dn),
	}, sts))
	if err != nil {
		return nil, errors.Wrap(err, "get dn service statefulset")
	}

	if !foundSts || !foundSvc {
		return d.Create, nil
	}

	podList := &corev1.PodList{}
	err = ctx.List(podList, client.InNamespace(dn.Namespace), client.MatchingLabels(common.SubResourceLabels(dn)))
	if err != nil {
		return nil, errors.Wrap(err, "list dn pods")
	}
	common.CollectStoreStatus(&dn.Status.FailoverStatus, podList.Items)

	if len(dn.Status.AvailableStores) >= int(dn.Spec.Replicas) {
		dn.Status.SetCondition(metav1.Condition{
			Type:   recon.ConditionTypeReady,
			Status: metav1.ConditionTrue,
		})
	} else {
		dn.Status.SetCondition(metav1.Condition{
			Type:   recon.ConditionTypeReady,
			Status: metav1.ConditionFalse,
			Reason: common.ReasonNoEnoughReadyStores,
		})
	}

	switch {
	case len(dn.Status.StoresFailedFor(storeDownTimeout)) > 0:
		return d.with(sts, svc).Repair, nil
	case dn.Spec.Replicas != *sts.Spec.Replicas:
		return d.with(sts, svc).Scale, nil
	}

	origin := sts.DeepCopy()
	if err := syncPods(ctx, sts); err != nil {
		return nil, err
	}

	if !equality.Semantic.DeepEqual(origin, sts) {
		return d.with(sts, svc).Update, nil
	}

	// update service of dnset
	originSvc := svc.DeepCopy()
	fmt.Println("hello")
	if !equality.Semantic.DeepEqual(originSvc, svc) {
		return d.with(sts, svc).SvcUpdate, nil
	}

	if recon.IsReady(&dn.Status.ConditionalStatus) {
		return nil, nil
	}

	return nil, recon.ErrReSync("dnset is not ready", reSyncAfter)
}

func (d *Actor) Finalize(ctx *recon.Context[*v1alpha1.DNSet]) (bool, error) {
	dn := ctx.Obj

	objs := []client.Object{&corev1.Service{ObjectMeta: metav1.ObjectMeta{
		Name: headlessSvcName(dn),
	}}, &kruise.StatefulSet{ObjectMeta: metav1.ObjectMeta{
		Name: stsName(dn),
	}}}
	for _, obj := range objs {
		obj.SetNamespace(dn.Namespace)
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

func (d *Actor) Create(ctx *recon.Context[*v1alpha1.DNSet]) error {
	klog.V(recon.Info).Info("create dn set...")
	dn := ctx.Obj

	hSvc := buildHeadlessSvc(dn)
	dnSet := buildDNSet(dn)
	syncReplicas(dn, dnSet)
	syncPodMeta(dn, dnSet)
	syncPodSpec(dn, dnSet, ctx.Dep.Deps.LogSet.Spec.SharedStorage)
	syncPersistentVolumeClaim(dn, dnSet)

	configMap, err := buildDNSetConfigMap(dn, ctx.Dep.Deps.LogSet)
	if err != nil {
		return err
	}

	if err := common.SyncConfigMap(ctx, &dnSet.Spec.Template.Spec, configMap); err != nil {
		return err
	}

	// create all resources
	err = lo.Reduce[client.Object, error]([]client.Object{
		hSvc,
		dnSet,
	}, func(errs error, o client.Object, _ int) error {
		err := ctx.CreateOwned(o)
		return multierr.Append(errs, util.Ignore(apierrors.IsAlreadyExists, err))
	}, nil)
	if err != nil {
		return errors.Wrap(err, "create dn service")
	}

	return nil
}

func (r *WithResources) Scale(ctx *recon.Context[*v1alpha1.DNSet]) error {
	return ctx.Patch(r.sts, func() error {
		syncReplicas(ctx.Obj, r.sts)
		return nil
	})
}

func (r *WithResources) Update(ctx *recon.Context[*v1alpha1.DNSet]) error {
	return ctx.Update(r.sts)
}

func (r *WithResources) SvcUpdate(ctx *recon.Context[*v1alpha1.DNSet]) error {
	return ctx.Patch(r.svc, func() error {
		sync
		return nil
	})
}

func (r *WithResources) Repair(ctx *recon.Context[*v1alpha1.DNSet]) error {
	toRepair := ctx.Obj.Status.StoresFailedFor(storeDownTimeout)
	if len(toRepair) == 0 {
		return nil
	}

	// repair one at a time
	ordinal, err := util.PodOrdinal(toRepair[0].PodName)
	if err != nil {
		return errors.Wrapf(err, "error parse ordinal from pod name %s", toRepair[0].PodName)
	}
	r.sts.Spec.ReserveOrdinals = util.Upsert(r.sts.Spec.ReserveOrdinals, ordinal)
	return nil
}

func (d *Actor) Reconcile(mgr manager.Manager) error {
	err := recon.Setup[*v1alpha1.DNSet](&v1alpha1.DNSet{}, "dnset", mgr, d,
		recon.WithBuildFn(func(b *builder.Builder) {
			b.Owns(&kruise.StatefulSet{}).
				Owns(&corev1.Service{})
		}))
	if err != nil {
		return err
	}

	return nil
}
