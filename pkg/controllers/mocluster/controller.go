// Copyright 2023 Matrix Origin
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

package mocluster

import (
	"fmt"
	recon "github.com/matrixorigin/controller-runtime/pkg/reconciler"
	"github.com/matrixorigin/controller-runtime/pkg/util"
	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	"github.com/matrixorigin/matrixone-operator/pkg/controllers/common"
	"github.com/matrixorigin/matrixone-operator/pkg/utils"
	kruisepolicy "github.com/openkruise/kruise-api/policy/v1alpha1"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"time"
)

const (
	resyncAfter = 15 * time.Second

	usernameKey = "username"
	passwordKey = "password"

	maxUnavailablePod = 1
)

var _ recon.Actor[*v1alpha1.MatrixOneCluster] = &MatrixOneClusterActor{}

type MatrixOneClusterActor struct{}

func (r *MatrixOneClusterActor) Observe(ctx *recon.Context[*v1alpha1.MatrixOneCluster]) (recon.Action[*v1alpha1.MatrixOneCluster], error) {
	mo := ctx.Obj

	maxUnavailable := intstr.FromInt(maxUnavailablePod)
	unavailableBudget := &kruisepolicy.PodUnavailableBudget{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: mo.Namespace,
			Name:      mo.Name,
		},
	}
	if err := recon.CreateOwnedOrUpdate(ctx, unavailableBudget, func() error {
		unavailableBudget.Spec.Selector = &metav1.LabelSelector{
			MatchLabels: map[string]string{
				common.MatrixoneClusterLabelKey: mo.Name,
			},
		}
		unavailableBudget.Spec.MaxUnavailable = &maxUnavailable
		return nil
	}); err != nil {
		return nil, errors.Wrap(err, "sync cluster unavailable budget")
	}

	// sync specs
	ls := &v1alpha1.LogSet{
		ObjectMeta: v1alpha1.LogSetKey(mo),
	}
	dn := &v1alpha1.DNSet{
		ObjectMeta: v1alpha1.DNSetKey(mo),
		Deps:       v1alpha1.DNSetDeps{LogSetRef: ls.AsDependency()},
	}
	_, err := utils.CreateOwnedOrUpdate(ctx, ls, func() error {
		ls.Spec = mo.Spec.LogService
		setPodSetDefault(&ls.Spec.PodSet, mo)
		setOverlay(&ls.Spec.Overlay, mo)
		ls.Spec.Image = mo.LogSetImage()
		return nil
	})
	if err != nil {
		return nil, errors.Wrap(err, "sync LogSet")
	}
	_, err = utils.CreateOwnedOrUpdate(ctx, dn, func() error {
		dn.Spec = mo.Spec.DN
		setPodSetDefault(&dn.Spec.PodSet, mo)
		setOverlay(&dn.Spec.Overlay, mo)
		dn.Spec.Image = mo.DnSetImage()
		return nil
	})
	if err != nil {
		return nil, errors.Wrap(err, "sync DNSet")
	}

	cnGroups := append([]v1alpha1.CNGroup{}, mo.Spec.CNGroups...)
	// append TP and AP cnset for backward compatibility
	if mo.Spec.TP != nil {
		spec := *mo.Spec.TP
		// for backward compatibility, the TP CN may store UUID in cache volume and check consistency
		if spec.DNSBasedIdentity == nil {
			spec.DNSBasedIdentity = pointer.Bool(false)
		}
		cnGroups = append(cnGroups, v1alpha1.CNGroup{Name: "tp", CNSetSpec: spec})
	}
	if mo.Spec.AP != nil {
		cnGroups = append(cnGroups, v1alpha1.CNGroup{Name: "ap", CNSetSpec: *mo.Spec.AP})
	}
	desiredCNSets := map[string]bool{}
	for _, g := range cnGroups {
		cnSetName := fmt.Sprintf("%s-%s", mo.Name, g.Name)
		desiredCNSets[cnSetName] = true
		tpl := &v1alpha1.CNSet{
			ObjectMeta: common.CNSetKey(mo, cnSetName),
			Deps: v1alpha1.CNSetDeps{
				LogSetRef: ls.AsDependency(),
				DNSet:     &v1alpha1.DNSet{ObjectMeta: v1alpha1.DNSetKey(mo)},
			},
		}
		_, err = utils.CreateOwnedOrUpdate(ctx, tpl, func() error {
			// ensure label for legacy CNSet
			if tpl.Labels == nil {
				tpl.Labels = map[string]string{}
			}
			tpl.Labels[common.MatrixoneClusterLabelKey] = mo.Name
			tpl.Spec = g.CNSetSpec
			if mo.Spec.Proxy != nil {
				if tpl.Spec.Config == nil {
					tpl.Spec.Config = v1alpha1.NewTomlConfig(map[string]interface{}{})
				}
				tpl.Spec.Config.Set([]string{"cn", "frontend", "proxy-enabled"}, true)
			}
			setPodSetDefault(&tpl.Spec.PodSet, mo)
			setOverlay(&tpl.Spec.Overlay, mo)
			tpl.Spec.Image = common.CNSetImage(mo, &g.CNSetSpec)
			return nil
		})
		if err != nil {
			return nil, errors.Wrapf(err, "sync CNSet %s", g.Name)
		}
	}

	// GC no longer needed CNSets
	groupStatus := v1alpha1.CNGroupStatus{DesiredGroups: len(cnGroups)}
	csList := &v1alpha1.CNSetList{}
	cnSelector := map[string]string{common.MatrixoneClusterLabelKey: mo.Name}
	if err := ctx.List(csList, client.InNamespace(mo.Namespace), client.MatchingLabels(cnSelector)); err != nil {
		return nil, errors.Wrap(err, "error list current CNSets of the cluster")
	}
	for i := range csList.Items {
		cnSet := csList.Items[i]
		if !desiredCNSets[cnSet.Name] {
			ctx.Log.V(4).Info("delete CNSet as it is no longer needed", "name", cnSet.Name)
			if err := ctx.Delete(&cnSet); err != nil {
				return nil, errors.Wrap(err, "error delete cnset")
			}
			continue
		}
		if recon.IsReady(&cnSet) {
			groupStatus.ReadyGroups++
		}
		if recon.IsSynced(&cnSet) {
			groupStatus.SyncedGroups++
		}
	}
	mo.Status.CNGroupStatus = groupStatus

	if mo.Spec.WebUI != nil {
		webui := &v1alpha1.WebUI{
			ObjectMeta: v1alpha1.WebUIKey(mo),
		}
		if err := recon.CreateOwnedOrUpdate(ctx, webui, func() error {
			webui.Spec = *mo.Spec.WebUI
			return nil
		}); err != nil {
			return nil, errors.Wrap(err, "sync webUI")
		}
		mo.Status.Webui = &webui.Status
	}

	if mo.Spec.Proxy != nil {
		proxy := &v1alpha1.ProxySet{
			ObjectMeta: v1alpha1.ProxyKey(mo),
			Deps: v1alpha1.ProxySetDeps{
				LogSetRef: ls.AsDependency(),
			},
		}
		if err := recon.CreateOwnedOrUpdate(ctx, proxy, func() error {
			proxy.Spec = *mo.Spec.Proxy
			setPodSetDefault(&proxy.Spec.PodSet, mo)
			setOverlay(&proxy.Spec.Overlay, mo)
			proxy.Spec.Image = mo.ProxySetImage()
			return nil
		}); err != nil {
			return nil, errors.Wrap(err, "sync proxy")
		}

		mo.Status.Proxy = &proxy.Status
	}

	// collect status
	mo.Status.LogService = &ls.Status
	mo.Status.DN = &dn.Status
	mo.Status.Phase = "NotReady"
	mo.Status.ConditionalStatus.SetCondition(syncedCondition(mo))

	subResourcesReady := readyCondition(mo)

	if mo.Status.CredentialRef == nil {
		// cluster not initialized
		if subResourcesReady.Status == metav1.ConditionFalse {
			// the underlying sets are not ready, wait
			mo.Status.ConditionalStatus.SetCondition(subResourcesReady)
			return nil, recon.ErrReSync("wait cluster ready to complete initialization", resyncAfter)
		}
		// checkpoint the status before the initialize action
		mo.Status.ConditionalStatus.SetCondition(metav1.Condition{
			Type:   recon.ConditionTypeReady,
			Status: metav1.ConditionFalse,
			Reason: "ClusterNotInitialized",
		})
		mo.Status.Phase = "Initializing"
		return r.Initialize, nil
	}
	mo.Status.ConditionalStatus.SetCondition(subResourcesReady)
	if subResourcesReady.Status == metav1.ConditionTrue {
		mo.Status.Phase = "Ready"
	}

	if recon.IsReady(&mo.Status) {
		return nil, nil
	}
	return nil, recon.ErrReSync("matrixone cluster is not ready", resyncAfter)
}

