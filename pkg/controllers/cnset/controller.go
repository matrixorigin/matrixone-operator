// Copyright 2025 Matrix Origin
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
	"time"

	"github.com/matrixorigin/matrixone-operator/api/features"
	"github.com/matrixorigin/matrixone-operator/pkg/utils"
	"github.com/openkruise/kruise-api/apps/pub"
	kruisev1alpha1 "github.com/openkruise/kruise-api/apps/v1alpha1"
	"k8s.io/utils/pointer"

	"github.com/go-errors/errors"
	recon "github.com/matrixorigin/controller-runtime/pkg/reconciler"
	"github.com/matrixorigin/controller-runtime/pkg/util"
	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	"github.com/matrixorigin/matrixone-operator/pkg/controllers/common"
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
	cnReadySeconds = 30

	reSyncAfter = 10 * time.Second

	cloneSetForceSpecifiedDelete = "apps.kruise.io/force-specified-delete"
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

	ctx.Log.Info("observe cnset", "name", cn.Name, "operatorVersion", cn.Spec.GetOperatorVersion())
	cs := &kruisev1alpha1.CloneSet{}
	err, foundCs := util.IsFound(ctx.Get(client.ObjectKey{Namespace: cn.Namespace, Name: setName(cn)}, cs))
	if err != nil {
		return nil, errors.WrapPrefix(err, "get cn clonset", 0)
	}
	if !foundCs {
		return c.Create, nil
	}

	if features.DefaultFeatureGate.Enabled(features.S3Reclaim) && cn.Deps.LogSet != nil {
		err = v1alpha1.AddBucketFinalizer(ctx.Context, ctx.Client, cn.Deps.LogSet.ObjectMeta, utils.MakeHashFinalizer(v1alpha1.BucketCNFinalizerPrefix, cn))
		if err != nil {
			return nil, errors.WrapPrefix(err, "add bucket finalizer", 0)
		}
	}

	svc := buildSvc(cn)
	if err := recon.CreateOwnedOrUpdate(ctx, svc, func() error {
		syncService(cn, svc)
		return nil
	}); err != nil {
		return nil, errors.WrapPrefix(err, "sync service", 0)
	}

	// diff desired cloneset and determine whether should an update be invoked
	origin := cs.DeepCopy()
	if err := syncCloneSet(ctx, cs); err != nil {
		return nil, err
	}
	if err = ctx.Update(cs, client.DryRunAll); err != nil {
		return nil, errors.WrapPrefix(err, "dry run update cnset", 0)
	}
	if !equality.Semantic.DeepEqual(origin, cs) {
		if cn.Spec.PauseUpdate {
			ctx.Log.Info("CNSet does not reach desired state, but update is paused, only in-place update will be applied")
			inplaceMutated := origin.DeepCopy()
			inplaceMutated.Spec.ScaleStrategy = cs.Spec.ScaleStrategy
			inplaceMutated.Spec.UpdateStrategy = cs.Spec.UpdateStrategy
			inplaceMutated.Spec.Lifecycle = cs.Spec.Lifecycle
			inplaceMutated.Spec.Template.ObjectMeta.Labels = cs.Spec.Template.ObjectMeta.Labels
			inplaceMutated.Spec.Template.ObjectMeta.Annotations = cs.Spec.Template.ObjectMeta.Annotations

			inplaceMutated.Labels = cs.Labels
			inplaceMutated.Annotations = cs.Annotations
			if !equality.Semantic.DeepEqual(inplaceMutated, origin) {
				return c.with(inplaceMutated).Update, nil
			}
		} else {
			return c.with(cs).Update, nil
		}
	}
	// calculate status
	var stores []v1alpha1.CNStore
	podList := &corev1.PodList{}
	err = ctx.List(podList, client.InNamespace(cn.Namespace),
		client.MatchingLabels(common.SubResourceLabels(cn)))
	if err != nil {
		return nil, errors.WrapPrefix(err, "list cn pods", 0)
	}
	livePods := map[string]bool{}
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
		livePods[pod.Name] = true
	}
	cn.Status.Stores = stores
	cn.Status.Replicas = cs.Status.Replicas
	cn.Status.ReadyReplicas = cs.Status.ReadyReplicas
	cn.Status.LabelSelector = cs.Status.LabelSelector
	// sync status from cloneset
	if cs.Status.ReadyReplicas >= cn.Spec.Replicas {
		setReady(cn)
	} else {
		setNotReady(cn)
	}

	if features.DefaultFeatureGate.Enabled(features.S3Reclaim) && cn.Deps.LogSet != nil {
		if cs.Status.ReadyReplicas > 0 {
			err = v1alpha1.SyncBucketEverRunningAnn(ctx.Context, ctx.Client, cn.Deps.LogSet.ObjectMeta)
			if err != nil {
				return nil, errors.WrapPrefix(err, "set bucket ever running ann", 0)
			}
		}
	}
	if cn.Spec.Replicas != *cs.Spec.Replicas ||
		!equality.Semantic.DeepEqual(cn.Spec.PodsToDelete, cs.Spec.ScaleStrategy.PodsToDelete) {
		return c.with(cs).Scale, nil
	}

	if cn.Spec.CacheVolume != nil {
		if err := common.SyncCloneSetVolumeSize(ctx, cn, cn.Spec.CacheVolume.Size, cs); err != nil {
			return nil, errors.WrapPrefix(err, "sync volume size", 0)
		}
	}

	if recon.IsReady(&cn.Status.ConditionalStatus) {
		cn.Status.Host = fmt.Sprintf("%s.%s", svc.Name, svc.Namespace)
		cn.Status.Port = CNSQLPort
		if cs.Status.UpdatedReadyReplicas >= cn.Spec.Replicas {
			// CN ready and updated, reconciliation complete
			return nil, nil
		}
	}

	return nil, recon.ErrReSync("cnset is not ready or synced", reSyncAfter)
}

