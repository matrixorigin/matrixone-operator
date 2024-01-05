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
	"strconv"
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

// Actor reconciles CNClaimSet
type Actor struct {
}

func NewActor() *Actor {
	return &Actor{}
}

func (r *Actor) Observe(ctx *recon.Context[*v1alpha1.CNClaimSet]) (recon.Action[*v1alpha1.CNClaimSet], error) {
	return nil, r.Sync(ctx)
}

func (r *Actor) Sync(ctx *recon.Context[*v1alpha1.CNClaimSet]) error {
	s := ctx.Obj
	filteredClaims, filterOutClaims, err := listClaims(ctx, s)
	if err != nil {
		return errors.Wrap(err, "error filter claims")
	}
	if s.Spec.Replicas > int32(len(filteredClaims)) {
		if err := r.scaleOut(ctx, filteredClaims, filterOutClaims, int(s.Spec.Replicas)-len(filterOutClaims)); err != nil {
			return errors.Wrap(err, "error scale out cn claim set")
		}
	} else if s.Spec.Replicas < int32(len(filteredClaims)) {
		if err := r.scaleIn(ctx, filteredClaims, len(filterOutClaims)-int(s.Spec.Replicas)); err != nil {
			return errors.Wrap(err, "error scale out cn claim set")
		}
	}
	// clean lost claims
	if err := cleanClaims(ctx, filterOutClaims); err != nil {
		return errors.Wrap(err, "clean filter out claims")
	}
	// collect status
	s.Status.Replicas = int32(len(filteredClaims))
	var claimStatuses []v1alpha1.CNClaimStatus
	for _, c := range filteredClaims {
		claimStatuses = append(claimStatuses, c.Status)
	}
	return nil
}

func (r *Actor) scaleOut(ctx *recon.Context[*v1alpha1.CNClaimSet], filtered []v1alpha1.CNClaim, filteredOut []v1alpha1.CNClaim, count int) error {
	used := sets.New[string]()
	for _, c := range filtered {
		used.Insert(c.Labels[ClaimInstanceIDLabel])
	}
	for _, c := range filteredOut {
		used.Insert(c.Labels[ClaimInstanceIDLabel])
	}
	ids := genAvailableIds(count, used)
	for _, id := range ids {
		err := ctx.Create(makeClaim(ctx.Obj, id))
		if err != nil {
			return errors.Wrap(err, "error create new claim")
		}
	}
	return nil
}

func makeClaim(cs *v1alpha1.CNClaimSet, id string) *v1alpha1.CNClaim {
	return &v1alpha1.CNClaim{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: cs.Namespace,
			Name:      cs.Name + "-" + id,
			Labels: map[string]string{
				ClaimInstanceIDLabel: id,
			},
		},
		Spec: *cs.Spec.Template.Spec.DeepCopy(),
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

func (r *Actor) scaleIn(ctx *recon.Context[*v1alpha1.CNClaimSet], filtered []v1alpha1.CNClaim, count int) error {
	var cps []claimAndPod
	for i := range filtered {
		c := filtered[i]
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
	for i := 0; i < count; i++ {
		c := cps[i].claim
		if err := ctx.Delete(c); err != nil {
			if !apierrors.IsNotFound(err) {
				return errors.Wrapf(err, "error scale-in claim %s", c.Name)
			}
		}
	}
	return nil
}

func (r *Actor) Finalize(ctx *recon.Context[*v1alpha1.CNClaimSet]) (bool, error) {
	c1, c2, err := listClaims(ctx, ctx.Obj)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return true, nil
		}
		return false, errors.Wrap(err, "error list claims")
	}
	if len(c1) == 0 && len(c2) == 0 {
		return true, nil
	}
	for _, c := range append(c1, c2...) {
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

func listClaims(cli recon.KubeClient, s *v1alpha1.CNClaimSet) (filtered []v1alpha1.CNClaim, filterOut []v1alpha1.CNClaim, err error) {
	cList := &v1alpha1.CNClaimList{}
	err = cli.List(cList, client.InNamespace(s.Namespace), client.MatchingLabelsSelector{
		Selector: common.MustAsSelector(s.Spec.Selector),
	})
	if err != nil {
		return
	}
	for i := range cList.Items {
		c := cList.Items[i]
		if c.Status.Phase == v1alpha1.CNClaimPhaseLost || c.Labels[ClaimInstanceIDLabel] == "" {
			filterOut = append(filterOut, c)
		} else {
			filtered = append(filtered, c)
		}
	}
	return
}

func (r *Actor) Start(mgr manager.Manager) error {
	return recon.Setup[*v1alpha1.CNClaimSet](&v1alpha1.CNClaimSet{}, "cn-claimset-manager", mgr, r, recon.WithBuildFn(func(b *builder.Builder) {
		b.Owns(&v1alpha1.CNClaim{})
	}))
}
