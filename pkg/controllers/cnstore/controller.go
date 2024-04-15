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

package cnstore

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/blang/semver/v4"
	recon "github.com/matrixorigin/controller-runtime/pkg/reconciler"
	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	"github.com/matrixorigin/matrixone-operator/pkg/controllers/common"
	"github.com/matrixorigin/matrixone-operator/pkg/hacli"
	"github.com/matrixorigin/matrixone-operator/pkg/querycli"
	logpb "github.com/matrixorigin/matrixone/pkg/pb/logservice"
	"github.com/matrixorigin/matrixone/pkg/pb/metadata"
	"github.com/openkruise/kruise-api/apps/pub"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"strconv"
	"strings"
	"time"
)

const (
	messageCNCordon             = "CNStoreCordon"
	messageCNPrepareStop        = "CNStorePrepareStop"
	messageCNStoreReady         = "CNStoreReady"
	messageCNStoreNotRegistered = "CNStoreNotRegistered"
)

const retryInterval = 5 * time.Second
const resyncInterval = 30 * time.Second

type Controller struct {
	clientMgr *hacli.HAKeeperClientManager
	queryCli  *querycli.Client
}

type withCNSet struct {
	*Controller

	cn *v1alpha1.CNSet
}

func NewController(mgr *hacli.HAKeeperClientManager, qc *querycli.Client) *Controller {
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
		err = wc.withHAKeeperClient(ctx, func(timeout context.Context, h *hacli.Handler) error {
			return h.Client.DeleteCNStore(timeout, logpb.DeleteCNStore{
				StoreID: uid,
			})
		})
		if err != nil {
			return errors.Wrap(err, "error remove CN store")
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
		return c.completeDraining(ctx)
	}
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
		return errors.Wrap(err, "patch pod readiness")
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
			return errors.Wrapf(err, "error parsing start time %s", startTimeStr)
		}
		startTime = parsed
	} else {
		startTime = time.Now()
		if err := ctx.Patch(pod, func() error {
			pod.Annotations[v1alpha1.StoreDrainingStartAnno] = startTime.Format(time.RFC3339)
			return nil
		}); err != nil {
			return errors.Wrap(err, "error patching store draining start time")
		}
	}
	// check whether timeout is reached
	if time.Since(startTime) > sc.GetStoreDrainTimeout() {
		ctx.Log.Info("store draining timeout, force delete CN", "uuid", uid)
		return c.completeDraining(ctx)
	}
	ctx.Log.Info("call HAKeeper to drain CN store", "uuid", uid)
	err := c.withHAKeeperClient(ctx, func(timeout context.Context, h *hacli.Handler) error {
		return h.Client.PatchCNStore(timeout, logpb.CNStateLabel{
			UUID:  uid,
			State: metadata.WorkState_Draining,
		})
	})
	if err != nil {
		// optimize: if the CN does not exist in HAKeeper, shortcut to complete draining
		if strings.Contains(err.Error(), "does not exist") {
			return c.completeDraining(ctx)
		}
		return errors.Wrap(err, "error set CN state draining")
	}
	if time.Since(startTime) < sc.GetMinDelayDuration() {
		return recon.ErrReSync("wait for min delay", retryInterval)
	}

	storeConnection, err := common.GetStoreScore(pod)
	if err != nil {
		return errors.Wrap(err, "error get store connection count")
	}
	if storeConnection.IsSafeToReclaim() {
		return c.completeDraining(ctx)
	}
	return recon.ErrReSync("wait for CN store draining", retryInterval)
}

