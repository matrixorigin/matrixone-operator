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
	"fmt"
	"github.com/matrixorigin/matrixone-operator/api/features"
	"github.com/openkruise/kruise-api/apps/pub"
	kruisev1alpha1 "github.com/openkruise/kruise-api/apps/v1alpha1"
	"k8s.io/apimachinery/pkg/util/intstr"
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
	cs  *kruisev1alpha1.CloneSet
	svc *corev1.Service
}

func (c *Actor) with(cs *kruisev1alpha1.CloneSet) *WithResources {
	return &WithResources{Actor: c, cs: cs}
}

func (c *Actor) Observe(ctx *recon.Context[*v1alpha1.CNSet]) (recon.Action[*v1alpha1.CNSet], error) {
	cn := ctx.Obj

	cs := &kruisev1alpha1.CloneSet{}
	err, foundCs := util.IsFound(ctx.Get(client.ObjectKey{Namespace: cn.Namespace, Name: setName(cn)}, cs))
	if err != nil {
		return nil, errors.Wrap(err, "get cn clonset")
	}
	if !foundCs {
		return c.Create, nil
	}

	if features.DefaultFeatureGate.Enabled(features.S3Reclaim) && cn.Deps.LogSet != nil {
		err = v1alpha1.AddBucketFinalizer(ctx.Context, ctx.Client, cn.Deps.LogSet.ObjectMeta, bucketFinalizer(cn))
		if err != nil {
			return nil, errors.Wrap(err, "add bucket finalizer")
		}
	}

	svc := buildSvc(cn)
	if err := recon.CreateOwnedOrUpdate(ctx, svc, func() error {
		syncService(cn, svc)
		return nil
	}); err != nil {
		return nil, errors.Wrap(err, "sync service")
	}

	// diff desired cloneset and determine whether should an update be invoked
	origin := cs.DeepCopy()
	if err := syncCloneSet(ctx, cs); err != nil {
		return nil, err
	}
	if err = ctx.Update(cs, client.DryRunAll); err != nil {
		return nil, errors.Wrap(err, "dry run update cnset")
	}
	if !equality.Semantic.DeepEqual(origin, cs) {
		return c.with(cs).Update, nil
	}
	// calculate status
	var stores []v1alpha1.CNStore
	podList := &corev1.PodList{}
	err = ctx.List(podList, client.InNamespace(cn.Namespace),
		client.MatchingLabels(common.SubResourceLabels(cn)))
	if err != nil {
		return nil, errors.Wrap(err, "list cn pods")
	}
	for _, pod := range podList.Items {
		uid := v1alpha1.GetCNPodUUID(&pod)
		cnState := pod.Annotations[common.CNStateAnno]
		if cnState == "" {
			cnState = v1alpha1.CNStoreStateUnknown
		}
		stores = append(stores, v1alpha1.CNStore{
			UUID:    uid,
			PodName: pod.Name,
			State:   cnState,
		})
	}
	cn.Status.Stores = stores
	cn.Status.Replicas = cs.Status.Replicas
	cn.Status.LabelSelector = cs.Status.LabelSelector
	// sync status from cloneset
	if cs.Status.ReadyReplicas >= cn.Spec.Replicas {
		setReady(cn)
	} else {
		setNotReady(cn)
	}
	if cs.Status.UpdatedReplicas >= cn.Spec.Replicas {
		setSynced(cn)
	} else {
		setNotSynced(cn)
	}

	if features.DefaultFeatureGate.Enabled(features.S3Reclaim) && cn.Deps.LogSet != nil {
		if cs.Status.ReadyReplicas > 0 {
			err = v1alpha1.SyncBucketEverRunningAnn(ctx.Context, ctx.Client, cn.Deps.LogSet.ObjectMeta)
			if err != nil {
				return nil, errors.Wrap(err, "set bucket ever running ann")
			}
		}
	}
	if cn.Spec.Replicas != *cs.Spec.Replicas {
		return c.with(cs).Scale, nil
	}

	if recon.IsReady(&cn.Status.ConditionalStatus) {
		return nil, c.cleanup(ctx)
	}

	return nil, recon.ErrReSync("cnset is not ready", reSyncAfter)
}

func (c *WithResources) Scale(ctx *recon.Context[*v1alpha1.CNSet]) error {
	return ctx.Patch(c.cs, func() error {
		syncReplicas(ctx.Obj, c.cs)
		return nil
	})
}

func (c *WithResources) Update(ctx *recon.Context[*v1alpha1.CNSet]) error {
	return ctx.Update(c.cs)
}

