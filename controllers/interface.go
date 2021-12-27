package controllers

import (
	"context"
	"fmt"
	"reflect"

	v1alpha1 "github.com/matrixorigin/matrixone-operator/api/v1"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type matrixoneEventReason string

type MatrixoneNodeStatus string

const (
	resourceCreated MatrixoneNodeStatus = "CREATED"
	resourceUpdated MatrixoneNodeStatus = "UPDATED"
)

const (
	rollingDeployWait          matrixoneEventReason = "matrixoneNodeRollingDeployWait"
	matrixoneOjectGetFail      matrixoneEventReason = "matrixoneOperatorGetFail"
	matrixoneNodeUpdateFail    matrixoneEventReason = "matrixoneOperatorUpdateFail"
	matrixoneNodeUpdateSuccess matrixoneEventReason = "matrixoneOperatorUpdateSuccess"
	matrixoneNodeDeleteFail    matrixoneEventReason = "matrixoneOperatorDeleteFail"
	matrixoneNodeDeleteSuccess matrixoneEventReason = "matrixoneOperatorDeleteSuccess"
	matrixoneNodeCreateSuccess matrixoneEventReason = "matrixoneOperatorCreateSuccess"
	matrixoneNodeCreateFail    matrixoneEventReason = "matrixoneOperatorCreateFail"
	matrixoneNodePatchFail     matrixoneEventReason = "matrixoneOperatorPatchFail"
	matrixoneNodePatchSucess   matrixoneEventReason = "matrixoneOperatorPatchSuccess"
	matrixoneObjectListFail    matrixoneEventReason = "matrixoneOperatorListFail"
)

type EmitEventFuncs struct {
	record.EventRecorder
}

type EventEmitter interface {
	K8sEventEmitter
	GenericEventEmitter
}

type GenericEventEmitter interface {
	EmitEventGeneric(obj object, eventReason, msg string, err error)
}

type K8sEventEmitter interface {
	EmitEventRollingDeployWait(obj, k8sObj object, nodeSpecUniqueStr string)
	EmitEventOnGetError(obj, getObj object, err error)
	EmitEventOnUpdate(obj, updateObj object, err error)
	EmitEventOnDelete(obj, deleteObj object, err error)
	EmitEventOnCreate(obj, createObj object, err error)
	EmitEventOnPatch(obj, patchObj object, err error)
	EmitEventOnList(obj object, listObj objectList, err error)
}

type object interface {
	metav1.Object
	runtime.Object
}

type objectList interface {
	metav1.ListInterface
	runtime.Object
}

// Reader Interface
type Reader interface {
	List(ctx context.Context, sdk client.Client, moc *v1alpha1.MatrixoneCluster, selectorLabels map[string]string, emitEvent EventEmitter, emptyListObjFn func() objectList, ListObjFn func(obj runtime.Object) []object) ([]object, error)
	Get(ctx context.Context, sdk client.Client, nodeSpecUniqueStr string, moc *v1alpha1.MatrixoneCluster, emptyObjFn func() object, emitEvent EventEmitter) (object, error)
}

// Writer Interface
type Writer interface {
	Delete(ctx context.Context, sdk client.Client, moc *v1alpha1.MatrixoneCluster, obj object, emitEvent EventEmitter, deleteOptions ...client.DeleteOption) error
	Create(ctx context.Context, sdk client.Client, moc *v1alpha1.MatrixoneCluster, obj object, emitEvent EventEmitter) (MatrixoneNodeStatus, error)
	Update(ctx context.Context, sdk client.Client, moc *v1alpha1.MatrixoneCluster, obj object, emitEvent EventEmitter) (MatrixoneNodeStatus, error)
	Patch(ctx context.Context, sdk client.Client, moc *v1alpha1.MatrixoneCluster, obj object, status bool, patch client.Patch, emitEvent EventEmitter) error
}

// WriterFuncs struct
type WriterFuncs struct{}

// ReaderFuncs struct
type ReaderFuncs struct{}

// Initalizie Reader
var readers Reader = ReaderFuncs{}

func (f ReaderFuncs) List(ctx context.Context, sdk client.Client, moc *v1alpha1.MatrixoneCluster, selectorLabels map[string]string, emitEvent EventEmitter, emptyListObjFn func() objectList, ListObjFn func(obj runtime.Object) []object) ([]object, error) {
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

func (f ReaderFuncs) Get(ctx context.Context, sdk client.Client, nodeSpecUniqueStr string, moc *v1alpha1.MatrixoneCluster, emptyObjFn func() object, emitEvent EventEmitter) (object, error) {
	obj := emptyObjFn()

	if err := sdk.Get(ctx, *namespacedName(nodeSpecUniqueStr, moc.Namespace), obj); err != nil {
		emitEvent.EmitEventOnGetError(moc, obj, err)
		return nil, err
	}
	return obj, nil
}

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
		}
	} else {
		if err := sdk.Status().Patch(ctx, obj, patch); err != nil {
			emitEvent.EmitEventOnPatch(moc, obj, err)
			return err
		}
	}
	return nil
}

