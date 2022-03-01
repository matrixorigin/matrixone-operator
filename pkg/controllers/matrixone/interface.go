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

	"github.com/matrixorigin/matrixone-operator/pkg/apis/matrixone/v1alpha1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type MatrixoneClusterStatus string

const (
	resourceCreated MatrixoneClusterStatus = "CREATED"
	resourceUpdated MatrixoneClusterStatus = "UPDATED"
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

type Reader interface {
	List(ctx context.Context, sdk client.Client, moc *v1alpha1.MatrixoneCluster, selectorLabels map[string]string, emitEvent EventEmitter,
		emptyListObjFn func() objectList, ListObjFn func(obj runtime.Object) []object) ([]object, error)
	Get(ctx context.Context, sdk client.Client, moc *v1alpha1.MatrixoneCluster, emptyObjFn func() object, emitEvent EventEmitter) (object, error)
}

type ReaderFuncs struct{}

var readers Reader = ReaderFuncs{}

// Get methods shall the get the object.
func (f ReaderFuncs) Get(ctx context.Context, sdk client.Client, moc *v1alpha1.MatrixoneCluster, emptyObjFn func() object, emitEvent EventEmitter) (object, error) {
	obj := emptyObjFn()

	if err := sdk.Get(ctx, *namespacedName(moc.Name, moc.Namespace), obj); err != nil {
		emitEvent.EmitEventOnGetError(moc, obj, err)
		return nil, err
	}
	return obj, nil
}

// List methods shall return the list of an object
func (f ReaderFuncs) List(
	ctx context.Context,
	sdk client.Client,
	moc *v1alpha1.MatrixoneCluster,
	selectorLabels map[string]string,
	emitEvent EventEmitter,
	emptyListObjFn func() objectList,
	ListObjFn func(obj runtime.Object) []object) ([]object, error) {

	listOpts := []client.ListOption{
		client.InNamespace(moc.Namespace),
		client.MatchingLabels(selectorLabels),
	}
	listObj := emptyListObjFn()

	if err := sdk.List(ctx, listObj, listOpts...); err != nil {
		emitEvent.EmitEventOnList(moc, listObj, err)
		return nil, err
	}

	return ListObjFn(listObj), nil
}

type Writer interface {
	Create(ctx context.Context, sdk client.Client, moc *v1alpha1.MatrixoneCluster, obj object, emitEvent EventEmitter) (MatrixoneClusterStatus, error)
	Delete(ctx context.Context, sdk client.Client, moc *v1alpha1.MatrixoneCluster, obj object, emitEvent EventEmitter, deleteOptions ...client.DeleteOption) error
	Patch(ctx context.Context, sdk client.Client, moc *v1alpha1.MatrixoneCluster, obj object, status bool, patch client.Patch, emitEvent EventEmitter) error
	Update(ctx context.Context, sdk client.Client, moc *v1alpha1.MatrixoneCluster, obj object, emitEvent EventEmitter) (MatrixoneClusterStatus, error)
}

// WriterFuncs struct
type WriterFuncs struct{}

// Initalize Writer
var writers Writer = WriterFuncs{}

// Patch method shall patch the status of Obj or the status.
// Pass status as true to patch the object status.
// NOTE: Not logging on patch success, it shall keep logging on each reconcile
func (f WriterFuncs) Patch(ctx context.Context, sdk client.Client, moc *v1alpha1.MatrixoneCluster, obj object, status bool, patch client.Patch, emitEvent EventEmitter) error {

	if !status {
		if err := sdk.Patch(ctx, obj, patch); err != nil {
			emitEvent.EmitEventOnPatch(moc, obj, err)
			return err
		} else {
			if err := sdk.Status().Patch(ctx, obj, patch); err != nil {
				emitEvent.EmitEventOnPatch(moc, obj, err)
				return err
			}
		}
	}

	return nil
}

func (f WriterFuncs) Create(ctx context.Context, sdk client.Client, moc *v1alpha1.MatrixoneCluster, obj object, emitEvent EventEmitter) (MatrixoneClusterStatus, error) {

	if err := sdk.Create(ctx, obj); err != nil {
		logger.Error(err, err.Error(), "object", stringifyForLogging(obj, moc), "name", moc.Name, "namespace", moc.Namespace, "errorType", apierrors.ReasonForError(err))
		emitEvent.EmitEventOnCreate(moc, obj, err)
		return "", err
	} else {
		emitEvent.EmitEventOnCreate(moc, obj, nil)
		return resourceCreated, nil
	}
}

// Update Func shall update the Object
func (f WriterFuncs) Update(ctx context.Context, sdk client.Client, moc *v1alpha1.MatrixoneCluster, obj object, emitEvent EventEmitter) (MatrixoneClusterStatus, error) {

	if err := sdk.Update(ctx, obj); err != nil {
		emitEvent.EmitEventOnUpdate(moc, obj, err)
		return "", err
	} else {
		emitEvent.EmitEventOnUpdate(moc, obj, nil)
		return resourceUpdated, nil
	}

}

func (f WriterFuncs) Delete(ctx context.Context, sdk client.Client, moc *v1alpha1.MatrixoneCluster, obj object, emitEvent EventEmitter, deleteOptions ...client.DeleteOption) error {

	if err := sdk.Delete(ctx, obj, deleteOptions...); err != nil {
		emitEvent.EmitEventOnDelete(moc, obj, err)
		return err
	} else {
		emitEvent.EmitEventOnDelete(moc, obj, err)
		return nil
	}
}
