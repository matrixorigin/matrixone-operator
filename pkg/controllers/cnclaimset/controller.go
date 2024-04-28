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

package cnclaimset

import (
	"context"
	"github.com/go-errors/errors"
	recon "github.com/matrixorigin/controller-runtime/pkg/reconciler"
	"github.com/matrixorigin/controller-runtime/pkg/util"
	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	"github.com/matrixorigin/matrixone-operator/pkg/controllers/common"
	"golang.org/x/exp/slices"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"strconv"
	"time"
)

const (
	ClaimInstanceIDLabel = "claim.matrixorigin.io/instance-id"

	LengthOfInstanceID = 5
)

var podPhaseToOrdinal = map[corev1.PodPhase]int{
	corev1.PodPending: 0,
	corev1.PodFailed:  1,
	corev1.PodRunning: 2,
}

var claimPhaseToOrdinal = map[v1alpha1.CNClaimPhase]int{
	v1alpha1.CNClaimPhasePending:  0,
	v1alpha1.CNClaimPhaseLost:     1,
	v1alpha1.CNClaimPhaseOutdated: 2,
	v1alpha1.CNClaimPhaseBound:    3,
}

type ownedClaims struct {
	owned []v1alpha1.CNClaim
	lost  []v1alpha1.CNClaim
}

// Actor reconciles CNClaimSet
type Actor struct {
	ClientNoCache client.Client
}

func NewActor(clientNoCache client.Client) *Actor {
	return &Actor{
		ClientNoCache: clientNoCache,
	}
}

func (r *Actor) Observe(ctx *recon.Context[*v1alpha1.CNClaimSet]) (recon.Action[*v1alpha1.CNClaimSet], error) {
	return nil, r.Sync(ctx)
}

func (r *Actor) Sync(ctx *recon.Context[*v1alpha1.CNClaimSet]) error {
	s := ctx.Obj
	oc, err := listOwnedClaims(ctx, ctx.Client, s)
	if err != nil {
		return errors.WrapPrefix(err, "error filter claims", 0)
	}
	if int32(len(oc.owned)) != s.Status.Replicas {
		// check whether the cache is in sync
		realC, err := listOwnedClaims(ctx, r.ClientNoCache, s)
		if err != nil {
			return errors.WrapPrefix(err, "error list claims directly", 0)
		}
		if len(oc.owned) != len(realC.owned) || len(oc.lost) != len(realC.lost) {
			// simply requeue to wait cache sync, since we heavily rely on cache in the following reconciliation
			ctx.Log.Info("cache not synced, wait",
				"cached owned", len(oc.owned),
				"real owned", len(realC.owned),
				"cached lost", len(oc.lost),
				"real lost", len(realC.lost))
			return recon.ErrReSync("wait cache sync", time.Second)
		}
	}
	if err := r.scale(ctx, oc); err != nil {
		return errors.WrapPrefix(err, "error scale cnclaimset", 0)
	}
	// clean lost claims
	if err := cleanClaims(ctx, oc.lost); err != nil {
		return errors.WrapPrefix(err, "clean filter out claims", 0)
	}
	// collect status
	var claimStatuses []v1alpha1.CNClaimStatus
	s.Status.Replicas = int32(len(oc.owned))
	var readyReplicas int32
	for _, c := range oc.owned {
		claimStatuses = append(claimStatuses, c.Status)
		if c.IsReady() {
			readyReplicas++
		}
	}
	// used to resolve all CN pods belonged to this CNSet
	podSelector := common.MustAsSelector(&metav1.LabelSelector{MatchLabels: map[string]string{
		v1alpha1.ClaimSetNameLabel: s.Name,
		v1alpha1.CNPodPhaseLabel:   v1alpha1.CNPodPhaseBound,
	}})
	s.Status.ReadyReplicas = readyReplicas
	s.Status.Claims = claimStatuses
	s.Status.LabelSelector = podSelector.String()
	return nil
}

func (r *Actor) scale(ctx *recon.Context[*v1alpha1.CNClaimSet], oc *ownedClaims) error {
	s := ctx.Obj
	var updated int
	var outdated int
	for _, c := range oc.owned {
		if c.IsReady() {
			if c.IsUpdated() {
				updated++
			} else {
				outdated++
			}
		}
	}
	current := len(oc.owned)
	ctx.Log.Info("scale claimset", "desiredReplicas", s.Spec.Replicas, "updatedReplicas", updated, "outdatedReplicas", outdated)
	desiredReplicas := int(s.Spec.Replicas)
	// TODO(aylei): simplify the following logic, hard to understand
	// total bound Claim exceed replicas, scale-in
	if updated+outdated > desiredReplicas {
		return r.scaleIn(ctx, oc, current-desiredReplicas)
	}
	// if the updated replicas < desiredReplicas, add 1 surge replica to rollout the claimset
	if updated+outdated >= desiredReplicas && updated < desiredReplicas {
		desiredReplicas++
	}
	// normal scaling
	if desiredReplicas > current {
		return r.scaleOut(ctx, oc, desiredReplicas-current)
	}
	if desiredReplicas < current {
		return r.scaleOut(ctx, oc, desiredReplicas-current)
	}
	return nil
}

