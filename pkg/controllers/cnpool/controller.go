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

package cnpool

import (
	"context"
	"fmt"
	"github.com/go-errors/errors"
	"github.com/go-logr/logr"
	recon "github.com/matrixorigin/controller-runtime/pkg/reconciler"
	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	"github.com/matrixorigin/matrixone-operator/pkg/controllers/common"
	"github.com/matrixorigin/matrixone-operator/pkg/utils"
	"golang.org/x/exp/slices"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"time"
)

// Actor reconciles CN Pool
type Actor struct {
	Logger logr.Logger
}

func (r *Actor) Observe(ctx *recon.Context[*v1alpha1.CNPool]) (recon.Action[*v1alpha1.CNPool], error) {
	return nil, r.Sync(ctx)
}

func (r *Actor) Sync(ctx *recon.Context[*v1alpha1.CNPool]) error {
	p := ctx.Obj
	desired, err := buildCNSet(p)
	if err != nil {
		return errors.WrapPrefix(err, "erros build CNSet", 0)
	}

	ls := ownedLabels(p)
	csList := &v1alpha1.CNSetList{}
	if err := ctx.List(csList, client.InNamespace(p.Namespace), client.MatchingLabels(ls)); err != nil {
		return errors.WrapPrefix(err, "error list current sets", 0)
	}

	var totalPods int32
	maxPods := p.Spec.Strategy.ScaleStrategy.GetMaxPods()
	var current *v1alpha1.CNSet
	for i := range csList.Items {
		cs := csList.Items[i]
		if cs.Name == desired.Name {
			current = &cs
			continue
		}
		legacyReplicas, err := r.syncLegacySet(ctx, &cs)
		if err != nil {
			return errors.Wrap(err, 0)
		}
		totalPods += legacyReplicas
	}

	var inUse int32
	var idlePods []*corev1.Pod
	var terminatingPods []*corev1.Pod
	if current != nil {
		pods, err := listCNSetPods(ctx, current)
		if err != nil {
			return errors.Wrap(err, 0)
		}
		for i := range pods {
			if podInUse(&pods[i]) {
				inUse++
			} else if pods[i].Labels[v1alpha1.CNPodPhaseLabel] == v1alpha1.CNPodPhaseTerminating {
				ctx.Log.Info("find pod still in terminating state", "pod", pods[i].Name)
				terminatingPods = append(terminatingPods, &pods[i])
			} else {
				idlePods = append(idlePods, &pods[i])
			}
		}
	}

	claims, err := listNominatedClaims(ctx, p)
	if err != nil {
		return errors.Wrap(err, 0)
	}
	var pendingClaims int32
	for _, claim := range claims {
		if claim.Status.Phase == v1alpha1.CNClaimPhasePending {
			pendingClaims++
		}
	}

	desiredReplicas := inUse + pendingClaims + p.Spec.Strategy.ScaleStrategy.MaxIdle
	totalPods += desiredReplicas
	if totalPods > maxPods {
		return recon.ErrReSync(fmt.Sprintf("Pool %s has reached MaxPods limit %d, total Pods: %d, requeue", p.Name, totalPods, maxPods), time.Minute)
	}
	// ensure and scale desired CNSet to provide enough CN pods
	err = recon.CreateOwnedOrUpdate(ctx, desired, func() error {
		// apply update, since the CNSet revision hash is not changed, this must be an inplace-update
		csSpec := p.Spec.Template.DeepCopy()
		syncCNSetSpec(p, csSpec)
		desired.Spec = *csSpec
		ctx.Log.Info("scale cnset", "cnset", desired.Name, "replicas", desiredReplicas)
		// sync terminating pods to delete
		desired.Spec.PodsToDelete = podNames(terminatingPods)
		if desired.Spec.Replicas > desiredReplicas {
			// CNSet is going to be scaled-in
			if pendingClaims > 0 {
				// don't scale-in if we still have pending claims
				ctx.Log.Info("pool has enough pods but there's still pending claims, pause scale-in",
					"current replicas", desired.Spec.Replicas,
					"desired replicas", desiredReplicas,
					"pending claims", pendingClaims,
					"in use pods", inUse)
				return nil
			}
			scaleInCount := desired.Spec.Replicas - desiredReplicas
			sortPodByDeletionOrder(idlePods)
			if int32(len(idlePods)) > scaleInCount {
				// pick first N to scale-in
				idlePods = idlePods[0:scaleInCount]
			}
			ctx.Log.Info("try scale-in CN Pool", "pods", len(idlePods))
			// set pod to terminating phase, optimistic lock prevents race
			var deleted []*corev1.Pod
			for i := range idlePods {
				pod := idlePods[i]
				idlePods[i].Labels[v1alpha1.CNPodPhaseLabel] = v1alpha1.CNPodPhaseTerminating
				if err := ctx.Update(idlePods[i]); err != nil {
					ctx.Log.Error(err, "error set pod terminating", "pod", idlePods[i].Name)
					continue
				}
				deleted = append(deleted, pod)
			}
			ctx.Log.Info("scale-in CN Pool complete", "deleted", len(deleted))
			desired.Spec.Replicas = desired.Spec.Replicas - int32(len(deleted))
			desired.Spec.PodsToDelete = append(desired.Spec.PodsToDelete, podNames(deleted)...)
		} else {
			// scale-out, if we have terminating pods left, replace them
			desired.Spec.Replicas = desiredReplicas
		}
		// GC the pods that have already been deleted from podsToDelete
		return nil
	})
	if err != nil {
		return errors.Wrap(err, 0)
	}
	return nil
}