func (c *WithResources) Scale(ctx *recon.Context[*v1alpha1.CNSet]) error {
	return ctx.Patch(c.cs, func() error {
		scaleSet(ctx.Obj, c.cs)
		return nil
	})
}

func (c *WithResources) Update(ctx *recon.Context[*v1alpha1.CNSet]) error {
	return ctx.Update(c.cs)
}

func (c *Actor) Finalize(ctx *recon.Context[*v1alpha1.CNSet]) (bool, error) {
	cn := ctx.Obj

	if cn.Spec.GetTerminationPolicy() == v1alpha1.CNSetTerminationPolicyDrain {
		if done, err := waitAllCNDrained(ctx); err != nil || !done {
			return false, err
		}
	}

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
		err := v1alpha1.RemoveBucketFinalizer(ctx.Context, ctx.Client, cn.Deps.LogSet.ObjectMeta, utils.MakeHashFinalizer(v1alpha1.BucketCNFinalizerPrefix, cn))
		if err != nil {
			return false, err
		}
	}
	return true, nil
}

func waitAllCNDrained(ctx *recon.Context[*v1alpha1.CNSet]) (bool, error) {
	cn := ctx.Obj
	// scale CNSet to zero and then delete the CNSet to ensure gracefulness
	cs := &kruisev1alpha1.CloneSet{ObjectMeta: metav1.ObjectMeta{
		Namespace: cn.Namespace,
		Name:      cn.Name,
	}}
	if err := ctx.Get(client.ObjectKeyFromObject(cs), cs); err != nil {
		if apierrors.IsNotFound(err) {
			// cloneset had been deleted, skip
			return true, nil
		}
		return false, errors.WrapPrefix(err, "error get cloneset", 0)
	}
	if err := ctx.Patch(cs, func() error {
		cs.Spec.Replicas = pointer.Int32(0)
		return nil
	}); err != nil {
		return false, errors.WrapPrefix(err, "error scale cloneset to 0", 0)
	}
	if cs.Status.Replicas > 0 {
		ctx.Log.V(4).Info("waiting for CNSet to be scaled to 0", "replicas", cs.Status.Replicas)
		return false, nil
	}
	return true, nil
}