// Update Func shall update the Object
func (f WriterFuncs) Update(ctx context.Context, sdk client.Client, moc *v1alpha1.MatrixoneCluster, obj object, emitEvent EventEmitter) (MatrixoneNodeStatus, error) {

	if err := sdk.Update(ctx, obj); err != nil {
		emitEvent.EmitEventOnUpdate(moc, obj, err)
		return "", err
	} else {
		emitEvent.EmitEventOnUpdate(moc, obj, nil)
		return resourceUpdated, nil
	}

}

// Create methods shall create an object, and returns a string, error
func (f WriterFuncs) Create(ctx context.Context, sdk client.Client, moc *v1alpha1.MatrixoneCluster, obj object, emitEvent EventEmitter) (MatrixoneNodeStatus, error) {

	if err := sdk.Create(ctx, obj); err != nil {
		logger.Error(err, err.Error(), "object", stringifyForLogging(obj, moc), "name", moc.Name, "namespace", moc.Namespace, "errorType", apierrors.ReasonForError(err))
		emitEvent.EmitEventOnCreate(moc, obj, err)
		return "", err
	} else {
		emitEvent.EmitEventOnCreate(moc, obj, nil)
		return resourceCreated, nil
	}

}

// Delete methods shall delete the object, deleteOptions is a variadic parameter to support various delete options such as cascade deletion.
func (f WriterFuncs) Delete(ctx context.Context, sdk client.Client, moc *v1alpha1.MatrixoneCluster, obj object, emitEvent EventEmitter, deleteOptions ...client.DeleteOption) error {

	if err := sdk.Delete(ctx, obj, deleteOptions...); err != nil {
		emitEvent.EmitEventOnDelete(moc, obj, err)
		return err
	} else {
		emitEvent.EmitEventOnDelete(moc, obj, err)
		return nil
	}
}

// EmitEventGeneric shall emit a generic event
func (e EmitEventFuncs) EmitEventGeneric(obj object, eventReason, msg string, err error) {
	if err != nil {
		e.Event(obj, v1.EventTypeWarning, eventReason, err.Error())
	} else if msg != "" {
		e.Event(obj, v1.EventTypeNormal, eventReason, msg)

	}
}

// EmitEventOnCreate shall emit event on CREATE operation
func (e EmitEventFuncs) EmitEventOnCreate(obj, createObj object, err error) {
	if err != nil {
		errMsg := fmt.Errorf("Error creating object [%s] in namespace [%s:%s] due to [%s]", createObj.GetName(), createObj.GetObjectKind().GroupVersionKind().Kind, createObj.GetNamespace(), err.Error())
		e.Event(obj, v1.EventTypeWarning, string(matrixoneNodeCreateFail), errMsg.Error())
	} else {
		msg := fmt.Sprintf("Successfully created object [%s:%s] in namespace [%s]", createObj.GetName(), createObj.GetObjectKind().GroupVersionKind().Kind, createObj.GetNamespace())
		e.Event(obj, v1.EventTypeNormal, string(matrixoneNodeCreateSuccess), msg)
	}
}

// EmitEventOnDelete shall emit event on DELETE operation
func (e EmitEventFuncs) EmitEventOnDelete(obj, deleteObj object, err error) {
	if err != nil {
		errMsg := fmt.Errorf("Error deleting object [%s:%s] in namespace [%s] due to [%s]", deleteObj.GetObjectKind().GroupVersionKind().Kind, deleteObj.GetName(), deleteObj.GetNamespace(), err.Error())
		e.Event(obj, v1.EventTypeWarning, string(matrixoneNodeDeleteFail), errMsg.Error())
	} else {
		msg := fmt.Sprintf("Successfully deleted object [%s:%s] in namespace [%s]", deleteObj.GetName(), deleteObj.GetObjectKind().GroupVersionKind().Kind, deleteObj.GetNamespace())
		e.Event(obj, v1.EventTypeNormal, string(matrixoneNodeDeleteSuccess), msg)
	}
}