// TODO(aylei): rethink here, operator should try it best to reuse cache
func (r *Actor) syncLegacySet(ctx *recon.Context[*v1alpha1.CNPool], cnSet *v1alpha1.CNSet) (int32, error) {
	var replicas int32
	pods, err := listCNSetPods(ctx, cnSet)
	if err != nil {
		return replicas, errors.Wrap(err, 0)
	}
	var toDelete []string
	for i := range pods {
		pod := pods[i]
		// TODO(aylei): reclaim timeout logic
		if podInUse(&pod) {
			// keep in-use CN but try reclaim the cnclaim
			if err := r.reclaimLegacyCNClaim(ctx, &pod); err != nil {
				return replicas, err
			}
			replicas++
		} else {
			// recalim other CN
			toDelete = append(toDelete, pods[i].Name)
		}
	}
	// clean unused CNSet
	if err := recon.CreateOwnedOrUpdate(ctx, cnSet, func() error {
		cnSet.Spec.Replicas = replicas
		cnSet.Spec.PodsToDelete = toDelete
		return nil
	}); err != nil {
		return replicas, errors.Wrap(err, 0)
	}
	// legacy CNSet is scaled to zero the scaling has been done, GC it
	if cnSet.Spec.Replicas == 0 && cnSet.Status.Replicas == 0 {
		if err := ctx.Delete(cnSet); err != nil {
			return replicas, errors.Wrap(err, 0)
		}
	}
	return replicas, nil
}

func (r *Actor) reclaimLegacyCNClaim(ctx *recon.Context[*v1alpha1.CNPool], pod *corev1.Pod) error {
	if pod.Labels[v1alpha1.CNPodPhaseLabel] != v1alpha1.CNPodPhaseBound {
		return nil
	}
	claimName := pod.Labels[v1alpha1.PodClaimedByLabel]
	if claimName == "" {
		return nil
	}
	claim := &v1alpha1.CNClaim{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: pod.Namespace,
			Name:      claimName,
		},
	}
	if err := ctx.Get(client.ObjectKeyFromObject(claim), claim); err != nil {
		if apierrors.IsNotFound(err) {
			return errors.Errorf("CNClaim %s/%s not found while pod is bound", pod.Namespace, claimName)
		}
		return errors.WrapPrefix(err, "error get claim", 0)
	}
	return ctx.PatchStatus(claim, func() error {
		claim.Status.Phase = v1alpha1.CNClaimPhaseOutdated
		return nil
	})
}

