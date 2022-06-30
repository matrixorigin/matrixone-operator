package reconciler

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"
	"golang.org/x/exp/slices"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	ctrl "sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/matrixorigin/matrixone-operator/runtime/pkg/actor"
)

const (
	finalizerPrefix = "matrixorigin.io"
)

var _ ctrl.Reconciler = &Reconciler[client.Object]{}

type Reconciler[T client.Object] struct {
	client.Client

	name  string
	newT  func() T
	actor actor.Actor[T]

	// TODO(aylei): add tracing
	record record.EventRecorder
	log    zap.Logger
}

func (r *Reconciler[T]) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	obj := r.newT()
	if err := r.Get(ctx, req.NamespacedName, obj); err != nil {
		return ctrl.Result{}, err
	}

	actionCtx := &actor.Context[T]{Obj: obj}
	if wasDeleted(obj) {
		return r.finalize(actionCtx)
	}
	
	if err := r.addFinalizer(ctx, obj); err != nil {
		return ctrl.Result{Requeue: true}, err
	}

	// TODO(aylei): wait dependencies to be ready
	action, err := r.actor.Observe(actionCtx)
	if err != nil {
		return ctrl.Result{Requeue: true}, err
	}
	if action == nil {
		return ctrl.Result{RequeueAfter: 5*time.Minute}, nil
	}
	if err := action(actionCtx); err != nil {
		return ctrl.Result{Requeue: true}, err
	}
	return ctrl.Result{Requeue: true}, nil
}

func (r *Reconciler[T]) finalize(ctx *actor.Context[T]) (ctrl.Result, error) {
	if !r.hasFinalizer(ctx.Obj) {
		// Finalizer work of current reconciler is done or not needed, the object might
		// wait other reconcilers to complete there finalizer work, ignore.
		return ctrl.Result{Requeue: false}, nil
	}
	if err := r.actor.Delete(ctx); err != nil {
		return ctrl.Result{Requeue: true}, err
	}
	if err := r.removeFinalizer(ctx, ctx.Obj); err != nil {
		return ctrl.Result{Requeue: true}, err
	}
	return ctrl.Result{Requeue: false}, nil
}

func (r *Reconciler[T]) hasFinalizer(obj T) bool {
	return slices.Contains(obj.GetFinalizers(), r.finalizer())
}

func (r *Reconciler[T]) finalizer() string {
	return fmt.Sprintf("%s/%s", finalizerPrefix, r.name)
}

func (r *Reconciler[T]) removeFinalizer(ctx context.Context, obj T) error {
	if controllerutil.RemoveFinalizer(obj, r.finalizer()) {
		return r.Update(ctx, obj)
	}
	return nil
}

func (r *Reconciler[T]) addFinalizer(ctx context.Context, obj T) error {
	if controllerutil.AddFinalizer(obj, r.finalizer()) {
		return r.Update(ctx, obj)
	}
	return nil
}

func contains[T comparable](list []T, val T) bool {
	for i := range list {
		if val == list[i] {
			return true
		}
	}
	return false
}

func wasDeleted(obj client.Object) bool {
	return obj.GetDeletionTimestamp() != nil
}

func mustCreatObject(kind schema.GroupVersionKind, oc runtime.ObjectCreater) runtime.Object {
	obj, err := oc.New(kind)
	if err != nil {
		panic(err)
	}
	return obj
}
