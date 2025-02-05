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

package cnstore

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/blang/semver/v4"
	"github.com/go-errors/errors"
	gerrors "github.com/go-errors/errors"
	"github.com/go-logr/logr"
	recon "github.com/matrixorigin/controller-runtime/pkg/reconciler"
	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	"github.com/matrixorigin/matrixone-operator/pkg/controllers/common"
	"github.com/matrixorigin/matrixone-operator/pkg/mocli"
	"github.com/matrixorigin/matrixone-operator/pkg/querycli"
	logpb "github.com/matrixorigin/matrixone/pkg/pb/logservice"
	"github.com/matrixorigin/matrixone/pkg/pb/metadata"
	"github.com/openkruise/kruise-api/apps/pub"
	kruisev1alpha1 "github.com/openkruise/kruise-api/apps/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

const (
	LockRestartSet = "matrixorigin.io/lock-restart"
)

const (
	messageCNCordon             = "CNStoreCordon"
	messageCNPrepareStop        = "CNStorePrepareStop"
	messageCNStoreReady         = "CNStoreReady"
	messageCNStoreNotRegistered = "CNStoreNotRegistered"

	defaultConcurrency = 8

	storeDrainTakesLongDuration = 5 * time.Minute

	diagnosDrainingAnno = "matrixorigin.io/diagnos-draining"
)

const retryInterval = 5 * time.Second
const resyncInterval = 30 * time.Second

type Controller struct {
	clientMgr *mocli.MORPCClientManager
	queryCli  *querycli.Client
}

type withCNSet struct {
	*Controller

	cn *v1alpha1.CNSet
}

func NewController(mgr *mocli.MORPCClientManager, qc *querycli.Client) *Controller {
	return &Controller{clientMgr: mgr, queryCli: qc}
}

var _ recon.Actor[*corev1.Pod] = &Controller{}

// OnDeleted delete CNStore and cleanup finalizer on Pod deletion
func (c *Controller) OnDeleted(ctx *recon.Context[*corev1.Pod]) error {
	pod := ctx.Obj
	uid := v1alpha1.GetCNPodUUID(pod)
	cnSet, err := common.ResolveCNSet(ctx, pod)
	if err == nil {
		wc := &withCNSet{
			Controller: c,
			cn:         cnSet,
		}
		// clean up CN store if any, note that OnDeleted() and the termination of Pod containers
		// are simultaneous, so the cleanup below is merely a best-effort attempt in extraordinary case, e.g.
		// the Pod is deleted forcefully by a human operator. Normal cleanup must be done before we enter OnDeleted()
		// to avoid zombie CN in HAKeeper.
		ctx.Log.Info("call HAKeeper to remove CN store", "uuid", uid)
		err = wc.withMOClientSet(ctx, func(timeout context.Context, h *mocli.ClientSet) error {
			return h.Client.DeleteCNStore(timeout, logpb.DeleteCNStore{
				StoreID: uid,
			})
		})
		if err != nil {
			return errors.WrapPrefix(err, "error remove CN store", 0)
		}
	} else {
		ctx.Log.Info("error resolve CNSet of the deleted CN, skip", "error", err.Error())
	}
	if err := ctx.Patch(pod, func() error {
		controllerutil.RemoveFinalizer(pod, common.CNDrainingFinalizer)
		return nil
	}); err != nil {
		return err
	}
	return nil
}

