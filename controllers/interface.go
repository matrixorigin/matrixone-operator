package controllers

import (
	"fmt"
	"reflect"

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
)

type matrixoneEventReason string

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
