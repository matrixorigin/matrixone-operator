package logset

import (
	"k8s.io/apimachinery/pkg/api/equality"
	"time"

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
	BootstrapAnnoKey = "logset.matrixorigin.io/bootstrap"

	IDRangeStart int = 131072
	IDRangeEnd   int = 262144

	ReasonNoEnoughReadyStores = "NoEnoughReadyStores"
)

const (
	// TODO(aylei): should be configurable
	storeDownTimeout = 5 * time.Minute
)

var _ recon.Actor[*v1alpha1.LogSet] = &LogSetActor{}

type LogSetActor struct{}

type WithResources struct {
	*LogSetActor
	sts *kruisev1.StatefulSet
}

func (r *LogSetActor) with(sts *kruisev1.StatefulSet) *WithResources {
	return &WithResources{LogSetActor: r, sts: sts}
}

func (r *LogSetActor) Observe(ctx *recon.Context[*v1alpha1.LogSet]) (recon.Action[*v1alpha1.LogSet], error) {
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
			Type:   v1alpha1.ConditionTypeReady,
			Status: metav1.ConditionTrue,
		})
	} else {
		ls.Status.SetCondition(metav1.Condition{
			Type:   v1alpha1.ConditionTypeReady,
			Status: metav1.ConditionFalse,
			Reason: ReasonNoEnoughReadyStores,
		})
	}
	ls.Status.Discovery = &v1alpha1.LogSetDiscovery{
		Port:    LogServicePort,
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
	return nil, nil
}

func (r *LogSetActor) Create(ctx *recon.Context[*v1alpha1.LogSet]) error {
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
	syncPodSpec(ls, sts)
	syncPersistentVolumeClaim(ls, sts)
	discovery := buildDiscoveryService(ls)

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
	return ctx.Patch(r.sts, func() error {
		syncReplicas(ctx.Obj, r.sts)
		return nil
	})
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
	// repair one at a time
	ordinal, err := util.PodOrdinal(toRepair[0].PodName)
	if err != nil {
		return errors.Wrapf(err, "error parse ordinal from pod name %s", toRepair[0].PodName)
	}
	r.sts.Spec.ReserveOrdinals = util.Upsert(r.sts.Spec.ReserveOrdinals, ordinal)
	return nil
}

// Update rolling-update the log set pods to match the desired state
// TODO(aylei): should logset controller take care of graceful rolling?
func (r *WithResources) Update(ctx *recon.Context[*v1alpha1.LogSet]) error {
	return ctx.Update(r.sts)
}

func (r *LogSetActor) Finalize(ctx *recon.Context[*v1alpha1.LogSet]) (bool, error) {
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

func syncPods(ctx *recon.Context[*v1alpha1.LogSet], sts *kruisev1.StatefulSet) error {
	cm, err := buildConfigMap(ctx.Obj)
	if err != nil {
		return err
	}
	syncPodMeta(ctx.Obj, sts)
	syncPodSpec(ctx.Obj, sts)
	return common.SyncConfigMap(ctx, &sts.Spec.Template.Spec, cm)
}
