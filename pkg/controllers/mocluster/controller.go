package mocluster

import (
	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	recon "github.com/matrixorigin/matrixone-operator/runtime/pkg/reconciler"
	"github.com/matrixorigin/matrixone-operator/runtime/pkg/util"
	"go.uber.org/multierr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ recon.Actor[*v1alpha1.MatrixoneCluster] = &MatrixoneClusterActor{}

type MatrixoneClusterActor struct{}

func (r *MatrixoneClusterActor) Observe(ctx *recon.Context[*v1alpha1.MatrixoneCluster]) (recon.Action[*v1alpha1.MatrixoneCluster], error) {
	mo := ctx.Obj

	// sync specs
	ls := &v1alpha1.LogSet{
		ObjectMeta: logSetKey(mo),
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
			ls.Spec.LogSetBasic = mo.Spec.LogService
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
	if errs != nil {
		return nil, errs
	}

	// collect status
	mo.Status.LogService = &ls.Status
	mo.Status.DN = &dn.Status
	mo.Status.TP = &tp.Status
	if mo.Status.TP.Ready() {
		mo.Status.ConditionalStatus.SetCondition(metav1.Condition{
			Type:   v1alpha1.ConditionTypeReady,
			Status: metav1.ConditionTrue,
		})
	}

	return nil, nil
}

func (r *MatrixoneClusterActor) Finalize(ctx *recon.Context[*v1alpha1.MatrixoneCluster]) (bool, error) {
	mo := ctx.Obj
	objs := []client.Object{
		&v1alpha1.LogSet{ObjectMeta: logSetKey(mo)},
		&v1alpha1.DNSet{ObjectMeta: dnSetKey(mo)},
		&v1alpha1.CNSet{ObjectMeta: tpSetKey(mo)},
		&v1alpha1.CNSet{ObjectMeta: apSetKey(mo)},
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

func logSetKey(mo *v1alpha1.MatrixoneCluster) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:      mo.Name + "-log",
		Namespace: mo.Namespace,
	}
}

func dnSetKey(mo *v1alpha1.MatrixoneCluster) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:      mo.Name + "-dn",
		Namespace: mo.Namespace,
	}
}

func tpSetKey(mo *v1alpha1.MatrixoneCluster) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:      mo.Name + "-cn",
		Namespace: mo.Namespace,
	}
}

func apSetKey(mo *v1alpha1.MatrixoneCluster) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:      mo.Name + "-ap",
		Namespace: mo.Namespace,
	}
}