func (c *withCNSet) completeDraining(ctx *recon.Context[*corev1.Pod]) error {
	if err := ctx.Patch(ctx.Obj, func() error {
		controllerutil.RemoveFinalizer(ctx.Obj, common.CNDrainingFinalizer)
		delete(ctx.Obj.Annotations, v1alpha1.StoreDrainingStartAnno)
		return nil
	}); err != nil {
		return errors.Wrap(err, "error removing CN draining finalizer")
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
		return errors.Wrap(err, "ensure finalizers for CNStore Pod")
	}
	// remove draining start time in case we regret formal deletion decision
	if pod.Annotations == nil {
		pod.Annotations = map[string]string{}
	}
	if err := ctx.Patch(pod, func() error {
		delete(pod.Annotations, v1alpha1.StoreDrainingStartAnno)
		return nil
	}); err != nil {
		return errors.Wrap(err, "removing CN draining start time")
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
			return errors.Wrap(err, "unmarshal CNLabels")
		}
	}

	var err error
	if c.cn.Spec.ScalingConfig.GetStoreDrainEnabled() {
		err = c.withHAKeeperClient(ctx, func(timeout context.Context, h *hacli.Handler) error {
			return h.Client.PatchCNStore(timeout, logpb.CNStateLabel{
				UUID:   uid,
				State:  metadata.WorkState_Working,
				Labels: common.ToStoreLabels(cnLabels),
			})
		})
	} else {
		err = c.withHAKeeperClient(ctx, func(timeout context.Context, h *hacli.Handler) error {
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
		return errors.Wrap(err, "patch pod readiness")
	}
	return nil
}

func (c *withCNSet) OnCordon(ctx *recon.Context[*corev1.Pod]) error {
	pod := ctx.Obj
	uid := v1alpha1.GetCNPodUUID(pod)
	ctx.Log.Info("call HAKeeper to cordon CN store", "uuid", uid)
	err := c.withHAKeeperClient(ctx, func(timeout context.Context, h *hacli.Handler) error {
		return h.Client.PatchCNStore(timeout, logpb.CNStateLabel{
			UUID:  uid,
			State: metadata.WorkState_Draining,
		})
	})
	if err != nil {
		return errors.Wrap(err, "error cordon cn store")
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
		return errors.Wrap(err, "error resolve CNSet")
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
		return errors.Wrap(err, "error set pod normal")
	}
	// trigger next reconciliation later to refresh the stats
	// TODO(aylei): better stats handling
	return recon.ErrReSync("resync", resyncInterval)
}

func (c *withCNSet) syncStats(ctx *recon.Context[*corev1.Pod]) error {
	pod := ctx.Obj
	uid := v1alpha1.GetCNPodUUID(pod)
	moVersion := common.GetSemanticVersion(&pod.ObjectMeta)
	var queryAddress string
	if err := c.withHAKeeperClient(ctx, func(ctx context.Context, handler *hacli.Handler) error {
		cn, ok := handler.StoreCache.GetCN(uid)
		if !ok {
			return errors.Errorf("CN with uuid %s not found", uid)
		}
		queryAddress = cn.QueryAddress
		return nil
	}); err != nil {
		ctx.Log.Info("error sync stats, cn not found in store-cache", "error", err.Error())
		return nil
	}
	count, err := c.getSessionCount(queryAddress, moVersion)
	if err != nil {
		return errors.Wrap(err, "error get sessions")
	}
	var pipelineCount int
	if v1alpha1.HasMOFeature(moVersion, v1alpha1.MOFeaturePipelineInfo) {
		pipelineCount, err = c.getPipelineCount(queryAddress)
		if err != nil {
			return errors.Wrap(err, "error get pipeline count")
		}
	}

	sc := &common.StoreScore{
		SessionCount:  count,
		PipelineCount: pipelineCount,
	}
	err = ctx.Patch(pod, func() error {
		if err := common.SetStoreScore(pod, sc); err != nil {
			return err
		}
		pod.Annotations[common.DeletionCostAnno] = strconv.Itoa(sc.GenDeletionCost())
		if pod.Labels == nil {
			pod.Labels = map[string]string{}
		}
		pod.Labels[common.CNUUIDLabelKey] = uid
		// NB: store-connections anno is no longer used in mo-operator, but must be kept for external compatibility
		// ref: https://github.com/matrixorigin/MO-Cloud/issues/3007
		pod.Annotations[v1alpha1.StoreConnectionAnno] = strconv.Itoa(sc.PipelineCount + sc.SessionCount)
		return nil
	})
	if err != nil {
		return errors.Wrap(err, "error patch stats to pod anno")
	}
	return nil
}

func (c *Controller) getSessionCount(queryAddress string, moVersion semver.Version) (int, error) {
	var count int
	resp, err := c.queryCli.ShowProcessList(context.Background(), queryAddress)
	if err != nil {
		return 0, errors.Wrap(err, "show processlist")
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
	return count, nil
}

func (c *Controller) getPipelineCount(queryAddress string) (int, error) {
	resp, err := c.queryCli.GetPipelineInfo(context.Background(), queryAddress)
	if err != nil {
		return 0, errors.Wrap(err, "get pipeline info")
	}
	return int(resp.GetCount()), nil
}

func (c *withCNSet) withHAKeeperClient(ctx *recon.Context[*corev1.Pod], fn func(context.Context, *hacli.Handler) error) error {
	pod := ctx.Obj
	ls, err := common.ResolveLogSet(ctx, c.cn)
	if err != nil {
		return errors.Wrap(err, "error resolve logset")
	}
	if !recon.IsReady(ls) {
		return recon.ErrReSync(fmt.Sprintf("logset is not ready for Pod %s, cannot update CN labels", pod.Name), retryInterval)
	}
	handler, err := c.clientMgr.GetClient(ls)
	if err != nil {
		return errors.Wrap(err, "get HAKeeper client")
	}
	timeout, cancel := context.WithTimeout(context.Background(), hacli.HAKeeperTimeout)
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
