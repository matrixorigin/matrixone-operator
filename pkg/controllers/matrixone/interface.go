// Copyright 2021 Matrix Origin
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package matrixone

import (
	"context"
	"reflect"

	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ClusterStatus string

const (
	resourceCreated ClusterStatus = "CREATED"
	resourceUpdated ClusterStatus = "UPDATED"
)

// Object Interface : Wrapper interface includes metav1 object and runtime object interface.
type object interface {
	metav1.Object
	runtime.Object
}

// Object List Interface : Wrapper interface includes metav1 List and runtime object interface.
type objectList interface {
	metav1.ListInterface
	runtime.Object
}

func newObject[T any]() T {
	var obj T
	if typ := reflect.TypeOf(obj); typ.Kind() == reflect.Ptr {
		return reflect.New(typ.Elem()).Interface().(T)
	}
	return obj
}

// Get methods shall the get the object.
func Get[T object](ctx context.Context, sdk client.Client, moc *v1alpha1.MatrixoneCluster, emitEvent EventEmitter) (T, error) {
	obj := newObject[T]()
	if err := sdk.Get(ctx, *namespacedName(moc.Name, moc.Namespace), obj); err != nil {
		emitEvent.EmitEventOnGetError(moc, obj, err)
		return obj, err
	}
	return obj, nil
}

// List methods shall return the list of an object
func List[T object, TList objectList](
	ctx context.Context,
	sdk client.Client,
	moc *v1alpha1.MatrixoneCluster,
	selectorLabels map[string]string,
	emitEvent EventEmitter) ([]T, error) {

	listOpts := []client.ListOption{
		client.InNamespace(moc.Namespace),
		client.MatchingLabels(selectorLabels),
	}
	listObj := newObject[TList]()

	if err := sdk.List(ctx, listObj, listOpts...); err != nil {
		emitEvent.EmitEventOnList(moc, listObj, err)
		return nil, err
	}

	return extractList[T](listObj)
}

// extractList extract the items from an objectList interface.
// ideally, we should have a type constraint between Object and ObjectList:
//     type ObjectList[T] interface {
//         GetItems() []T
//     }
// so that the conversion can be type-safe. But the generated code of client-go
// does not have such constraint yet.
func extractList[T object](listObj objectList) ([]T, error) {

	items, err := meta.ExtractList(listObj)
	if err != nil {
		return nil, err
	}
	var res []T
	for _, item := range items {
		if obj, ok := item.(T); ok {
			res = append(res, obj)
		} else {
			return nil, errors.Errorf("unexpected type: %T", item)
		}
	}
	return res, nil
}

// Patch method shall patch the status of Obj or the status.
// Pass status as true to patch the object status.
// NOTE: Not logging on patch success, it shall keep logging on each reconcile
func Patch[T object](ctx context.Context, sdk client.Client, moc *v1alpha1.MatrixoneCluster, obj T, status bool, patch client.Patch, emitEvent EventEmitter) error {

	if !status {
		if err := sdk.Patch(ctx, obj, patch); err != nil {
			emitEvent.EmitEventOnPatch(moc, obj, err)
			return err
		}
		if err := sdk.Status().Patch(ctx, obj, patch); err != nil {
			emitEvent.EmitEventOnPatch(moc, obj, err)
			return err
		}
	}

	return nil
}

func Create[T object](ctx context.Context, sdk client.Client, moc *v1alpha1.MatrixoneCluster, obj T, emitEvent EventEmitter) (ClusterStatus, error) {

	if err := sdk.Create(ctx, obj); err != nil {
		logger.Error(err, err.Error(), "object", stringifyForLogging(obj, moc), "name", moc.Name, "namespace", moc.Namespace, "errorType", apierrors.ReasonForError(err))
		emitEvent.EmitEventOnCreate(moc, obj, err)
		return "", err
	}
	emitEvent.EmitEventOnCreate(moc, obj, nil)
	return resourceCreated, nil
}

// Update Func shall update the Object
func Update[T object](ctx context.Context, sdk client.Client, moc *v1alpha1.MatrixoneCluster, obj T, emitEvent EventEmitter) (ClusterStatus, error) {

	if err := sdk.Update(ctx, obj); err != nil {
		emitEvent.EmitEventOnUpdate(moc, obj, err)
		return "", err
	}
	emitEvent.EmitEventOnUpdate(moc, obj, nil)
	return resourceUpdated, nil

}

func Delete[T object](ctx context.Context, sdk client.Client, moc *v1alpha1.MatrixoneCluster, obj T, emitEvent EventEmitter, deleteOptions ...client.DeleteOption) error {

	err := sdk.Delete(ctx, obj, deleteOptions...)

	if err != nil {
		emitEvent.EmitEventOnDelete(moc, obj, err)
		return err
	}
	emitEvent.EmitEventOnDelete(moc, obj, nil)
	return nil
}
