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

package dnset

import (
	"github.com/matrixorigin/matrixone-operator/api/features"
	"github.com/matrixorigin/matrixone-operator/pkg/utils"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"strconv"
	"time"

	"github.com/go-errors/errors"
	recon "github.com/matrixorigin/controller-runtime/pkg/reconciler"
	"github.com/matrixorigin/controller-runtime/pkg/util"
	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	"github.com/matrixorigin/matrixone-operator/pkg/controllers/common"
	kruise "github.com/openkruise/kruise-api/apps/v1beta1"
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

	ctx.Log.Info("observe dnset")
	svc := &corev1.Service{}
	err, foundSvc := util.IsFound(ctx.Get(client.ObjectKey{Namespace: dn.Namespace, Name: headlessSvcName(dn)}, svc))
	if err != nil {
		return nil, errors.WrapPrefix(err, "get dn service", 0)
	}

	sts := &kruise.StatefulSet{}
	err, foundSts := util.IsFound(ctx.Get(client.ObjectKey{
		Namespace: dn.Namespace,
		Name:      stsName(dn),
	}, sts))
	if err != nil {
		return nil, errors.WrapPrefix(err, "get dn service statefulset", 0)
	}

	if !foundSts || !foundSvc {
		return d.Create, nil
	}

	if features.DefaultFeatureGate.Enabled(features.S3Reclaim) && dn.Deps.LogSet != nil {
		err = v1alpha1.AddBucketFinalizer(ctx.Context, ctx.Client, dn.Deps.LogSet.ObjectMeta, utils.MakeHashFinalizer(v1alpha1.BucketDNFinalizerPrefix, dn))
		if err != nil {
			return nil, errors.WrapPrefix(err, "add bucket finalizer", 0)
		}
	}
	podList := &corev1.PodList{}
	err = ctx.List(podList, client.InNamespace(dn.Namespace), client.MatchingLabels(common.SubResourceLabels(dn)))
	if err != nil {
		return nil, errors.WrapPrefix(err, "list dn pods", 0)
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

	if features.DefaultFeatureGate.Enabled(features.S3Reclaim) && dn.Deps.LogSet != nil {
		if len(dn.Status.AvailableStores) > 0 {
			err = v1alpha1.SyncBucketEverRunningAnn(ctx.Context, ctx.Client, dn.Deps.LogSet.ObjectMeta)
			if err != nil {
				return nil, errors.WrapPrefix(err, "set bucket ever running ann", 0)
			}
		}
	}

	switch {
	case dn.Spec.Replicas != *sts.Spec.Replicas:
		return d.with(sts, svc).Scale, nil
	}

	origin := sts.DeepCopy()
	if err := syncPods(ctx, sts); err != nil {
		return nil, err
	}

	if err = ctx.Update(sts, client.DryRunAll); err != nil {
		return nil, errors.WrapPrefix(err, "dry run update dnset statefulset", 0)
	}

	if !equality.Semantic.DeepEqual(origin, sts) {
		return d.with(sts, svc).Update, nil
	}

	if err := d.syncMetricService(ctx); err != nil {
		return nil, errors.WrapPrefix(err, "sync metric service", 0)
	}

	if recon.IsReady(&dn.Status.ConditionalStatus) && sts.Status.UpdatedReadyReplicas >= dn.Spec.Replicas {
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
	if features.DefaultFeatureGate.Enabled(features.S3Reclaim) && dn.Deps.LogSet != nil {
		err := v1alpha1.RemoveBucketFinalizer(ctx.Context, ctx.Client, dn.Deps.LogSet.ObjectMeta, utils.MakeHashFinalizer(v1alpha1.BucketDNFinalizerPrefix, dn))
		if err != nil {
			return false, err
		}
	}
	return true, nil
}

func (d *Actor) Create(ctx *recon.Context[*v1alpha1.DNSet]) error {
	ctx.Log.Info("create dn set")
	dn := ctx.Obj

	hSvc := buildHeadlessSvc(dn)
	dnSet := buildDNSet(dn)
	syncReplicas(dn, dnSet)
	syncPodMeta(dn, dnSet)
	syncPodSpec(dn, dnSet, ctx.Dep.Deps.LogSet.Spec.SharedStorage)
	syncPersistentVolumeClaim(dn, dnSet)

	configMap, configSuffix, err := buildDNSetConfigMap(dn, ctx.Dep.Deps.LogSet)
	if err != nil {
		return err
	}
	dnSet.Spec.Template.Annotations[common.ConfigSuffixAnno] = configSuffix
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
		return errors.WrapPrefix(err, "create dn service", 0)
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

func (d *Actor) syncMetricService(ctx *recon.Context[*v1alpha1.DNSet]) error {
	dn := ctx.Obj
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ctx.Obj.Namespace,
			Name:      ctx.Obj.Name + "-dn-metric",
			Labels:    common.SubResourceLabels(dn),
		},
		Spec: corev1.ServiceSpec{
			Selector: common.SubResourceLabels(dn),
		},
	}
	return recon.CreateOwnedOrUpdate(ctx, svc, func() error {
		svc.Spec.Ports = []corev1.ServicePort{{
			Name: "metric",
			Port: int32(common.MetricsPort),
		}}
		if dn.Spec.PromDiscoveredByService() {
			svc.Annotations = map[string]string{
				common.PrometheusScrapeAnno: "true",
				common.PrometheusPortAnno:   strconv.Itoa(common.MetricsPort),
			}
		} else {
			delete(svc.Annotations, common.PrometheusScrapeAnno)
		}
		return nil
	})
}

func (d *Actor) Reconcile(mgr manager.Manager) error {
	err := recon.Setup[*v1alpha1.DNSet](&v1alpha1.DNSet{}, "dnset", mgr, d,
		recon.WithBuildFn(func(b *builder.Builder) {
			b.Owns(&kruise.StatefulSet{}).
				Owns(&corev1.Service{})
			b.WithEventFilter(predicate.NewPredicateFuncs(func(obj client.Object) bool {
				set, ok := obj.(*v1alpha1.DNSet)
				if ok && !set.Spec.GetOperatorVersion().Equals(v1alpha1.LatestOpVersion) {
					// only filter DNSet
					return false
				}
				return true
			}))
		}))
	if err != nil {
		return err
	}

	return nil
}