func (r *Actor) scaleOut(ctx *recon.Context[*v1alpha1.CNClaimSet], oc *ownedClaims, count int) error {
	used := sets.New[string]()
	for _, c := range oc.owned {
		used.Insert(c.Labels[ClaimInstanceIDLabel])
	}
	for _, c := range oc.lost {
		used.Insert(c.Labels[ClaimInstanceIDLabel])
	}
	ids := genAvailableIds(count, used)
	for _, id := range ids {
		claim := makeClaim(ctx.Obj, id)
		err := ctx.CreateOwned(claim)
		if err != nil {
			return errors.WrapPrefix(err, "error create new Claim", 0)
		}
		oc.owned = append(oc.owned, *claim)
	}
	return nil
}

func makeClaim(cs *v1alpha1.CNClaimSet, id string) *v1alpha1.CNClaim {
	tpl := cs.Spec.Template
	labels := tpl.Labels
	labels[v1alpha1.ClaimSetNameLabel] = cs.Name
	labels[ClaimInstanceIDLabel] = id
	// allow client to override ownerName in claimTemplate
	if tpl.Spec.OwnerName == nil {
		tpl.Spec.OwnerName = &cs.Name
	}
	return &v1alpha1.CNClaim{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   cs.Namespace,
			Name:        cs.Name + "-" + id,
			Labels:      labels,
			Annotations: tpl.Annotations,
		},
		Spec: tpl.Spec,
	}
}

type ClaimAndPod struct {
	Claim *v1alpha1.CNClaim
	Pod   *corev1.Pod
}

func (c *ClaimAndPod) scoreHasPod() int {
	if c.Pod != nil {
		return 1
	}
	return 0
}

func (r *Actor) scaleIn(ctx *recon.Context[*v1alpha1.CNClaimSet], oc *ownedClaims, count int) error {
	var cps []ClaimAndPod
	for i := range oc.owned {
		c := oc.owned[i]
		pod, err := getClaimedPod(ctx, &c)
		if err != nil {
			return errors.WrapPrefix(err, "error get claimed Pod", 0)
		}
		cps = append(cps, ClaimAndPod{
			Claim: &c,
			Pod:   pod,
		})
	}
	if count >= len(cps) {
		// simply delete all claims
		count = len(cps)
	} else {
		sortClaimsToDelete(cps)
		ctx.Log.Info("sort claims to scale-in", "sorted", cps)
	}
	var i int
	for ; i < count; i++ {
		c := cps[i].Claim
		if err := ctx.Delete(c); err != nil {
			if !apierrors.IsNotFound(err) {
				return errors.Wrap(err, 0)
			}
		}
	}
	var left []v1alpha1.CNClaim
	for ; i < len(cps); i++ {
		left = append(left, *cps[i].Claim)
	}
	oc.owned = left
	return nil
}

func (r *Actor) Finalize(ctx *recon.Context[*v1alpha1.CNClaimSet]) (bool, error) {
	oc, err := listOwnedClaims(ctx, ctx.Client, ctx.Obj)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return true, nil
		}
		return false, errors.WrapPrefix(err, "error list claims", 0)
	}
	if len(oc.owned) == 0 && len(oc.lost) == 0 {
		return true, nil
	}
	for _, c := range append(oc.owned, oc.lost...) {
		if err := ctx.Delete(&c); err != nil && !apierrors.IsNotFound(err) {
			return false, errors.Wrap(err, 0)
		}
	}
	return false, nil
}

func getClaimedPod(cli recon.KubeClient, c *v1alpha1.CNClaim) (*corev1.Pod, error) {
	if c.Spec.PodName == "" || c.Status.Phase == v1alpha1.CNClaimPhaseLost {
		return nil, nil
	}
	pod := &corev1.Pod{}
	err := cli.Get(types.NamespacedName{Namespace: c.Namespace, Name: c.Spec.PodName}, pod)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, errors.WrapPrefix(err, "error get claimed Pod", 0)
	}
	return pod, nil
}

func sortClaimsToDelete(cps []ClaimAndPod) {
	slices.SortFunc(cps, claimDeletionOrder)
}

