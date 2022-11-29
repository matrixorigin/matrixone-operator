// Copyright 2022 Matrix Origin
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package webui

import (
	recon "github.com/matrixorigin/controller-runtime/pkg/reconciler"
	"github.com/matrixorigin/controller-runtime/pkg/util"
	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	"github.com/matrixorigin/matrixone-operator/pkg/controllers/common"
	"github.com/pkg/errors"
	"github.com/samber/lo"
	"go.uber.org/multierr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"time"
)

const (
	reSyncAfter = 10 * time.Second
)

type Actor struct{}

var _ recon.Actor[*v1alpha1.WebUI] = &Actor{}

type WithResource struct {
	*Actor
	dp  *appsv1.Deployment
	svc *corev1.Service
}

func (w *Actor) with(dp *appsv1.Deployment, svc *corev1.Service) *WithResource {
	return &WithResource{Actor: w, dp: dp, svc: svc}
}

func (w *Actor) Observe(ctx *recon.Context[*v1alpha1.WebUI]) (recon.Action[*v1alpha1.WebUI], error) {
	wi := ctx.Obj

	svc := &corev1.Service{}
	err, foundSvc := util.IsFound(ctx.Get(client.ObjectKey{Namespace: wi.Namespace, Name: webUIName(wi)}, svc))
	if err != nil {
		return nil, errors.Wrap(err, "get webui service")
	}

	dp := &appsv1.Deployment{}
	err, foundDp := util.IsFound(ctx.Get(client.ObjectKey{
		Namespace: wi.Namespace,
		Name:      webUIName(wi),
	}, dp))
	if err != nil {
		return nil, errors.Wrap(err, "get webui deployment")
	}

	if !foundDp || !foundSvc {
		return w.Create, nil
	}

	origin := dp.DeepCopy()
	if err := syncPods(ctx, dp); err != nil {
		return nil, err
	}
	if !equality.Semantic.DeepEqual(origin, dp) {
		return w.with(dp, svc).Update, nil
	}

	// update Service of cnset
	originSvc := svc.DeepCopy()
	syncServiceType(ctx.Obj, svc)
	if !equality.Semantic.DeepEqual(originSvc, svc) {
		return w.with(dp, svc).SvcUpdate, nil
	}

	podList := &corev1.PodList{}
	err = ctx.List(podList, client.InNamespace(wi.Namespace), client.MatchingLabels(common.SubResourceLabels(wi)))
	if err != nil {
		return nil, errors.Wrap(err, "list webui pods")
	}

	common.CollectStoreStatus(&wi.Status.FailoverStatus, podList.Items)

	if len(wi.Status.AvailableStores) >= int(wi.Spec.Replicas) {
		wi.Status.SetCondition(metav1.Condition{
			Type:    recon.ConditionTypeReady,
			Status:  metav1.ConditionTrue,
			Message: "webui pod ready",
		})
	} else {
		wi.Status.SetCondition(metav1.Condition{
			Type:    recon.ConditionTypeReady,
			Status:  metav1.ConditionFalse,
			Reason:  common.ReasonNoEnoughReadyStores,
			Message: "webui pod not ready",
		})
	}

	if recon.IsReady(&wi.Status.ConditionalStatus) {
		return nil, nil
	}

	return nil, recon.ErrReSync("webui is not ready", reSyncAfter)
}

func (w *Actor) Finalize(ctx *recon.Context[*v1alpha1.WebUI]) (bool, error) {
	wi := ctx.Obj

	objs := []client.Object{&corev1.Service{ObjectMeta: metav1.ObjectMeta{
		Name: webUIName(wi),
	}}, &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{
		Name: webUIName(wi),
	}}}
	for _, obj := range objs {
		obj.SetNamespace(wi.Namespace)
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

func (w *Actor) Create(ctx *recon.Context[*v1alpha1.WebUI]) error {
	wi := ctx.Obj
	ctx.Log.Info("create webui service")

	wiObj := buildWebUI(wi)
	wiSvc := buildService(wi)
	syncReplicas(wi, wiObj)
	syncPodMeta(wi, wiObj)
	syncPodSpec(wi, wiObj)

	configMap, err := buildConfigMap(wi)
	if err != nil {
		return err
	}

	if err := common.SyncConfigMap(ctx, &wiObj.Spec.Template.Spec, configMap); err != nil {
		return err
	}

	// create all resources
	err = lo.Reduce[client.Object, error]([]client.Object{
		wiSvc,
		wiObj,
	}, func(errs error, o client.Object, _ int) error {
		err := ctx.CreateOwned(o)
		return multierr.Append(errs, util.Ignore(apierrors.IsAlreadyExists, err))
	}, nil)
	if err != nil {
		return errors.Wrap(err, "create webui service")
	}

	return nil
}

func (r *WithResource) Update(ctx *recon.Context[*v1alpha1.WebUI]) error {
	return ctx.Update(r.dp)
}

func (r *WithResource) SvcUpdate(ctx *recon.Context[*v1alpha1.WebUI]) error {
	return ctx.Patch(r.svc, func() error {
		syncServiceType(ctx.Obj, r.svc)
		return nil
	})

}

func (w *Actor) Reconcile(mgr manager.Manager) error {
	err := recon.Setup[*v1alpha1.WebUI](&v1alpha1.WebUI{}, "webui", mgr, w,
		recon.WithBuildFn(func(b *builder.Builder) {
			b.Owns(&appsv1.Deployment{}).
				Owns(&corev1.Service{})
		}))
	if err != nil {
		return err
	}

	return nil
}
