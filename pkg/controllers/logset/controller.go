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
package logset

import (
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"time"

	"k8s.io/apimachinery/pkg/api/equality"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	"github.com/matrixorigin/matrixone-operator/pkg/controllers/common"
	recon "github.com/matrixorigin/matrixone-operator/runtime/pkg/reconciler"
	"github.com/matrixorigin/matrixone-operator/runtime/pkg/util"
	kruisev1 "github.com/openkruise/kruise-api/apps/v1beta1"
	"github.com/pkg/errors"
	"github.com/samber/lo"
	"go.uber.org/multierr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// TODO(aylei): should be configurable
	storeDownTimeout = 5 * time.Minute
	reSyncAfter      = 15 * time.Second

	// failoverDeletionFinalizer hold the pod that chosen to be deleted until human confirmation
	failoverDeletionFinalizer = "matrixorigin.io/confirm-deletion"
)

var _ recon.Actor[*v1alpha1.LogSet] = &Actor{}

type Actor struct{}

type WithResources struct {
	*Actor
	sts *kruisev1.StatefulSet
}

func (r *Actor) with(sts *kruisev1.StatefulSet) *WithResources {
	return &WithResources{Actor: r, sts: sts}
}

func (r *Actor) Observe(ctx *recon.Context[*v1alpha1.LogSet]) (recon.Action[*v1alpha1.LogSet], error) {
	ls := ctx.Obj

	// get subresources
	discoverySvc := &corev1.Service{}
	err, foundDiscovery := util.IsFound(ctx.Get(client.ObjectKey{Namespace: ls.Namespace, Name: discoverySvcName(ls)}, discoverySvc))
	if err != nil {
		return nil, errors.Wrap(err, "get HAKeeper discovery service")
	}
	sts := &kruisev1.StatefulSet{}
	err, foundSts := util.IsFound(ctx.Get(client.ObjectKey{Namespace: ls.Namespace, Name: stsName(ls)}, sts))
	if err != nil {
		return nil, errors.Wrap(err, "get logservice statefulset")
	}
	if !foundDiscovery || !foundSts {
		return r.Create, nil
	}

	// calculate status
	podList := &corev1.PodList{}
	err = ctx.List(podList, client.InNamespace(ls.Namespace),
		client.MatchingLabels(common.SubResourceLabels(ls)))
	if err != nil {
		return nil, errors.Wrap(err, "list logservice pods")
	}
	collectStoreStatus(ls, podList.Items)

	if len(ls.Status.AvailableStores) >= int(ls.Spec.Replicas) {
		ls.Status.SetCondition(metav1.Condition{
			Type:   recon.ConditionTypeReady,
			Status: metav1.ConditionTrue,
		})
	} else {
		ls.Status.SetCondition(metav1.Condition{
			Type:   recon.ConditionTypeReady,
			Status: metav1.ConditionFalse,
			Reason: common.ReasonNotEnoughReadyStores,
		})
	}
	ls.Status.Discovery = &v1alpha1.LogSetDiscovery{
		Port:    logServicePort,
		Address: discoverySvcAddress(ls),
	}

	switch {
	case len(ls.StoresFailedFor(storeDownTimeout)) > 0:
		return r.with(sts).Repair, nil
	case ls.Spec.Replicas != *sts.Spec.Replicas:
		return r.with(sts).Scale, nil
	}
	origin := sts.DeepCopy()
	if err := syncPods(ctx, sts); err != nil {
		return nil, err
	}
	if !equality.Semantic.DeepEqual(origin, sts) {
		return r.with(sts).Update, nil
	}
	if recon.IsReady(&ls.Status.ConditionalStatus) && len(ls.Status.FailedStores) == 0 {
		return nil, nil
	}
	return nil, recon.ErrReSync("logset is not ready or has unready members", reSyncAfter)
}

func (r *Actor) Create(ctx *recon.Context[*v1alpha1.LogSet]) error {
	ctx.Log.Info("create logset")
	ls := ctx.Obj

	// build resources required by a logset
	bc, err := buildBootstrapConfig(ctx)
	if err != nil {
		return err
	}
	svc := buildHeadlessSvc(ls)
	sts := buildStatefulSet(ls, svc)
	syncReplicas(ls, sts)
	syncPodMeta(ls, sts)
	syncPodSpec(ls, &sts.Spec.Template.Spec)
	syncPersistentVolumeClaim(ls, sts)
	discovery := buildDiscoveryService(ls)
	gconfig, err := buildGossipSeedsConfigMap(ls, sts)
	if err != nil {
		return err
	}
	// sync the config
	cm, err := buildConfigMap(ls)
	if err != nil {
		return err
	}
	if err := common.SyncConfigMap(ctx, &sts.Spec.Template.Spec, cm); err != nil {
		return err
	}

	// create all resources
	err = lo.Reduce[client.Object, error]([]client.Object{
		bc,
		gconfig,
		svc,
		sts,
		discovery,
	}, func(errs error, o client.Object, _ int) error {
		err := ctx.CreateOwned(o)
		// ignore already exist during creation, updating of the underlying resources should be
		// done carefully in other Actions since updating might be destructive
		return multierr.Append(errs, util.Ignore(apierrors.IsAlreadyExists, err))
	}, nil)
	if err != nil {
		return errors.Wrap(err, "create")
	}
	return nil
}