func claimDeletionOrder(a, b ClaimAndPod) int {
	// 1. delete pending/lost Claim first
	hasPod := a.scoreHasPod() - b.scoreHasPod()
	if hasPod != 0 {
		return hasPod
	}
	// 2. delete outdated Claim < updated Claim
	if claimPhaseToOrdinal[a.Claim.Status.Phase] != claimPhaseToOrdinal[b.Claim.Status.Phase] {
		return claimPhaseToOrdinal[a.Claim.Status.Phase] - claimPhaseToOrdinal[b.Claim.Status.Phase]
	}

	// 3. there is a tie, two cases:
	if a.scoreHasPod() == 1 {
		// has Pod, compare Pod deletion order
		res := podDeletionOrder(a.Pod, b.Pod)
		return res
	}
	// both no Pod, compare creation time, delete newer one first
	return -int(a.Claim.CreationTimestamp.Sub(b.Claim.CreationTimestamp.Time).Seconds())
}

// TODO(aylei): Pod deletion order should be fine-tuned
func podDeletionOrder(a, b *corev1.Pod) int {
	// 1. UnScheduled < Scheduled
	if a.Spec.NodeName != b.Spec.NodeName && (a.Spec.NodeName == "" || b.Spec.NodeName == "") {
		return len(a.Spec.NodeName) - len(b.Spec.NodeName)
	}
	// 2. Pending < Failed < Running
	if podPhaseToOrdinal[a.Status.Phase] != podPhaseToOrdinal[b.Status.Phase] {
		return podPhaseToOrdinal[a.Status.Phase] - podPhaseToOrdinal[b.Status.Phase]
	}
	// 3. NotReady < Ready
	if util.IsPodReady(a) != util.IsPodReady(b) {
		if util.IsPodReady(a) {
			return 1
		}
		return -1
	}
	// 4. deletion-cost
	aCost := podDeletionCost(a)
	bCost := podDeletionCost(b)
	if aCost != bCost {
		if aCost-bCost < 0 {
			return -1
		}
		return 1
	}
	// delete the Claim with newer Pod first
	return -int(a.CreationTimestamp.Sub(b.CreationTimestamp.Time).Seconds())
}

func genAvailableIds(num int, used sets.Set[string]) []string {
	var ret []string
	for i := 0; i < num; i++ {
		id := genAvailableID(used)
		ret = append(ret, id)
		used.Insert(id)
	}
	return ret
}

func genAvailableID(used sets.Set[string]) string {
	var id string
	for {
		id = rand.String(LengthOfInstanceID)
		if !used.Has(id) {
			break
		}
	}
	return id
}

func podDeletionCost(pod *corev1.Pod) int64 {
	if value, exist := pod.Annotations[common.DeletionCostAnno]; exist {
		i, err := strconv.ParseInt(value, 10, 32)
		if err != nil {
			return 0
		}
		return i
	}
	return 0
}

func cleanClaims(ctx recon.KubeClient, cs []v1alpha1.CNClaim) error {
	for i := range cs {
		if cs[i].DeletionTimestamp != nil {
			// already deleted
			continue
		}
		if err := ctx.Delete(&cs[i]); err != nil {
			if !apierrors.IsNotFound(err) {
				return errors.WrapPrefix(err, "error delete lost Claim", 0)
			}
		}
	}
	return nil
}

func listOwnedClaims(ctx context.Context, cli client.Client, s *v1alpha1.CNClaimSet) (*ownedClaims, error) {
	cList := &v1alpha1.CNClaimList{}
	err := cli.List(ctx, cList, client.InNamespace(s.Namespace), client.MatchingLabelsSelector{
		Selector: common.MustAsSelector(s.Spec.Selector),
	})
	if err != nil {
		return nil, err
	}
	res := &ownedClaims{}
	for i := range cList.Items {
		c := cList.Items[i]
		if c.Status.Phase == v1alpha1.CNClaimPhaseLost ||
			c.Labels[ClaimInstanceIDLabel] == "" ||
			c.DeletionTimestamp != nil {
			res.lost = append(res.lost, c)
		} else {
			res.owned = append(res.owned, c)
		}
	}
	return res, nil
}

func (r *Actor) Start(mgr manager.Manager) error {
	return recon.Setup[*v1alpha1.CNClaimSet](&v1alpha1.CNClaimSet{}, "cn-claimset-manager", mgr, r, recon.WithBuildFn(func(b *builder.Builder) {
		// watch all updates to owned claims
		b.Owns(&v1alpha1.CNClaim{}, builder.WithPredicates(predicate.ResourceVersionChangedPredicate{}))
	}))
}