func setPodSetDefault(ps *v1alpha1.PodSet, mo *v1alpha1.MatrixOneCluster) {
	if ps.NodeSelector == nil {
		ps.NodeSelector = mo.Spec.NodeSelector
	}
	if ps.TopologyEvenSpread == nil {
		ps.TopologyEvenSpread = mo.Spec.TopologyEvenSpread
	}
}

func setOverlay(o **v1alpha1.Overlay, mo *v1alpha1.MatrixOneCluster) {
	if *o == nil {
		*o = &v1alpha1.Overlay{}
	}
	if (*o).ImagePullPolicy == nil && mo.Spec.ImagePullPolicy != nil {
		(*o).ImagePullPolicy = mo.Spec.ImagePullPolicy
	}
	if (*o).PodLabels == nil {
		(*o).PodLabels = map[string]string{}
	}
	(*o).PodLabels[common.MatrixoneClusterLabelKey] = mo.Name
}

// Initialize the MO cluster
func (r *MatrixOneClusterActor) Initialize(ctx *recon.Context[*v1alpha1.MatrixOneCluster]) error {
	// 1. generate the secret
	sec := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ctx.Obj.Namespace,
			Name:      credentialName(ctx.Obj),
		},
		StringData: map[string]string{
			// TODO: avoid using hardcoded username password
			usernameKey: "dump",
			passwordKey: "111",
		},
	}
	if err := ctx.CreateOwned(sec); err != nil {
		return err
	}
	// 2. initialize the cluster
	// TODO: initialize users that using the above secret after MO support ALTER USER

	// 3. update the status
	ctx.Obj.Status.CredentialRef = &corev1.LocalObjectReference{Name: sec.Name}
	return ctx.UpdateStatus(ctx.Obj)
}

