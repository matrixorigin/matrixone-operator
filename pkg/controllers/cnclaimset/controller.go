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
	recon "github.com/matrixorigin/controller-runtime/pkg/reconciler"
	"github.com/matrixorigin/controller-runtime/pkg/util"
	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	"github.com/matrixorigin/matrixone-operator/pkg/controllers/common"
	"github.com/pkg/errors"
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

type ownedClaims struct {
	active []v1alpha1.CNClaim
	lost   []v1alpha1.CNClaim
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
		return errors.Wrap(err, "error filter claims")
	}
	if int32(len(oc.active)) != s.Status.Replicas {
		// check whether the cache is in sync
		realC, err := listOwnedClaims(ctx, r.ClientNoCache, s)
		if err != nil {
			return errors.Wrap(err, "error list claims directly")
		}
		if len(oc.active) != len(realC.active) || len(oc.lost) != len(realC.lost) {
			// simply requeue to wait cache sync, since we heavily rely on cache in the following reconciliation
			ctx.Log.Info("cache not synced, wait",
				"cached active", len(oc.active),
				"real active", len(realC.active),
				"cached lost", len(oc.lost),
				"real lost", len(realC.lost))
			return recon.ErrReSync("wait cache sync", time.Second)
		}
	}
	if err := r.scale(ctx, oc); err != nil {
		return errors.Wrap(err, "error scale cnclaimset")
	}
	// clean lost claims
	if err := cleanClaims(ctx, oc.lost); err != nil {
		return errors.Wrap(err, "clean filter out claims")
	}
	// collect status
	var claimStatuses []v1alpha1.CNClaimStatus
	for _, c := range oc.active {
		claimStatuses = append(claimStatuses, c.Status)
	}
	s.Status.Replicas = int32(len(oc.active))
	s.Status.Claims = claimStatuses
	return nil
}

func (r *Actor) scale(ctx *recon.Context[*v1alpha1.CNClaimSet], oc *ownedClaims) error {
	s := ctx.Obj
	current := len(oc.active)
	ctx.Log.Info("scale claimset", "desiredReplicas", s.Spec.Replicas, "currentReplicas", current)
	if s.Spec.Replicas > int32(current) {
		return r.scaleOut(ctx, oc, int(s.Spec.Replicas)-current)
	} else if s.Spec.Replicas < int32(current) {
		return r.scaleIn(ctx, oc, current-int(s.Spec.Replicas))
	}
	return nil
}

func (r *Actor) scaleOut(ctx *recon.Context[*v1alpha1.CNClaimSet], oc *ownedClaims, count int) error {
	used := sets.New[string]()
	for _, c := range oc.active {
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
			return errors.Wrap(err, "error create new claim")
		}
		oc.active = append(oc.active, *claim)
	}
	return nil
}

func makeClaim(cs *v1alpha1.CNClaimSet, id string) *v1alpha1.CNClaim {
	tpl := cs.Spec.Template
	labels := tpl.Labels
	labels[ClaimInstanceIDLabel] = id
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

type claimAndPod struct {
	claim *v1alpha1.CNClaim
	pod   *corev1.Pod
}

func (c *claimAndPod) scoreHasPod() int {
	if c.pod != nil {
		return 1
	}
	return 0
}

func (r *Actor) scaleIn(ctx *recon.Context[*v1alpha1.CNClaimSet], oc *ownedClaims, count int) error {
	var cps []claimAndPod
	for i := range oc.active {
		c := oc.active[i]
		pod, err := getClaimedPod(ctx, &c)
		if err != nil {
			return errors.Wrap(err, "error get claimed pod")
		}
		cps = append(cps, claimAndPod{
			claim: &c,
			pod:   pod,
		})
	}
	if count >= len(cps) {
		// simply delete all claims
		count = len(cps)
	} else {
		slices.SortFunc(cps, claimDeletionOrder)
	}
	var i int
	for ; i < count; i++ {
		c := cps[i].claim
		if err := ctx.Delete(c); err != nil {
			if !apierrors.IsNotFound(err) {
				return errors.Wrapf(err, "error scale-in claim %s", c.Name)
			}
		}
	}
	var left []v1alpha1.CNClaim
	for ; i < len(cps); i++ {
		left = append(left, *cps[i].claim)
	}
	oc.active = left
	return nil
}

func (r *Actor) Finalize(ctx *recon.Context[*v1alpha1.CNClaimSet]) (bool, error) {
	oc, err := listOwnedClaims(ctx, ctx.Client, ctx.Obj)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return true, nil
		}
		return false, errors.Wrap(err, "error list claims")
	}
	if len(oc.active) == 0 && len(oc.lost) == 0 {
		return true, nil
	}
	for _, c := range append(oc.active, oc.lost...) {
		if err := ctx.Delete(&c); err != nil && !apierrors.IsNotFound(err) {
			return false, errors.Wrapf(err, "error delete claim %s", c.Name)
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
		return nil, errors.Wrap(err, "error get claimed pod")
	}
	return pod, nil
}

func claimDeletionOrder(a, b claimAndPod) int {
	// 1. delete pending/lost claim first
	hasPod := a.scoreHasPod() - b.scoreHasPod()
	if hasPod != 0 {
		return hasPod
	}
	// 2. there is a tie, two cases:
	if a.scoreHasPod() == 1 {
		// has pod, compare pod deletion order
		return podDeletionOrder(a.pod, b.pod)
	}
	// both no pod, compare creation time, delete newer one first
	return -(a.claim.CreationTimestamp.Second() - b.claim.CreationTimestamp.Second())
}

// TODO(aylei): pod deletion order should be fine-tuned
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
	return -(a.CreationTimestamp.Second() - b.CreationTimestamp.Second())
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
		if err := ctx.Delete(&cs[i]); err != nil {
			if !apierrors.IsNotFound(err) {
				return errors.Wrap(err, "error delete lost claim")
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
		if c.Status.Phase == v1alpha1.CNClaimPhaseLost || c.Labels[ClaimInstanceIDLabel] == "" {
			res.lost = append(res.lost, c)
		} else {
			res.active = append(res.active, c)
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
