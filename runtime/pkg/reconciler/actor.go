package reconciler

import (
	"context"
	"reflect"
	"runtime"

	"github.com/go-logr/logr"
	"golang.org/x/exp/slices"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type Actor[T client.Object] interface {
	Observe(*Context[T]) (Action[T], error)
	Finalize(*Context[T]) (done bool, err error)
}

type Action[T client.Object] func(*Context[T]) error

func (s Action[T]) String() string {
	return runtime.FuncForPC(reflect.ValueOf(s).Pointer()).Name()
}

type Context[T client.Object] struct {
	context.Context
	Obj T

	Client client.Client
	// TODO(aylei): add tracing
	Event EventEmitter
	Log   logr.Logger

	reconciler *Reconciler[T]
}

func (c *Context[T]) hasFinalizer() bool {
	return slices.Contains(c.Obj.GetFinalizers(), c.reconciler.finalizer())
}

func (c *Context[T]) removeFinalizer() error {
	if controllerutil.RemoveFinalizer(c.Obj, c.reconciler.finalizer()) {
		return c.Client.Update(c, c.Obj)
	}
	return nil
}

func (c *Context[T]) ensureFinalizer(ctx context.Context, obj T) error {
	if controllerutil.AddFinalizer(c.Obj, c.reconciler.finalizer()) {
		return c.Client.Update(c, obj)
	}
	return nil
}