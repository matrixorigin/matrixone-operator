// Copyright 2022 Matrix Origin
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package reconciler

import (
	"context"
	"reflect"
	"runtime"

	"github.com/go-logr/logr"
	"golang.org/x/exp/slices"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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

// TODO(aylei): add logging and tracing when operate upon kube-api
func (c *Context[T]) Create(obj client.Object, opts ...client.CreateOption) error {
	return c.Client.Create(c, obj, opts...)
}

func (c *Context[T]) Get(objKey client.ObjectKey, obj client.Object) error {
	return c.Client.Get(c, objKey, obj)
}

func (c *Context[T]) Update(obj client.Object, opts ...client.UpdateOption) error {
	return c.Client.Update(c, obj, opts...)
}

func (c *Context[T]) Delete(obj client.Object, opts ...client.DeleteOption) error {
	return c.Client.Delete(c, obj, opts...)
}

func (c *Context[T]) List(objList client.ObjectList, opts ...client.ListOption) error {
	return c.Client.List(c, objList, opts...)
}

func (c *Context[T]) CheckExists(objKey client.ObjectKey, kind client.Object) (bool, error) {
	err := c.Get(objKey, kind)
	if err != nil && apierrors.IsNotFound(err) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	return true, nil
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