func (c *Actor) Create(ctx *recon.Context[*v1alpha1.CNSet]) error {
	cn := ctx.Obj

	// headless svc for pod dns resolution
	hSvc := buildHeadlessSvc(cn)
	cnSet := buildCNSet(cn, hSvc)
	svc := buildSvc(cn)
	scaleSet(cn, cnSet)
	if err := syncCloneSet(ctx, cnSet); err != nil {
		return errors.WrapPrefix(err, "sync clone set", 0)
	}
	syncPersistentVolumeClaim(cn, cnSet)

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
		return errors.WrapPrefix(err, "create cn service", 0)
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
	cn := ctx.Obj
	pooling := cn.Spec.PodManagementPolicy != nil && *cn.Spec.PodManagementPolicy == v1alpha1.PodManagementPolicyPooling
	cs.Spec.UpdateStrategy.Type = kruisev1alpha1.InPlaceIfPossibleCloneSetUpdateStrategyType
	if pooling {
		cs.Spec.UpdateStrategy.Type = kruisev1alpha1.InPlaceOnlyCloneSetUpdateStrategyType
	}
	cs.Spec.UpdateStrategy.MaxUnavailable = cn.Spec.UpdateStrategy.MaxUnavailable
	cs.Spec.UpdateStrategy.MaxSurge = cn.Spec.UpdateStrategy.MaxSurge
	cs.Spec.MinReadySeconds = cnReadySeconds

	// scale-out without maxUnavailable limit to avoid unavailable pod abort the fail-over
	cs.Spec.ScaleStrategy.DisablePVCReuse = !cn.Spec.GetReusePVC()
	cs.Spec.ScaleStrategy.MaxUnavailable = nil
	if cs.Spec.Lifecycle == nil {
		cs.Spec.Lifecycle = &pub.Lifecycle{}
	}
	cs.Spec.Lifecycle.PreDelete = &pub.LifecycleHook{
		FinalizersHandler: []string{
			common.CNDrainingFinalizer,
		},
		MarkPodNotReady: true,
	}
	cs.Spec.Lifecycle.InPlaceUpdate = &pub.LifecycleHook{
		FinalizersHandler: []string{
			common.CNDrainingFinalizer,
		},
		// there is a bug the kruise cannot patch pod readiness after in-place update,
		// so we cannot MarkPodNotReady in this case, instead, we mark the pod as not ready
		// through our cn-store readiness-gate.
		MarkPodNotReady: false,
	}

	if err := syncPodMeta(ctx.Obj, cs); err != nil {
		return errors.WrapPrefix(err, "sync pod meta", 0)
	}
	if ctx.Dep != nil {
		syncPodSpec(ctx.Obj, cs, ctx.Dep.Deps.LogSet.Spec.SharedStorage)
	}
	if pooling {
		if cs.Annotations == nil {
			cs.Annotations = map[string]string{}
		}
		cs.Annotations[cloneSetForceSpecifiedDelete] = "y"
		if v1alpha1.GateInplacePoolRollingUpdate.Enabled(cn.Spec.GetOperatorVersion()) {
			if cs.Spec.Template.Annotations == nil {
				cs.Spec.Template.Annotations = map[string]string{}
			}
			cs.Spec.Template.Annotations[v1alpha1.InPlacePoolRollingAnnoKey] = "y"
		}
	}

	cm, configSuffix, err := buildCNSetConfigMap(ctx.Obj, ctx.Dep.Deps.LogSet)
	if err != nil {
		return err
	}
	if v1alpha1.GateInplaceConfigmapUpdate.Enabled(cn.Spec.GetOperatorVersion()) {
		cs.Spec.Template.Annotations[common.ConfigSuffixAnno] = configSuffix
	}
	return common.SyncConfigMap(ctx, &cs.Spec.Template.Spec, cm, cn.Spec.GetOperatorVersion())
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
