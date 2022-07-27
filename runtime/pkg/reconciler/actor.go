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
	"fmt"
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

type KubeClient interface {
	Create(obj client.Object, opts ...client.CreateOption) error
	CreateOwned(obj client.Object, opts ...client.CreateOption) error
	Get(objKey client.ObjectKey, obj client.Object) error
	Update(obj client.Object, opts ...client.UpdateOption) error
	UpdateStatus(obj client.Object, opts ...client.UpdateOption) error
	Delete(obj client.Object, opts ...client.DeleteOption) error
	List(objList client.ObjectList, opts ...client.ListOption) error
	Patch(obj client.Object, mutateFn func() error, opts ...client.PatchOption) error
	Exist(objKey client.ObjectKey, kind client.Object) (bool, error)
}

var _ KubeClient = &Context[client.Object]{}

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

// Update update the spec of the given obj
func (c *Context[T]) Update(obj client.Object, opts ...client.UpdateOption) error {
	return c.Client.Update(c, obj, opts...)
}

// UpdateStatus update the status of the given obj
func (c *Context[T]) UpdateStatus(obj client.Object, opts ...client.UpdateOption) error {
	return c.Client.Status().Update(c, obj, opts...)
}

// Delete marks the given obj to be deleted
func (c *Context[T]) Delete(obj client.Object, opts ...client.DeleteOption) error {
	return c.Client.Delete(c, obj, opts...)
}

func (c *Context[T]) List(objList client.ObjectList, opts ...client.ListOption) error {
	return c.Client.List(c, objList, opts...)
}

// Patch patches the mutation by mutateFn to the spec of given obj
// an error would be raised if mutateFn changed anything immutable (e.g. namespace / name)
func (c *Context[T]) Patch(obj client.Object, mutateFn func() error, opts ...client.PatchOption) error {
	key := client.ObjectKeyFromObject(obj)
	if err := c.Get(client.ObjectKeyFromObject(obj), obj); err != nil {
		return err
	}
	before := obj.DeepCopyObject().(client.Object)
	if err := mutateFn(); err != nil {
		return err
	}
	if newKey := client.ObjectKeyFromObject(obj); key != newKey {
		return fmt.Errorf("MutateFn cannot mutate object name and/or object namespace")
	}
	if reflect.DeepEqual(before, obj) {
		// no change to patch
		return nil
	}
	return c.Client.Patch(c, obj, client.MergeFrom(before), opts...)
}

// CreateOwned create the given object with an OwnerReference to the currently reconciling
// controller object (ctx.Obj)
func (c *Context[T]) CreateOwned(obj client.Object, opts ...client.CreateOption) error {
	if err := controllerutil.SetOwnerReference(c.Obj, obj, c.reconciler.Scheme()); err != nil {
		return err
	}
	return c.Client.Create(c, obj, opts...)
}

func (c *Context[T]) Exist(objKey client.ObjectKey, kind client.Object) (bool, error) {
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