func readyCondition(mo *v1alpha1.MatrixOneCluster) metav1.Condition {
	c := metav1.Condition{Type: recon.ConditionTypeReady}
	switch {
	case !recon.IsReady(mo.Status.LogService):
		c.Status = metav1.ConditionFalse
		c.Reason = "LogServiceNotReady"
	case !recon.IsReady(mo.Status.DN):
		c.Status = metav1.ConditionFalse
		c.Reason = "DNSetNotReady"
	case !mo.Status.CNGroupStatus.Ready():
		c.Status = metav1.ConditionFalse
		c.Reason = "SomeCNSetsAreNotReady"
	case mo.Spec.Proxy != nil && !recon.IsReady(mo.Status.Proxy):
		c.Status = metav1.ConditionFalse
		c.Reason = "ProxySetNotReady"
	default:
		c.Status = metav1.ConditionTrue
		c.Reason = "AllSetsReady"
	}
	return c
}

func syncedCondition(mo *v1alpha1.MatrixOneCluster) metav1.Condition {
	c := metav1.Condition{Type: recon.ConditionTypeSynced}
	switch {
	case !recon.IsSynced(mo.Status.LogService):
		c.Status = metav1.ConditionFalse
		c.Reason = "LogServiceNotSynced"
	case !recon.IsSynced(mo.Status.DN):
		c.Status = metav1.ConditionFalse
		c.Reason = "DNSetNotSynced"
	case !mo.Status.CNGroupStatus.Synced():
		c.Status = metav1.ConditionFalse
		c.Reason = "SomeCNSetsAreNotSynced"
	case mo.Spec.Proxy != nil && !recon.IsSynced(mo.Status.Proxy):
		c.Status = metav1.ConditionFalse
		c.Reason = "ProxyNotSynced"
	default:
		c.Status = metav1.ConditionTrue
		c.Reason = "AllSetsSynced"
	}
	return c
}

func (r *MatrixOneClusterActor) Finalize(ctx *recon.Context[*v1alpha1.MatrixOneCluster]) (bool, error) {
	mo := ctx.Obj
	err := ctx.Client.DeleteAllOf(ctx, &v1alpha1.CNSet{}, client.InNamespace(mo.Namespace), client.MatchingLabels(
		map[string]string{common.MatrixoneClusterLabelKey: mo.Name},
	))
	if err := util.Ignore(apierrors.IsNotFound, err); err != nil {
		return false, err
	}
	objs := []client.Object{
		&v1alpha1.LogSet{ObjectMeta: v1alpha1.LogSetKey(mo)},
		&v1alpha1.DNSet{ObjectMeta: v1alpha1.DNSetKey(mo)},
		&v1alpha1.WebUI{ObjectMeta: v1alpha1.WebUIKey(mo)},
		&v1alpha1.WebUI{ObjectMeta: v1alpha1.ProxyKey(mo)},
	}
	existAny := false
	for _, obj := range objs {
		exist, err := ctx.Exist(client.ObjectKeyFromObject(obj), obj)
		if err != nil {
			return false, err
		}
		if exist {
			if err := util.Ignore(apierrors.IsNotFound, ctx.Delete(obj)); err != nil {
				return false, err
			}
		}
		existAny = existAny || exist
	}
	return !existAny, nil
}

func (r *MatrixOneClusterActor) Reconcile(mgr manager.Manager) error {
	return recon.Setup[*v1alpha1.MatrixOneCluster](&v1alpha1.MatrixOneCluster{}, "matrixonecluster", mgr, r,
		recon.WithBuildFn(func(b *builder.Builder) {
			b.Owns(&v1alpha1.LogSet{}).
				Owns(&v1alpha1.DNSet{}).
				Owns(&v1alpha1.CNSet{}).
				Owns(&v1alpha1.WebUI{}).
				Owns(&v1alpha1.ProxySet{})
		}))
}

func credentialName(mo *v1alpha1.MatrixOneCluster) string {
	return fmt.Sprintf("%s-credential", mo.Name)
}

func updatingCondition() metav1.Condition {
	return metav1.Condition{
		Type:   recon.ConditionTypeSynced,
		Status: metav1.ConditionFalse,
		Reason: "Updating",
	}
}
