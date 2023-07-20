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

package cnstore

import (
	"context"
	"encoding/json"
	"fmt"
	recon "github.com/matrixorigin/controller-runtime/pkg/reconciler"
	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	"github.com/matrixorigin/matrixone-operator/pkg/controllers/common"
	"github.com/matrixorigin/matrixone-operator/pkg/controllers/mosql"
	"github.com/matrixorigin/matrixone-operator/pkg/hacli"
	"github.com/matrixorigin/matrixone/pkg/logservice"
	logpb "github.com/matrixorigin/matrixone/pkg/pb/logservice"
	"github.com/matrixorigin/matrixone/pkg/pb/metadata"
	"github.com/openkruise/kruise-api/apps/pub"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"time"
)

const (
	storeDrainingStartAnno = "matrixorigin.io/store-draining-start"
)

const retryInterval = 15 * time.Second

type Controller struct {
	clientMgr *hacli.HAKeeperClientManager
}

type withCNSet struct {
	*Controller

	cn *v1alpha1.CNSet
}

func NewController(mgr *hacli.HAKeeperClientManager) *Controller {
	return &Controller{clientMgr: mgr}
}

var _ recon.Actor[*corev1.Pod] = &Controller{}

// OnDeleted delete CNStore and cleanup finalizer on Pod deletion
func (c *withCNSet) OnDeleted(ctx *recon.Context[*corev1.Pod]) error {
	pod := ctx.Obj
	if err := ctx.Patch(pod, func() error {
		controllerutil.RemoveFinalizer(pod, common.CNDrainingFinalizer)
		return nil
	}); err != nil {
		return err
	}
	return nil
}

// OnPreparingDelete drains CN connections
func (c *withCNSet) OnPreparingDelete(ctx *recon.Context[*corev1.Pod]) error {
	pod := ctx.Obj
	uid := v1alpha1.GetCNPodUUID(ctx.Obj)
	if pod.Annotations == nil {
		pod.Annotations = map[string]string{}
	}

	// store draining disabled, cleanup finalizers and skip
	sc := c.cn.Spec.ScalingConfig
	if !sc.GetStoreDrainEnabled() {
		return c.completeDraining(ctx)
	}

	// start draining
	var startTime time.Time
	startTimeStr, ok := pod.Annotations[storeDrainingStartAnno]
	if ok {
		parsed, err := time.Parse(time.RFC3339, startTimeStr)
		if err != nil {
			return errors.Wrapf(err, "error parsing start time %s", startTimeStr)
		}
		startTime = parsed
	} else {
		startTime = time.Now()
		if err := ctx.Patch(pod, func() error {
			pod.Annotations[storeDrainingStartAnno] = startTime.Format(time.RFC3339)
			return nil
		}); err != nil {
			return errors.Wrap(err, "error patching store draining start time")
		}
	}
	ctx.Log.Info("call HAKeeper to drain CN store", "uuid", uid)
	err := c.withHAKeeperClient(ctx, func(timeout context.Context, hc logservice.ProxyHAKeeperClient) error {
		return hc.PatchCNStore(timeout, logpb.CNStateLabel{
			UUID:  uid,
			State: metadata.WorkState_Draining,
		})
	})
	if err != nil {
		return errors.Wrap(err, "error set CN state draining")
	}
	if err := c.setCNState(ctx, v1alpha1.CNStoreStateDraining); err != nil {
		return err
	}
	ctx.Log.Info("call MO to collect Store status", "uuid", uid)
	cnDrained := false
	if c.cn.Spec.MetricsSecretRef == nil {
		ctx.Log.Info("CN metric secret does not initialized, cannot query state")
	} else {
		// TODO: should use other CN to query conns
		sqlcli := mosql.NewClient(fmt.Sprintf("%s:%d", pod.Status.PodIP, 6001), ctx.Client, c.cn.Spec.MetricsSecretRef.NamespacedName())
		connCount, err := sqlcli.GetServerConnection(ctx, uid)
		if err != nil {
			return err
		}
		if connCount < 1 {
			cnDrained = true
		}
	}
	if cnDrained {
		return c.completeDraining(ctx)
	}
	if time.Since(startTime) > sc.GetStoreDrainTimeout() {
		ctx.Log.Info("store draining timeout, force delete CN", "uuid", uid)
		return c.completeDraining(ctx)
	}
	return recon.ErrReSync("wait for CN store draining", retryInterval)
}

func (c *withCNSet) completeDraining(ctx *recon.Context[*corev1.Pod]) error {
	if err := ctx.Patch(ctx.Obj, func() error {
		controllerutil.RemoveFinalizer(ctx.Obj, common.CNDrainingFinalizer)
		delete(ctx.Obj.Annotations, storeDrainingStartAnno)
		return nil
	}); err != nil {
		return errors.Wrap(err, "error removing CN draining finalizer")
	}
	return nil
}