// Scale scale-out/in the log set pods to match the desired state
// TODO(aylei): special treatment for scale-in
func (r *WithResources) Scale(ctx *recon.Context[*v1alpha1.LogSet]) error {
	err := ctx.Patch(r.sts, func() error {
		syncReplicas(ctx.Obj, r.sts)
		return nil
	})
	if err != nil {
		return err
	}
	// also update gossip config after scale
	return updateGossipConfig(ctx, r.sts)
}

// Repair repairs failed log set pods to match the desired state
func (r *WithResources) Repair(ctx *recon.Context[*v1alpha1.LogSet]) error {
	toRepair := ctx.Obj.StoresFailedFor(storeDownTimeout)
	if len(toRepair) == 0 {
		return nil
	}
	if len(toRepair) >= (*ctx.Obj.Spec.InitialConfig.LogShardReplicas)/2 {
		ctx.Log.Info("majority failure might happen, wait for human intervention")
		return nil
	}
	candidate := toRepair[0]
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ctx.Obj.Namespace,
			Name:      candidate.PodName,
		},
	}
	// FIXME: we might have to make this behavior configurable
	err := ctx.Patch(pod, func() error {
		controllerutil.AddFinalizer(pod, failoverDeletionFinalizer)
		// mark the pod as need external action and cleanup all old labels
		pod.Labels = map[string]string{
			common.ActionRequiredLabelKey: common.ActionRequiredLabelValue,
		}
		return nil
	})
	if err != nil {
		return errors.Wrap(err, "cannot orphan the victim pod")
	}
	// repair one at a time
	ordinal, err := util.PodOrdinal(candidate.PodName)
	if err != nil {
		return errors.Wrapf(err, "error parse ordinal from pod name %s", toRepair[0].PodName)
	}
	r.sts.Spec.ReserveOrdinals = util.Upsert(r.sts.Spec.ReserveOrdinals, ordinal)
	if err := ctx.Update(r.sts); err != nil {
		return err
	}
	// also update gossip config after failover
	return updateGossipConfig(ctx, r.sts)
}

// Update rolling-update the log set pods to match the desired state
// TODO(aylei): should logset controller take care of graceful rolling?
func (r *WithResources) Update(ctx *recon.Context[*v1alpha1.LogSet]) error {
	return ctx.Update(r.sts)
}

func (r *Actor) Finalize(ctx *recon.Context[*v1alpha1.LogSet]) (bool, error) {
	ls := ctx.Obj
	// TODO(aylei): we may encode the created resources in etcd so that we don't have
	// to maintain a hardcoded list
	objs := []client.Object{&corev1.Service{ObjectMeta: metav1.ObjectMeta{
		Name: headlessSvcName(ls),
	}}, &kruisev1.StatefulSet{ObjectMeta: metav1.ObjectMeta{
		Name: stsName(ls),
	}}, &corev1.Service{ObjectMeta: metav1.ObjectMeta{
		Name: discoverySvcName(ls),
	}}}
	for _, obj := range objs {
		obj.SetNamespace(ls.Namespace)
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

func updateGossipConfig(ctx *recon.Context[*v1alpha1.LogSet], sts *kruisev1.StatefulSet) error {
	gossipCM, err := buildGossipSeedsConfigMap(ctx.Obj, sts)
	if err != nil {
		return err
	}
	o := gossipCM.DeepCopy()
	return recon.CreateOwnedOrUpdate(ctx, o, func() error {
		o.Data = gossipCM.Data
		return nil
	})
}

func syncPods(ctx *recon.Context[*v1alpha1.LogSet], sts *kruisev1.StatefulSet) error {
	cm, err := buildConfigMap(ctx.Obj)
	if err != nil {
		return err
	}
	syncPodMeta(ctx.Obj, sts)
	syncPodSpec(ctx.Obj, &sts.Spec.Template.Spec)
	return common.SyncConfigMap(ctx, &sts.Spec.Template.Spec, cm)
}

func (r *Actor) Reconcile(mgr manager.Manager) error {
	return recon.Setup[*v1alpha1.LogSet](&v1alpha1.LogSet{}, "logset", mgr, r,
		recon.WithBuildFn(func(b *builder.Builder) {
			// watch all changes on the owned statefulset since we need perform failover if there is a pod failure
			b.Owns(&kruisev1.StatefulSet{}).
				Owns(&corev1.Service{})
		}))
}