// listNominatedClaims list all claims that are nominated to this pool
func listNominatedClaims(cli recon.KubeClient, pool *v1alpha1.CNPool) ([]v1alpha1.CNClaim, error) {
	claimList := &v1alpha1.CNClaimList{}
	if err := cli.List(claimList, client.InNamespace(pool.Namespace), client.MatchingLabels(map[string]string{
		v1alpha1.PoolNameLabel: pool.Name,
	})); err != nil {
		return nil, errors.Wrap(err, 0)
	}
	return claimList.Items, nil
}

func listCNSetPods(cli recon.KubeClient, cnSet *v1alpha1.CNSet) ([]corev1.Pod, error) {
	if cnSet.Status.LabelSelector == "" {
		return nil, nil
	}
	ls, err := metav1.ParseToLabelSelector(cnSet.Status.LabelSelector)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}
	podList := &corev1.PodList{}
	if err := cli.List(podList, client.InNamespace(cnSet.Namespace), client.MatchingLabels(ls.MatchLabels)); err != nil {
		return nil, errors.Wrap(err, 0)
	}
	return podList.Items, nil
}

func buildCNSet(p *v1alpha1.CNPool) (*v1alpha1.CNSet, error) {
	csSpec := p.Spec.Template.DeepCopy()
	syncCNSetSpec(p, csSpec)
	// generate the controller revision hash
	hash, err := generateRevisionHash(csSpec)
	if err != nil {
		return nil, errors.WrapPrefix(err, "error generating hash", 0)
	}
	name := fmt.Sprintf("%s-%s", p.Name, hash)

	labels := ownedLabels(p)
	labels[appsv1.ControllerRevisionHashLabelKey] = hash
	cs := &v1alpha1.CNSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: p.Namespace,
			Labels:    labels,
		},
		Spec: *csSpec,
		Deps: p.Spec.Deps,
	}
	return cs, nil
}

func syncCNSetSpec(p *v1alpha1.CNPool, csSpec *v1alpha1.CNSetSpec) {
	// override fields that managed by Pool, we expect the webhook will reject these fields if
	// they are set by user, so that this process would not silently change users' expectation.
	csSpec.PodManagementPolicy = pointer.String(v1alpha1.PodManagementPolicyPooling)
	// pause update, cn pool don't rolling-update a single set, instead, we roll-out new sets if spec changes
	csSpec.PauseUpdate = true
	csSpec.ScalingConfig.StoreDrainEnabled = pointer.Bool(true)
	csSpec.Labels = nil
	csSpec.PodSet.Replicas = 0
	csSpec.ServiceType = ""
	csSpec.ServiceAnnotations = nil
	csSpec.NodePort = nil
	// don't create surge pod when in-place mutate the underlying CNSet (e.g. mutate annotations)
	csSpec.UpdateStrategy.MaxSurge = utils.PtrTo(intstr.FromInt(0))
	csSpec.UpdateStrategy.MaxUnavailable = utils.PtrTo(intstr.FromInt(1))
	if csSpec.Overlay == nil {
		csSpec.Overlay = &v1alpha1.Overlay{}
	}
	if csSpec.Overlay.PodLabels == nil {
		csSpec.Overlay.PodLabels = map[string]string{}
	}
	for k, v := range p.Spec.PodLabels {
		csSpec.Overlay.PodLabels[k] = v
	}
	csSpec.Overlay.PodLabels[v1alpha1.CNPodPhaseLabel] = v1alpha1.CNPodPhaseUnknown
	csSpec.Overlay.PodLabels[v1alpha1.PoolNameLabel] = p.Name
}

func generateRevisionHash(cn *v1alpha1.CNSetSpec) (string, error) {
	tpl := &v1alpha1.CNSetSpec{}
	tpl.PodSet = *cn.PodSet.DeepCopy()
	tpl.ConfigThatChangeCNSpec = *cn.ConfigThatChangeCNSpec.DeepCopy()
	// special case: PodMeta can be in-place updated without restarting container
	tpl.PodSet.Overlay.PodLabels = nil
	tpl.PodSet.Overlay.PodAnnotations = nil
	return common.HashControllerRevision(tpl)
}

