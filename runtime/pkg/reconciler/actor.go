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

	mlogs "github.com/matrixorigin/matrixone-operator/runtime/pkg/logs"
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
	Log   mlogs.Mlog

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