func (c *Actor) cleanup(ctx *recon.Context[*v1alpha1.CNSet]) error {
	sts := &kruise.StatefulSet{}
	err := ctx.Get(client.ObjectKey{Namespace: ctx.Obj.Namespace, Name: setName(ctx.Obj)}, sts)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return errors.Wrap(err, "error check legacy CN statefulset")
	}
	if err := ctx.Delete(sts); err != nil {
		return errors.Wrap(err, "error delete legacy CN statefulset")
	}
	return recon.ErrReSync("wait legacy CNSet deleted")
}

func (c *Actor) Finalize(ctx *recon.Context[*v1alpha1.CNSet]) (bool, error) {
	cn := ctx.Obj

	objs := []client.Object{&kruisev1alpha1.CloneSet{ObjectMeta: metav1.ObjectMeta{
		Name: setName(cn),
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
	if features.DefaultFeatureGate.Enabled(features.S3Reclaim) && cn.Deps.LogSet != nil {
		err := v1alpha1.RemoveBucketFinalizer(ctx.Context, ctx.Client, cn.Deps.LogSet.ObjectMeta, bucketFinalizer(cn))
		if err != nil {
			return false, err
		}
	}
	return true, nil
}

func (c *Actor) Create(ctx *recon.Context[*v1alpha1.CNSet]) error {
	cn := ctx.Obj

	// headless svc for pod dns resolution
	hSvc := buildHeadlessSvc(cn)
	cnSet := buildCNSet(cn, hSvc)
	svc := buildSvc(cn)
	syncReplicas(cn, cnSet)
	if err := syncCloneSet(ctx, cnSet); err != nil {
		return errors.Wrap(err, "sync clone set")
	}
	syncPersistentVolumeClaim(ctx.Obj, cnSet)

	// create all resources
	err := lo.Reduce[client.Object, error]([]client.Object{
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
			b.Owns(&kruisev1alpha1.CloneSet{}).
				Owns(&corev1.Service{})
		}))
	if err != nil {
		return err
	}

	return nil
}
func syncCloneSet(ctx *recon.Context[*v1alpha1.CNSet], cs *kruisev1alpha1.CloneSet) error {
	maxUnavailable := intstr.FromInt(1)
	cs.Spec.UpdateStrategy.Type = kruisev1alpha1.InPlaceIfPossibleCloneSetUpdateStrategyType
	cs.Spec.UpdateStrategy.MaxUnavailable = &maxUnavailable
	cs.Spec.ScaleStrategy.DisablePVCReuse = true
	cs.Spec.ScaleStrategy.MaxUnavailable = &maxUnavailable
	if cs.Spec.Lifecycle == nil {
		cs.Spec.Lifecycle = &pub.Lifecycle{}
	}
	cs.Spec.Lifecycle.PreDelete = &pub.LifecycleHook{
		FinalizersHandler: []string{
			common.CNDrainingFinalizer,
		},
		MarkPodNotReady: true,
	}

	if err := syncPodMeta(ctx.Obj, cs); err != nil {
		return errors.Wrap(err, "sync pod meta")
	}
	if ctx.Dep != nil {
		syncPodSpec(ctx.Obj, cs, ctx.Dep.Deps.LogSet.Spec.SharedStorage)
	}
	// TODO(aylei): CNSet should support update cacheVolume

	cm, err := buildCNSetConfigMap(ctx.Obj, ctx.Dep.Deps.LogSet)
	if err != nil {
		return err
	}
	return common.SyncConfigMap(ctx, &cs.Spec.Template.Spec, cm)
}

func bucketFinalizer(cn *v1alpha1.CNSet) string {
	return fmt.Sprintf("%s-%s-%s", v1alpha1.BucketCNFinalizerPrefix, cn.Namespace, cn.Name)
}

func setReady(cn *v1alpha1.CNSet) {
	cn.Status.SetCondition(metav1.Condition{
		Type:    recon.ConditionTypeReady,
		Status:  metav1.ConditionTrue,
		Message: "cn stores ready",
	})
}

func setNotReady(cn *v1alpha1.CNSet) {
	cn.Status.SetCondition(metav1.Condition{
		Type:    recon.ConditionTypeReady,
		Status:  metav1.ConditionFalse,
		Reason:  common.ReasonNoEnoughReadyStores,
		Message: "cn stores not ready",
	})
}

func setSynced(cn *v1alpha1.CNSet) {
	cn.Status.SetCondition(metav1.Condition{
		Type:    recon.ConditionTypeSynced,
		Status:  metav1.ConditionTrue,
		Message: "cn synced",
	})
}

func setNotSynced(cn *v1alpha1.CNSet) {
	cn.Status.SetCondition(metav1.Condition{
		Type:    recon.ConditionTypeSynced,
		Status:  metav1.ConditionFalse,
		Reason:  common.ReasonNoEnoughUpdatedStores,
		Message: "cn stores not ready",
	})
}
