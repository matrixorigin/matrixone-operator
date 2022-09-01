package webui

import (
	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	recon "github.com/matrixorigin/matrixone-operator/runtime/pkg/reconciler"
	"github.com/matrixorigin/matrixone-operator/runtime/pkg/util"
	"github.com/pkg/errors"
	"github.com/samber/lo"
	"go.uber.org/multierr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

type WebUIActor struct{}

var _ recon.Actor[*v1alpha1.WebUI] = &WebUIActor{}

type WithResource struct {
	*WebUIActor
	dp *appsv1.Deployment
}

func (w *WebUIActor) with(dp *appsv1.Deployment) *WithResource {
	return &WithResource{WebUIActor: w, dp: dp}
}

func (w *WebUIActor) Observe(ctx *recon.Context[*v1alpha1.WebUI]) (recon.Action[*v1alpha1.WebUI], error) {
	wi := ctx.Obj

	dp := &appsv1.Deployment{}
	err, foundDp := util.IsFound(ctx.Get(client.ObjectKey{
		Namespace: wi.Namespace,
		Name:      webUIName(wi),
	}, dp))
	if err != nil {
		return nil, errors.Wrap(err, "get webui deployment")
	}

	if !foundDp {
		return w.Create, nil

	}

	origin := dp.DeepCopy()
	syncPods(ctx, dp)
	if !equality.Semantic.DeepEqual(origin, dp) {
		return w.with(dp).Update, nil
	}

	return nil, nil
}

func (w *WebUIActor) Finalize(ctx *recon.Context[*v1alpha1.WebUI]) (bool, error) {
	wi := ctx.Obj

	objs := []client.Object{&corev1.Service{ObjectMeta: metav1.ObjectMeta{
		Name: webUIName(wi),
	}}, &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{
		Name: webUIName(wi),
	}}}
	for _, obj := range objs {
		obj.SetNamespace(wi.Namespace)
		if err := util.Ignore(apierrors.IsNotFound, ctx.Delete(obj)); err != nil {
			return false, err
		}
	}
	for _, obj := range objs {
		exist, err := ctx.Exist(client.ObjectKeyFromObject(obj), obj)
		if err != nil {
			return false, err
		}
		if exist {
			return false, nil
		}
	}
	return true, nil
}

func (w *WebUIActor) Create(ctx *recon.Context[*v1alpha1.WebUI]) error {
	klog.V(recon.Info).Info("create webui service")
	wi := ctx.Obj

	wiObj := buildWebUI(wi)
	wiSvc := buildService(wi)
	syncReplicas(wi, wiObj)
	syncPodMeta(wi, wiObj)
	syncPodSpec(wi, wiObj)

	// create all resources
	err := lo.Reduce[client.Object, error]([]client.Object{
		wiSvc,
		wiObj,
	}, func(errs error, o client.Object, _ int) error {
		err := ctx.CreateOwned(o)
		return multierr.Append(errs, util.Ignore(apierrors.IsAlreadyExists, err))
	}, nil)
	if err != nil {
		return errors.Wrap(err, "create webui service")
	}

	return nil
}

func (r *WithResource) Update(ctx *recon.Context[*v1alpha1.WebUI]) error {
	return ctx.Update(r.dp)
}

func (w *WebUIActor) Reconcile(mgr manager.Manager) error {
	err := recon.Setup[*v1alpha1.WebUI](&v1alpha1.WebUI{}, "webui", mgr, w,
		recon.WithBuildFn(func(b *builder.Builder) {
			b.Owns(&appsv1.Deployment{}).
				Owns(&corev1.Service{})
		}))
	if err != nil {
		return err
	}

	return nil
}
