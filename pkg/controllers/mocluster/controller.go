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
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
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
		Spec: v1alpha1.LogSetSpec{
			LogSetBasic: v1alpha1.LogSetBasic{
				InitialConfig: mo.Spec.LogService.InitialConfig,
			},
		},
	}
	dn := &v1alpha1.DNSet{
		ObjectMeta: v1alpha1.DNSetKey(mo),
		Deps:       v1alpha1.DNSetDeps{LogSetRef: ls.AsDependency()},
	}
	tp := &v1alpha1.CNSet{
		ObjectMeta: v1alpha1.TPSetKey(mo),
		Deps:       v1alpha1.CNSetDeps{LogSetRef: ls.AsDependency()},
	}
	result, err := utils.CreateOwnedOrUpdate(ctx, ls, func() error {
		ls.Spec.LogSetBasic = mo.Spec.LogService
		setPodSetDefault(&ls.Spec.LogSetBasic.PodSet, mo)
		setOverlay(&ls.Spec.Overlay, mo)
		ls.Spec.Image = mo.LogSetImage()
		return nil
	})
	if err != nil {
		return nil, errors.Wrap(err, "sync LogSet")
	}
	if result == controllerutil.OperationResultUpdated {
		// mark the logset as NotSynced to avoid race condition
		ls.SetCondition(updatingCondition())
		if err := ctx.UpdateStatus(ls); err != nil {
			return nil, err
		}
	}
	result, err = utils.CreateOwnedOrUpdate(ctx, dn, func() error {
		dn.Spec.DNSetBasic = mo.Spec.DN
		setPodSetDefault(&dn.Spec.DNSetBasic.PodSet, mo)
		setOverlay(&dn.Spec.Overlay, mo)
		dn.Spec.Image = mo.DnSetImage()
		dn.Deps.LogSet = &v1alpha1.LogSet{ObjectMeta: v1alpha1.LogSetKey(mo)}
		return nil
	})
	if err != nil {
		return nil, errors.Wrap(err, "sync DNSet")
	}
	result, err = utils.CreateOwnedOrUpdate(ctx, tp, func() error {
		tp.Spec.CNSetBasic = mo.Spec.TP
		setPodSetDefault(&tp.Spec.CNSetBasic.PodSet, mo)
		setOverlay(&tp.Spec.Overlay, mo)
		tp.Spec.Image = mo.TpSetImage()
		tp.Deps.LogSet = &v1alpha1.LogSet{ObjectMeta: v1alpha1.LogSetKey(mo)}
		tp.Deps.DNSet = &v1alpha1.DNSet{ObjectMeta: v1alpha1.DNSetKey(mo)}
		return nil
	})
	if err != nil {
		return nil, errors.Wrap(err, "sync TP CNSet")
	}
	if mo.Spec.AP != nil {
		ap := &v1alpha1.CNSet{
			ObjectMeta: v1alpha1.APSetKey(mo),
			Deps:       v1alpha1.CNSetDeps{LogSetRef: ls.AsDependency()},
		}
		if err := recon.CreateOwnedOrUpdate(ctx, ap, func() error {
			ap.Spec.CNSetBasic = *mo.Spec.AP
			setPodSetDefault(&ap.Spec.CNSetBasic.PodSet, mo)
			setOverlay(&ap.Spec.Overlay, mo)
			ap.Spec.Image = mo.ApSetImage()
			ap.Deps.LogSet = &v1alpha1.LogSet{ObjectMeta: v1alpha1.LogSetKey(mo)}
			ap.Deps.DNSet = &v1alpha1.DNSet{ObjectMeta: v1alpha1.DNSetKey(mo)}
			return nil
		}); err != nil {
			return nil, errors.Wrap(err, "sync AP CNSet")
		}
		mo.Status.AP = &ap.Status
	}

	if mo.Spec.WebUI != nil {
		webui := &v1alpha1.WebUI{
			ObjectMeta: v1alpha1.WebUIKey(mo),
			Deps: v1alpha1.WebUIDeps{
				CNSet: &v1alpha1.CNSet{ObjectMeta: v1alpha1.TPSetKey(mo)},
			},
		}
		if err := recon.CreateOwnedOrUpdate(ctx, webui, func() error {
			webui.Spec.WebUIBasic = *mo.Spec.WebUI
			return nil
		}); err != nil {
			return nil, errors.Wrap(err, "sync webUI")
		}
		mo.Status.Webui = &webui.Status
	}

	// collect status
	mo.Status.LogService = &ls.Status
	mo.Status.DN = &dn.Status
	mo.Status.TP = &tp.Status
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
	if mo.Spec.ImagePullPolicy != nil {
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
	case !recon.IsReady(mo.Status.TP):
		c.Status = metav1.ConditionFalse
		c.Reason = "TPSetNotReady"
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
	case !recon.IsSynced(mo.Status.TP):
		c.Status = metav1.ConditionFalse
		c.Reason = "TPSetNotSynced"
	default:
		c.Status = metav1.ConditionTrue
		c.Reason = "AllSetsSynced"
	}
	return c
}

func (r *MatrixOneClusterActor) Finalize(ctx *recon.Context[*v1alpha1.MatrixOneCluster]) (bool, error) {
	mo := ctx.Obj
	objs := []client.Object{
		&v1alpha1.LogSet{ObjectMeta: v1alpha1.LogSetKey(mo)},
		&v1alpha1.DNSet{ObjectMeta: v1alpha1.DNSetKey(mo)},
		&v1alpha1.CNSet{ObjectMeta: v1alpha1.TPSetKey(mo)},
		&v1alpha1.CNSet{ObjectMeta: v1alpha1.APSetKey(mo)},
		&v1alpha1.WebUI{ObjectMeta: v1alpha1.WebUIKey(mo)},
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
				Owns(&v1alpha1.WebUI{})
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
