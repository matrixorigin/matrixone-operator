// Copyright 2022 Matrix Origin
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
	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	recon "github.com/matrixorigin/matrixone-operator/runtime/pkg/reconciler"
	"github.com/matrixorigin/matrixone-operator/runtime/pkg/util"
	"go.uber.org/multierr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

var _ recon.Actor[*v1alpha1.MatrixOneCluster] = &MatrixOneClusterActor{}

type MatrixOneClusterActor struct{}

func (r *MatrixOneClusterActor) Observe(ctx *recon.Context[*v1alpha1.MatrixOneCluster]) (recon.Action[*v1alpha1.MatrixOneCluster], error) {
	mo := ctx.Obj

	// sync specs
	ls := &v1alpha1.LogSet{
		ObjectMeta: logSetKey(mo),
		Spec: v1alpha1.LogSetSpec{
			LogSetBasic: v1alpha1.LogSetBasic{
				InitialConfig: mo.Spec.LogService.InitialConfig,
			},
		},
	}
	dn := &v1alpha1.DNSet{
		ObjectMeta: dnSetKey(mo),
		Deps:       v1alpha1.DNSetDeps{LogSetRef: ls.AsDependency()},
	}
	tp := &v1alpha1.CNSet{
		ObjectMeta: tpSetKey(mo),
		Deps:       v1alpha1.CNSetDeps{LogSetRef: ls.AsDependency()},
	}

	errs := multierr.Combine(
		recon.CreateOwnedOrUpdate(ctx, ls, func() error {
			ls.Spec.LogSetBasic.PodSet = mo.Spec.LogService.PodSet
			ls.Spec.LogSetBasic.SharedStorage = mo.Spec.LogService.SharedStorage
			ls.Spec.LogSetBasic.Volume = mo.Spec.LogService.Volume
			ls.Spec.Image = mo.LogSetImage()
			return nil
		}),
		recon.CreateOwnedOrUpdate(ctx, dn, func() error {
			dn.Spec.DNSetBasic = mo.Spec.DN
			dn.Spec.Image = mo.DnSetImage()
			dn.Deps.LogSet = &v1alpha1.LogSet{ObjectMeta: logSetKey(mo)}
			return nil
		}),
		recon.CreateOwnedOrUpdate(ctx, tp, func() error {
			tp.Spec.CNSetBasic = mo.Spec.TP
			tp.Spec.Image = mo.TpSetImage()
			tp.Deps.LogSet = &v1alpha1.LogSet{ObjectMeta: logSetKey(mo)}
			return nil
		}),
	)
	if mo.Spec.AP != nil {
		ap := &v1alpha1.CNSet{
			ObjectMeta: apSetKey(mo),
			Deps:       v1alpha1.CNSetDeps{LogSetRef: ls.AsDependency()},
		}
		errs = multierr.Append(errs, recon.CreateOwnedOrUpdate(ctx, ap, func() error {
			ap.Spec.CNSetBasic = *mo.Spec.AP
			ap.Spec.Image = mo.ApSetImage()
			return nil
		}))
		mo.Status.AP = &ap.Status
	}
	if mo.Spec.WebUI != nil {
		webui := &v1alpha1.WebUI{
			ObjectMeta: webUIKey(mo),
		}
		errs = multierr.Append(errs, recon.CreateOwnedOrUpdate(ctx, webui, func() error {
			webui.Spec.WebUIBasic = *mo.Spec.WebUI
			return nil
		}))
	}
	if errs != nil {
		return nil, errs
	}

	// collect status
	mo.Status.LogService = &ls.Status
	mo.Status.DN = &dn.Status
	mo.Status.TP = &tp.Status
	if recon.IsReady(mo.Status.TP) {
		mo.Status.ConditionalStatus.SetCondition(metav1.Condition{
			Type:   recon.ConditionTypeReady,
			Status: metav1.ConditionTrue,
		})
	}

	return nil, nil
}

func (r *MatrixOneClusterActor) Finalize(ctx *recon.Context[*v1alpha1.MatrixOneCluster]) (bool, error) {
	mo := ctx.Obj
	objs := []client.Object{
		&v1alpha1.LogSet{ObjectMeta: logSetKey(mo)},
		&v1alpha1.DNSet{ObjectMeta: dnSetKey(mo)},
		&v1alpha1.CNSet{ObjectMeta: tpSetKey(mo)},
		&v1alpha1.CNSet{ObjectMeta: apSetKey(mo)},
		&v1alpha1.WebUI{ObjectMeta: webUIKey(mo)},
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

func logSetKey(mo *v1alpha1.MatrixOneCluster) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:      mo.Name,
		Namespace: mo.Namespace,
	}
}

func dnSetKey(mo *v1alpha1.MatrixOneCluster) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:      mo.Name,
		Namespace: mo.Namespace,
	}
}

func tpSetKey(mo *v1alpha1.MatrixOneCluster) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:      mo.Name + "-tp",
		Namespace: mo.Namespace,
	}
}

func apSetKey(mo *v1alpha1.MatrixOneCluster) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:      mo.Name + "-ap",
		Namespace: mo.Namespace,
	}
}

func webUIKey(mo *v1alpha1.MatrixOneCluster) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:      mo.Name,
		Namespace: mo.Namespace,
	}
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