// EmitEventOnGetError shall emit event on GET err operation
func (e EmitEventFuncs) EmitEventOnGetError(obj, getObj object, err error) {
	getErr := fmt.Errorf("Failed to get [Object:%s] due to [%s]", getObj.GetName(), err.Error())
	e.Event(obj, v1.EventTypeWarning, string(matrixoneOjectGetFail), getErr.Error())
}

//  EmitEventOnList shall emit event on LIST err operation
func (e EmitEventFuncs) EmitEventOnList(obj object, listObj objectList, err error) {
	if err != nil {
		errMsg := fmt.Errorf("Error listing object [%s] in namespace [%s] due to [%s]", listObj.GetObjectKind().GroupVersionKind().Kind, obj.GetNamespace(), err.Error())
		e.Event(obj, v1.EventTypeWarning, string(matrixoneObjectListFail), errMsg.Error())
	}
}

// EmitEventOnPatch shall emit event on PATCH operation
func (e EmitEventFuncs) EmitEventOnPatch(obj, patchObj object, err error) {
	if err != nil {
		errMsg := fmt.Errorf("Error patching object [%s:%s] in namespace [%s] due to [%s]", patchObj.GetName(), patchObj.GetObjectKind().GroupVersionKind().Kind, patchObj.GetNamespace(), err.Error())
		e.Event(obj, v1.EventTypeWarning, string(matrixoneNodePatchFail), errMsg.Error())
	} else {
		msg := fmt.Sprintf("Successfully patched object [%s:%s] in namespace [%s]", patchObj.GetName(), patchObj.GetObjectKind().GroupVersionKind().Kind, patchObj.GetNamespace())
		e.Event(obj, v1.EventTypeNormal, string(matrixoneNodePatchSucess), msg)
	}
}

// EmitEventOnUpdate shall emit event on UPDATE operation
func (e EmitEventFuncs) EmitEventOnUpdate(obj, updateObj object, err error) {
	if err != nil {
		errMsg := fmt.Errorf("Failed to update [%s:%s] due to [%s].", updateObj.GetName(), updateObj.GetObjectKind().GroupVersionKind().Kind, err.Error())
		e.Event(obj, v1.EventTypeWarning, string(matrixoneNodeUpdateFail), errMsg.Error())
	} else {
		msg := fmt.Sprintf("Updated [%s:%s].", updateObj.GetName(), updateObj.GetObjectKind().GroupVersionKind().Kind)
		e.Event(obj, v1.EventTypeNormal, string(matrixoneNodeUpdateSuccess), msg)
	}
}

// EmitEventRollingDeployWait shall emit an event when the current state of a matrixone node is rolling deploy
func (e EmitEventFuncs) EmitEventRollingDeployWait(obj, k8sObj object, nodeSpecUniqueStr string) {
	if detectType(k8sObj) == "*v1.StatefulSet" {
		msg := fmt.Sprintf("StatefulSet[%s] roll out is in progress CurrentRevision[%s] != UpdateRevision[%s]", nodeSpecUniqueStr, k8sObj.(*appsv1.StatefulSet).Status.CurrentRevision, k8sObj.(*appsv1.StatefulSet).Status.UpdateRevision)
		e.Event(obj, v1.EventTypeNormal, string(rollingDeployWait), msg)
	} else if detectType(k8sObj) == "*v1.Deployment" {
		msg := fmt.Sprintf("Deployment[%s] roll out is in progress in namespace [%s], ReadyReplicas [%d] != Current Replicas [%d]", k8sObj.(*appsv1.Deployment).Name, k8sObj.GetNamespace(), k8sObj.(*appsv1.Deployment).Status.ReadyReplicas, k8sObj.(*appsv1.Deployment).Status.Replicas)
		e.Event(obj, v1.EventTypeNormal, string(rollingDeployWait), msg)
	}
}

// return k8s object type
// Deployment : *v1.Deployment
// StatefulSet: *v1.StatefulSet
func detectType(obj object) string { return reflect.TypeOf(obj).String() }