func (r *Actor) Finalize(ctx *recon.Context[*v1alpha1.CNPool]) (bool, error) {

	ls := ownedLabels(ctx.Obj)
	if err := ctx.Client.DeleteAllOf(ctx, &v1alpha1.CNSet{}, client.InNamespace(ctx.Obj.Namespace), client.MatchingLabels(ls)); err != nil {
		if apierrors.IsNotFound(err) {
			return true, nil
		}
		return false, errors.Wrap(err, 0)
	}
	csList := &v1alpha1.CNSetList{}
	if err := ctx.List(csList, client.InNamespace(ctx.Obj.Namespace), client.MatchingLabels(ls)); err != nil {
		if apierrors.IsNotFound(err) {
			return true, nil
		}
		return false, errors.Wrap(err, 0)
	}
	return len(csList.Items) < 1, nil
}

func ownedLabels(p *v1alpha1.CNPool) map[string]string {
	return map[string]string{
		common.LabelManagedBy: fmt.Sprintf("%s-%s", p.Kind, p.Name),
		common.LabelOwnerUID:  string(p.UID),
	}
}

func podInUse(pod *corev1.Pod) bool {
	return pod.Labels[v1alpha1.CNPodPhaseLabel] == v1alpha1.CNPodPhaseBound ||
		pod.Labels[v1alpha1.CNPodPhaseLabel] == v1alpha1.CNPodPhaseDraining
}

// sortPodByDeletionOrder sort the pool pods to be deleted
func sortPodByDeletionOrder(pods []*corev1.Pod) {
	slices.SortFunc(pods, deletionOrder)
}

func deletionOrder(a, b *corev1.Pod) int {
	c := deletionCost(a) - deletionCost(b)
	if c == 0 {
		// if two pods have same deletion cost, delete the newly created one first
		return -int(a.CreationTimestamp.Sub(b.CreationTimestamp.Time).Seconds())
	}
	return c
}

func deletionCost(pod *corev1.Pod) int {
	score := 0
	switch pod.Labels[v1alpha1.CNPodPhaseLabel] {
	case v1alpha1.CNPodPhaseTerminating:
		score += 0
	case "", v1alpha1.CNPodPhaseUnknown:
		score += 1 << 8
	case v1alpha1.CNPodPhaseIdle:
		score += 1 << 9
	default:
		score += 1 << 10
	}
	// scale un-used pod first in same phase
	if pod.Labels[v1alpha1.PodClaimedByLabel] != "" {
		score++
	}
	return score
}

func (r *Actor) Start(mgr manager.Manager) error {
	return recon.Setup[*v1alpha1.CNPool](&v1alpha1.CNPool{}, "cn-pool-manager", mgr, r, recon.WithBuildFn(func(b *builder.Builder) {
		b.Watches(&corev1.Pod{}, handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, object client.Object) []reconcile.Request {
			pod, ok := object.(*corev1.Pod)
			if !ok {
				return nil
			}
			poolName, ok := pod.Labels[v1alpha1.PoolNameLabel]
			if !ok {
				return nil
			}
			return []reconcile.Request{{
				NamespacedName: types.NamespacedName{
					Namespace: pod.Namespace,
					Name:      poolName,
				},
			}}
		}))
		b.Watches(&v1alpha1.CNClaim{}, handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, object client.Object) []reconcile.Request {
			claim, ok := object.(*v1alpha1.CNClaim)
			if !ok {
				return nil
			}
			if claim.Spec.PoolName == "" {
				return nil
			}
			return []reconcile.Request{{
				NamespacedName: types.NamespacedName{
					Namespace: claim.Namespace,
					Name:      claim.Spec.PoolName,
				},
			}}
		}))
		b.Owns(&v1alpha1.CNSet{})
	}))
}
func podNames(pods []*corev1.Pod) []string {
	var ss []string
	for _, pod := range pods {
		ss = append(ss, pod.Name)
	}
	return ss
}