// OnNormal ensure CNStore labels and transit CN store to UP state
func (c *withCNSet) OnNormal(ctx *recon.Context[*corev1.Pod]) error {
	pod := ctx.Obj

	// 1. ensure finalizers
	if c.cn.Spec.ScalingConfig.GetStoreDrainEnabled() {
		if err := ctx.Patch(ctx.Obj, func() error {
			controllerutil.AddFinalizer(ctx.Obj, common.CNDrainingFinalizer)
			return nil
		}); err != nil {
			return errors.Wrap(err, "ensure finalizers for CNStore Pod")
		}
	}

	// 2. remove draining start time in case we regret formal deletion decision
	if pod.Annotations == nil {
		pod.Annotations = map[string]string{}
	}
	if err := ctx.Patch(pod, func() error {
		delete(pod.Annotations, storeDrainingStartAnno)
		return nil
	}); err != nil {
		return errors.Wrap(err, "removing CN draining start time")
	}

	// 3. sync CN labels for store and mark store as UP state
	labelStr, ok := pod.Annotations[common.CNLabelAnnotation]
	if !ok {
		// no label to sync
		return nil
	}
	var cnLabels []v1alpha1.CNLabel
	err := json.Unmarshal([]byte(labelStr), &cnLabels)
	if err != nil {
		return errors.Wrap(err, "unmarshal CNLabels")
	}
	uid := v1alpha1.GetCNPodUUID(pod)
	err = c.withHAKeeperClient(ctx, func(timeout context.Context, hc logservice.ProxyHAKeeperClient) error {
		return hc.PatchCNStore(timeout, logpb.CNStateLabel{
			UUID:   uid,
			State:  metadata.WorkState_Working,
			Labels: toStoreLabels(cnLabels),
		})
	})
	if err != nil {
		ctx.Log.Error(err, "update CN failed", "uuid", uid)
		return recon.ErrReSync("update cn failed", retryInterval)
	}
	return c.setCNState(ctx, v1alpha1.CNStoreStateUp)
}

func (c *Controller) observe(ctx *recon.Context[*corev1.Pod]) error {
	pod := ctx.Obj
	if component, ok := pod.Labels[common.ComponentLabelKey]; !ok || component != "CNSet" {
		ctx.Log.V(4).Info("pod is not a CN pod, skip", zap.String("namespace", pod.Namespace), zap.String("name", pod.Name))
		return nil
	}
	cnName, ok := pod.Labels[common.InstanceLabelKey]
	if !ok || cnName == "" {
		return errors.Errorf("cannot find CNSet for CN pod %s/%s, instance label is empty", pod.Namespace, pod.Name)
	}
	cn := &v1alpha1.CNSet{}
	if err := ctx.Get(types.NamespacedName{Namespace: pod.Namespace, Name: cnName}, cn); err != nil {
		return errors.Wrap(err, "get CNSet")
	}
	wc := &withCNSet{
		Controller: c,
		cn:         cn,
	}

	if pod.DeletionTimestamp != nil {
		return wc.OnDeleted(ctx)
	}
	if state, ok := pod.Labels[pub.LifecycleStateKey]; ok && state == string(pub.LifecycleStatePreparingDelete) {
		return wc.OnPreparingDelete(ctx)
	}
	return wc.OnNormal(ctx)
}

func (c *withCNSet) withHAKeeperClient(ctx *recon.Context[*corev1.Pod], fn func(context.Context, logservice.ProxyHAKeeperClient) error) error {
	pod := ctx.Obj
	cn := c.cn
	// TODO: consider edge cluster federation scenario
	if cn.Deps.LogSet == nil {
		return errors.Errorf("cannot get logset of CN pod %s/%s, logset dep is nil", pod.Namespace, pod.Name)
	}
	ls := &v1alpha1.LogSet{}
	// refresh logset status
	if err := ctx.Get(client.ObjectKeyFromObject(cn.Deps.LogSet), ls); err != nil {
		return errors.Wrap(err, "error get logset")
	}
	if !recon.IsReady(ls) {
		return recon.ErrReSync(fmt.Sprintf("logset is not ready for Pod %s, cannot update CN labels", pod.Name), retryInterval)
	}
	haClient, err := c.clientMgr.GetClient(ls)
	if err != nil {
		return errors.Wrap(err, "get HAKeeper client")
	}
	timeout, cancel := context.WithTimeout(context.Background(), hacli.HAKeeperTimeout)
	defer cancel()
	if err := fn(timeout, haClient); err != nil {
		return err
	}
	return nil
}

func (c *Controller) setCNState(ctx *recon.Context[*corev1.Pod], state string) error {
	err := ctx.Patch(ctx.Obj, func() error {
		if ctx.Obj.Annotations == nil {
			ctx.Obj.Annotations = map[string]string{}
		}
		ctx.Obj.Annotations[common.CNStateAnno] = state
		return nil
	})
	if err != nil {
		return errors.Wrap(err, "error set cn state")
	}
	return nil
}

func toStoreLabels(labels []v1alpha1.CNLabel) map[string]metadata.LabelList {
	lm := make(map[string]metadata.LabelList, len(labels))
	for _, l := range labels {
		lm[l.Key] = metadata.LabelList{
			Labels: l.Values,
		}
	}
	return lm
}

func (c *Controller) Observe(ctx *recon.Context[*corev1.Pod]) (recon.Action[*corev1.Pod], error) {
	return nil, c.observe(ctx)
}

func (c *Controller) Finalize(ctx *recon.Context[*corev1.Pod]) (bool, error) {
	// deletion alo handled by observe
	return true, c.observe(ctx)
}

func (c *Controller) Reconcile(mgr manager.Manager) error {
	// Pod does not have generation field, so we cannot use the default reconcile
	return recon.Setup[*corev1.Pod](&corev1.Pod{}, "cnstore", mgr, c,
		recon.SkipStatusSync(),
		recon.WithPredicate(
			predicate.Or(predicate.LabelChangedPredicate{},
				predicate.AnnotationChangedPredicate{},
				predicate.GenerationChangedPredicate{},
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