// OnPreparingUpdate perform actions that should be done on CN preparing stop
func (c *withCNSet) OnPreparingUpdate(ctx *recon.Context[*corev1.Pod]) error {
	// if update is paused, then the preparing must be triggered by a change
	// that won't restart the application container, safely bypass
	if c.cn.Spec.PauseUpdate {
		ctx.Log.Info("skip draining CN store, no restart required", "CN", client.ObjectKeyFromObject(ctx.Obj))
		return c.completeDraining(ctx)
	}
	// TODO: should diff with cloneset spec
	// if pod image is not going to be updated, skip draining
	// NB: change envFrom(labels/annotations) will restart container in-place, but we cannot
	// distinguish such case now, CN will be restarted without draining if we introduce envFrom
	// mutation in other modules. E2Es are needed to guard such issue.
	//if !common.NeedUpdateImage(ctx.Obj) {
	//	ctx.Log.Info("skip draining CN store, no image update", "CN", client.ObjectKeyFromObject(ctx.Obj))
	//	return c.completeDraining(ctx)
	//}
	return c.OnPreparingStop(ctx)
}

// OnPreparingStop drains CN connections
func (c *withCNSet) OnPreparingStop(ctx *recon.Context[*corev1.Pod]) error {
	pod := ctx.Obj
	uid := v1alpha1.GetCNPodUUID(ctx.Obj)
	if pod.Annotations == nil {
		pod.Annotations = map[string]string{}
	}

	if err := c.patchCNReadiness(ctx, corev1.ConditionFalse, messageCNPrepareStop); err != nil {
		return errors.WrapPrefix(err, "patch pod readiness", 0)
	}
	// store draining disabled, cleanup finalizers and skip
	sc := c.cn.Spec.ScalingConfig
	if !sc.GetStoreDrainEnabled() {
		return c.completeDraining(ctx)
	}

	// start draining
	var startTime time.Time
	startTimeStr, ok := pod.Annotations[v1alpha1.StoreDrainingStartAnno]
	if ok {
		parsed, err := time.Parse(time.RFC3339, startTimeStr)
		if err != nil {
			return errors.Wrap(err, 0)
		}
		startTime = parsed
	} else {
		startTime = time.Now()
		if err := ctx.Patch(pod, func() error {
			pod.Annotations[v1alpha1.StoreDrainingStartAnno] = startTime.Format(time.RFC3339)
			return nil
		}); err != nil {
			return errors.WrapPrefix(err, "error patching store draining start time", 0)
		}
	}
	// check whether timeout is reached
	if time.Since(startTime) > sc.GetStoreDrainTimeout() {
		ctx.Log.Info("store draining timeout, force delete CN", "uuid", uid)
		return c.completeDraining(ctx)
	}

	var connAndShardMigrated, lockMigrated bool
	err := c.withMOClientSet(ctx, func(timeout context.Context, h *mocli.ClientSet) error {
		var err error
		connAndShardMigrated, err = c.handleConnectionDraining(ctx, uid, timeout, h)
		if err != nil {
			return err
		}
		if time.Since(startTime) < sc.GetMinDelayDuration() {
			return recon.ErrReSync("wait min-delay for CN draining state get propagated", sc.GetMinDelayDuration())
		}
		if !connAndShardMigrated {
			// lock migration should be done after connection get migrated
			return nil
		}
		lockMigrated, err = c.handleLockMigration(ctx, uid, timeout, h)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		// if the CN does not exist in HAKeeper, shortcut to complete draining
		if strings.Contains(err.Error(), "does not exist") {
			return c.completeDraining(ctx)
		}
		return err
	}
	if connAndShardMigrated && lockMigrated {
		return c.completeDraining(ctx)
	}
	if time.Since(startTime) > storeDrainTakesLongDuration {
		c.diagnosisDraining(ctx, uid)
	}
	return recon.ErrReSync("wait for CN store draining", retryInterval)
}

func (c *Controller) diagnosisDraining(ctx *recon.Context[*corev1.Pod], uid string) {
	ctx.Log.Info("store draining takes too long, collect diagnostic info", "uuid", uid)
	pod := ctx.Obj
	if err := ctx.Patch(pod, func() error {
		if pod.Annotations == nil {
			pod.Annotations = map[string]string{}
		}
		pod.Annotations[diagnosDrainingAnno] = "y"
		return nil
	}); err != nil {
		ctx.Log.Error(err, "error patching diagnos draining anno")
	}
}

func (c *withCNSet) handleConnectionDraining(ctx *recon.Context[*corev1.Pod], uid string, timeout context.Context, h *mocli.ClientSet) (bool, error) {
	pod := ctx.Obj
	ctx.Log.Info("set CN store draining", "uuid", uid)
	if err := h.Client.PatchCNStore(timeout, logpb.CNStateLabel{
		UUID:  uid,
		State: metadata.WorkState_Draining,
	}); err != nil {
		return false, errors.WrapPrefix(err, "error set CN state draining", 0)
	}
	storeConnection, err := common.GetStoreScore(pod)
	if err != nil {
		return false, errors.WrapPrefix(err, "error get store connection count", 0)
	}

	if !storeConnection.IsSafeToReclaim() {
		ctx.Log.Info("wait store connection to be drained", "uuid", uid, "connections", storeConnection.SessionCount, "pipelines", storeConnection.PipelineCount, "replicas", storeConnection.ReplicaCount)
	}
	return storeConnection.IsSafeToReclaim(), nil
}

func (c *withCNSet) handleLockMigration(ctx *recon.Context[*corev1.Pod], uid string, timeout context.Context, h *mocli.ClientSet) (bool, error) {
	pod := ctx.Obj
	handleLockDone := true
	if v1alpha1.HasMOFeature(common.GetSemanticVersion(&pod.ObjectMeta), v1alpha1.MOFeatureLockMigration) {
		ok, err := h.LockServiceClient.CanRestartCN(timeout, uid)
		if err != nil {
			return false, err
		}
		if !ok {
			ctx.Log.Info("cannot restart CN now, check reason", "UID", uid)
			remainTxns, err := h.LockServiceClient.RemainTxnCount(timeout, uid)
			if err != nil {
				ctx.Log.Error(err, "cannot get remaining transactions")
			} else {
				ctx.Log.Info("CN has remaining transactions, cannot restart now", "UID", uid, "remainTxns", remainTxns)
			}
			handleLockDone = false
			_, lockRestartSet := pod.Annotations[LockRestartSet]
			if !lockRestartSet {
				ctx.Log.Info("set lock-service restarting", "cn", uid, "pod", pod.Name)
				ok, err := h.LockServiceClient.SetRestartCN(timeout, uid)
				if err != nil {
					return false, err
				}
				if !ok {
					return false, errors.New("error set restart CN")
				}
				if err := ctx.Patch(pod, func() error {
					if pod.Annotations == nil {
						pod.Annotations = map[string]string{}
					}
					pod.Annotations[LockRestartSet] = "true"
					return nil
				}); err != nil {
					return false, errors.Wrap(err, 0)
				}
			}
		} else {
			ctx.Log.Info("lock-service migrated, can restart CN now", "UUID", uid)
		}
	}
	return handleLockDone, nil
}

func (c *withCNSet) completeDraining(ctx *recon.Context[*corev1.Pod]) error {
	if err := ctx.Patch(ctx.Obj, func() error {
		controllerutil.RemoveFinalizer(ctx.Obj, common.CNDrainingFinalizer)
		delete(ctx.Obj.Annotations, v1alpha1.StoreDrainingStartAnno)
		delete(ctx.Obj.Annotations, LockRestartSet)
		delete(ctx.Obj.Annotations, diagnosDrainingAnno)
		return nil
	}); err != nil {
		return errors.WrapPrefix(err, "error removing CN draining finalizer", 0)
	}
	if _, ok := ctx.Obj.Labels[v1alpha1.DirectPodLabel]; ok {
		if err := ctx.Delete(ctx.Obj); err != nil {
			return errors.Wrap(err, 0)
		}
	}
	return nil
}

// OnNormal ensure CNStore labels and transit CN store to UP state
func (c *withCNSet) OnNormal(ctx *recon.Context[*corev1.Pod]) error {
	pod := ctx.Obj

	// ensure finalizers
	if err := ctx.Patch(pod, func() error {
		controllerutil.AddFinalizer(ctx.Obj, common.CNDrainingFinalizer)
		return nil
	}); err != nil {
		return errors.WrapPrefix(err, "ensure finalizers for CNStore Pod", 0)
	}
	// remove draining start time in case we regret formal deletion decision
	if pod.Annotations == nil {
		pod.Annotations = map[string]string{}
	}
	if err := ctx.Patch(pod, func() error {
		delete(pod.Annotations, v1alpha1.StoreDrainingStartAnno)
		return nil
	}); err != nil {
		return errors.WrapPrefix(err, "removing CN draining start time", 0)
	}

	if _, ok := ctx.Obj.Labels[v1alpha1.DirectPodLabel]; ok {
		// GC idle direct-pod
		if ctx.Obj.Labels[v1alpha1.CNPodPhaseLabel] == v1alpha1.CNPodPhaseIdle || ctx.Obj.Labels[kruisev1alpha1.SpecifiedDeleteKey] != "" {
			if err := ctx.Patch(ctx.Obj, func() error {
				ctx.Obj.Labels[pub.LifecycleStateKey] = string(pub.LifecycleStatePreparingDelete)
				return nil
			}); err != nil {
				return errors.Wrap(err, 0)
			}
			return recon.ErrReSync("direct pod is idle, transfer to preparing delete")
		}
	}
	// policy based reconciliation
	if v1alpha1.IsPoolingPolicy(ctx.Obj) {
		return c.poolingCNReconcile(ctx)
	}
	return c.defaultCNNormalReconcile(ctx)
}

func (c *withCNSet) defaultCNNormalReconcile(ctx *recon.Context[*corev1.Pod]) error {
	pod := ctx.Obj
	uid := v1alpha1.GetCNPodUUID(pod)

	// sync CN labels for store and mark store as UP state
	var cnLabels []v1alpha1.CNLabel
	labelStr, ok := pod.Annotations[common.CNLabelAnnotation]
	if ok {
		err := json.Unmarshal([]byte(labelStr), &cnLabels)
		if err != nil {
			return errors.WrapPrefix(err, "unmarshal CNLabels", 0)
		}
	}

	var err error
	if c.cn.Spec.ScalingConfig.GetStoreDrainEnabled() {
		err = c.withMOClientSet(ctx, func(timeout context.Context, h *mocli.ClientSet) error {
			return h.Client.PatchCNStore(timeout, logpb.CNStateLabel{
				UUID:   uid,
				State:  metadata.WorkState_Working,
				Labels: common.ToStoreLabels(cnLabels),
			})
		})
	} else {
		err = c.withMOClientSet(ctx, func(timeout context.Context, h *mocli.ClientSet) error {
			return h.Client.UpdateCNLabel(timeout, logpb.CNStoreLabel{
				UUID:   uid,
				Labels: common.ToStoreLabels(cnLabels),
			})
		})
	}
	if err != nil {
		ctx.Log.Error(err, "update CN failed", "uuid", uid)
		return recon.ErrReSync("update cn failed", retryInterval)
	}
	ctx.Log.V(4).Info("successfully set CN working")

	return c.patchCNReadiness(ctx, corev1.ConditionTrue, messageCNStoreReady)
}

func (c *withCNSet) patchCNReadiness(ctx *recon.Context[*corev1.Pod], newC corev1.ConditionStatus, reason string) error {
	pod := ctx.Obj
	if err := ctx.PatchStatus(pod, func() error {
		cond := common.GetReadinessCondition(pod, common.CNStoreReadiness)
		if cond == nil {
			pod.Status.Conditions = append(pod.Status.Conditions, common.NewCNReadinessCondition(newC, reason))
		} else {
			if cond.Status != newC {
				cond.Status = newC
				cond.LastTransitionTime = metav1.Now()
			}
			cond.Message = reason
		}
		c.setCNState(pod, v1alpha1.CNStoreStateUp)
		return nil
	}); err != nil {
		return errors.WrapPrefix(err, "patch pod readiness", 0)
	}
	return nil
}

func (c *withCNSet) OnCordon(ctx *recon.Context[*corev1.Pod]) error {
	pod := ctx.Obj
	uid := v1alpha1.GetCNPodUUID(pod)
	ctx.Log.Info("call HAKeeper to cordon CN store", "uuid", uid)
	err := c.withMOClientSet(ctx, func(timeout context.Context, h *mocli.ClientSet) error {
		return h.Client.PatchCNStore(timeout, logpb.CNStateLabel{
			UUID:  uid,
			State: metadata.WorkState_Draining,
		})
	})
	if err != nil {
		return errors.WrapPrefix(err, "error cordon cn store", 0)
	}
	// set pod unready to unregister the pod from internal service
	return c.patchCNReadiness(ctx, corev1.ConditionFalse, messageCNCordon)
}

func (c *Controller) observe(ctx *recon.Context[*corev1.Pod]) error {
	pod := ctx.Obj

	// 1. process delete
	if pod.DeletionTimestamp != nil {
		return c.OnDeleted(ctx)
	}

	// 2. resolve CNSet
	cnSet, err := common.ResolveCNSet(ctx, pod)
	if err != nil {
		return errors.WrapPrefix(err, "error resolve CNSet", 0)
	}
	wc := &withCNSet{
		Controller: c,
		cn:         cnSet,
	}

	// 3. sync stats, including connections and deletion cost
	if err := wc.syncStats(ctx); err != nil {
		ctx.Log.Info("error sync stats", "error", err.Error())
		// sync stats should not block state sync, continue
	}

	// 4. optionally, store is asked to be cordoned
	if _, ok := pod.Annotations[v1alpha1.StoreCordonAnno]; ok {
		return wc.OnCordon(ctx)
	}

	lifecycleState := pod.Labels[pub.LifecycleStateKey]
	if lifecycleState == string(pub.LifecycleStatePreparingUpdate) {
		return wc.OnPreparingUpdate(ctx)
	} else if lifecycleState == string(pub.LifecycleStatePreparingDelete) {
		return wc.OnPreparingStop(ctx)
	}

	if err := wc.OnNormal(ctx); err != nil {
		return err
	}
	// trigger next reconciliation later to refresh the stats
	// TODO(aylei): better stats handling
	return recon.ErrReSync("resync", resyncInterval)
}

type connectionDiagnosis struct {
	Logger  logr.Logger
	Enabled bool
}

func (c *withCNSet) syncStats(ctx *recon.Context[*corev1.Pod]) error {
	pod := ctx.Obj

	startedTime := common.GetCNStartedTime(pod)
	if startedTime == nil {
		return errors.New("CN not started")
	}
	sc := &common.StoreScore{}
	previous, err := common.GetStoreScore(pod)
	if err == nil {
		sc = previous
	}
	if sc.StartedTime == nil || !sc.StartedTime.Equal(*startedTime) {
		// clean previously recorded score and update startTime if CN is restarted
		sc.Restarted(startedTime)
	}

	uid := v1alpha1.GetCNPodUUID(pod)
	moVersion := common.GetSemanticVersion(&pod.ObjectMeta)
	var queryAddress string
	if err := c.withMOClientSet(ctx, func(ctx context.Context, handler *mocli.ClientSet) error {
		cn, ok := handler.StoreCache.GetCN(uid)
		if !ok {
			return gerrors.Errorf("CN with uuid %s not found", uid)
		}
		queryAddress = cn.QueryAddress
		return nil
	}); err != nil {
		ctx.Log.Info("error refresh stats, cn not found in store-cache", "error", err.Error())
		return c.patchStoreStats(ctx, sc)
	}

	_, diagnosDraining := pod.Annotations[diagnosDrainingAnno]
	diagosis := &connectionDiagnosis{
		Logger:  ctx.Log,
		Enabled: diagnosDraining,
	}
	count, err := c.getSessionCount(queryAddress, moVersion, diagosis)
	if err != nil {
		ctx.Log.Info("error get session count", "error", err.Error())
	} else {
		// update session count
		sc.SessionCount = count
	}
	var pipelineCount int
	if v1alpha1.HasMOFeature(moVersion, v1alpha1.MOFeaturePipelineInfo) {
		pipelineCount, err = c.getPipelineCount(queryAddress, diagosis)
		if err != nil {
			ctx.Log.Info("error get pipeline count", "error", err.Error())
		} else {
			// update pipeline count
			sc.PipelineCount = pipelineCount
		}
	} else {
		// clear pipeline count if feature is disabled
		sc.PipelineCount = 0
	}
	var replicaCount int
	if v1alpha1.HasMOFeature(moVersion, v1alpha1.MOFeatureShardingMigration) {
		replicaCount, err = c.getReplicaCount(queryAddress, diagosis)
		if err != nil {
			ctx.Log.Info("error get replica count", "error", err.Error())
		} else {
			sc.ReplicaCount = replicaCount
		}
	} else {
		sc.ReplicaCount = 0
	}

	return c.patchStoreStats(ctx, sc)
}

func (c *Controller) patchStoreStats(ctx *recon.Context[*corev1.Pod], sc *common.StoreScore) error {
	pod := ctx.Obj
	err := ctx.Patch(pod, func() error {
		if err := common.SetStoreScore(pod, sc); err != nil {
			return err
		}
		pod.Annotations[common.DeletionCostAnno] = strconv.Itoa(sc.GenDeletionCost())
		if pod.Labels == nil {
			pod.Labels = map[string]string{}
		}
		pod.Labels[common.CNUUIDLabelKey] = v1alpha1.GetCNPodUUID(pod)
		// NB: store-connections anno is no longer used in mo-operator, but must be kept for external compatibility
		// ref:
		pod.Annotations[v1alpha1.StoreConnectionAnno] = strconv.Itoa(sc.PipelineCount + sc.SessionCount)
		return nil
	})
	if err != nil {
		return errors.WrapPrefix(err, "error patch stats to pod anno", 0)
	}
	return nil
}

func (c *Controller) getSessionCount(queryAddress string, moVersion semver.Version, diagosis *connectionDiagnosis) (int, error) {
	var count int
	resp, err := c.queryCli.ShowProcessList(context.Background(), queryAddress)
	if err != nil {
		return 0, errors.WrapPrefix(err, "show processlist", 0)
	}
	for _, sess := range resp.GetSessions() {
		if v1alpha1.HasMOFeature(moVersion, v1alpha1.MOFeatureSessionSource) {
			if sess.FromProxy {
				count++
			}
		} else {
			if sess.Account != "" && sess.Account != "sys" {
				count++
			}
		}
	}
	if diagosis.Enabled && count > 0 {
		diagosis.Logger.Info("CN sessions", "count", count, "detail", resp.GetSessions())
	}
	return count, nil
}

func (c *Controller) getReplicaCount(queryAddress string, diagosis *connectionDiagnosis) (int, error) {
	resp, err := c.queryCli.GetReplicaCount(context.Background(), queryAddress)
	if err != nil {
		return 0, errors.WrapPrefix(err, "get replica count", 0)
	}
	if diagosis.Enabled {
		diagosis.Logger.Info("CN replica count", "count", resp.GetCount())
	}
	return int(resp.GetCount()), nil
}

func (c *Controller) getPipelineCount(queryAddress string, diagosis *connectionDiagnosis) (int, error) {
	resp, err := c.queryCli.GetPipelineInfo(context.Background(), queryAddress)
	if err != nil {
		return 0, errors.WrapPrefix(err, "get pipeline info", 0)
	}
	if diagosis.Enabled {
		diagosis.Logger.Info("CN pipeline count", "count", resp.GetCount())
	}
	return int(resp.GetCount()), nil
}

func (c *withCNSet) withMOClientSet(ctx *recon.Context[*corev1.Pod], fn func(context.Context, *mocli.ClientSet) error) error {
	pod := ctx.Obj
	ls, err := common.ResolveLogSet(ctx, c.cn)
	if err != nil {
		return errors.WrapPrefix(err, "error resolve logset", 0)
	}
	if !recon.IsReady(ls) {
		return recon.ErrReSync(fmt.Sprintf("logset is not ready for Pod %s, cannot update CN labels", pod.Name), retryInterval)
	}
	handler, err := c.clientMgr.GetClient(ls)
	if err != nil {
		return errors.WrapPrefix(err, "get HAKeeper client", 0)
	}
	timeout, cancel := context.WithTimeout(context.Background(), mocli.DefaultRPCTimeout)
	defer cancel()
	if err := fn(timeout, handler); err != nil {
		return err
	}
	return nil
}

func (c *Controller) setCNState(pod *corev1.Pod, state string) {
	if pod.Annotations == nil {
		pod.Annotations = map[string]string{}
	}
	pod.Annotations[common.CNStateAnno] = state
}

func (c *Controller) Observe(ctx *recon.Context[*corev1.Pod]) (recon.Action[*corev1.Pod], error) {
	return nil, c.observe(ctx)
}

func (c *Controller) Finalize(ctx *recon.Context[*corev1.Pod]) (bool, error) {
	// deletion also handled by observe
	return true, c.observe(ctx)
}

func (c *Controller) Reconcile(mgr manager.Manager) error {
	// Pod does not have generation field, so we cannot use the default reconcile
	return recon.Setup[*corev1.Pod](&corev1.Pod{}, "cnstore", mgr, c,
		recon.WithControllerOptions(controller.Options{
			MaxConcurrentReconciles: defaultConcurrency,
		}),
		recon.SkipStatusSync(),
		recon.WithPredicate(
			predicate.Or(predicate.LabelChangedPredicate{},
				predicate.GenerationChangedPredicate{},
				annotationChangedExcludeStats{},
				deletedPredicate{})),
		recon.WithBuildFn(func(b *builder.Builder) {
			b.WithEventFilter(predicate.NewPredicateFuncs(func(obj client.Object) bool {
				pod, ok := obj.(*corev1.Pod)
				if !ok {
					return false
				}
				if pod.Labels == nil {
					return false
				}
				if component, ok := pod.Labels[common.ComponentLabelKey]; !ok || component != "CNSet" {
					return false
				}
				return true
			}))
		}),
	)
}

// annotationChangedExcludeStats reconciles the object when annotations are changed (exclude stats)
type annotationChangedExcludeStats struct {
	predicate.Funcs
}

func (annotationChangedExcludeStats) Update(e event.UpdateEvent) bool {
	if e.ObjectOld == nil || e.ObjectNew == nil {
		return false
	}
	oldAnnos := e.ObjectOld.GetAnnotations()
	newAnnos := e.ObjectNew.GetAnnotations()
	for k, v := range newAnnos {
		// exclude stats
		if k == common.DeletionCostAnno || k == v1alpha1.StoreConnectionAnno || k == v1alpha1.StoreScoreAnno {
			continue
		}
		// only consider newly added annotations or annotation value change, deletion of annotation key
		// do not need to be reconciled
		if oldAnnos[k] != v {
			return true
		}
	}
	return false
}

// deletePredicate reconciles the object when the deletionTimestamp field is changed
type deletedPredicate struct {
	predicate.Funcs
}

func (deletedPredicate) Update(e event.UpdateEvent) bool {
	if e.ObjectOld == nil {
		return false
	}
	if e.ObjectNew == nil {
		return false
	}

	return !reflect.DeepEqual(e.ObjectNew.GetDeletionTimestamp(), e.ObjectOld.GetDeletionTimestamp())
}
