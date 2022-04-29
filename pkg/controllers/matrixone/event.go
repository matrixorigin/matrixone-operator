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
	"fmt"
	"reflect"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/record"
)

type matrixoneEventReason string

const (
	rollingDeployWait       matrixoneEventReason = "MatrixoneRollingDeployWait"
	matrixoneCreateSuccess  matrixoneEventReason = "MatrixoneOperatorCreateSuccess"
	matrixoneCreateFail     matrixoneEventReason = "MatrixoneOperatorCreateFail"
	matrixonePatchFail      matrixoneEventReason = "MatrixoneOperatorPatchFail"
	matrixonePatchSuccess   matrixoneEventReason = "MatrixoneOperatorPatchSuccess"
	matrixoneObjectGetFail  matrixoneEventReason = "MatrixoneObjectGetFail"
	matrixoneUpdateFail     matrixoneEventReason = "MatrixoneUpdateFail"
	matrixoneUpdateSuccess  matrixoneEventReason = "MatrixoneUpdateSuccess"
	matrixoneObjectListFail matrixoneEventReason = "MatrixoneObjectListFail"
	matrixoneDeleteFail     matrixoneEventReason = "MatrixoneDeleteFail"
	matrixoneDeleteSuccess  matrixoneEventReason = "MatrixoneDeleteSuccess"
)

//  EventEmitter Interface is a wrapper interface for all the emitter interface  operator shall support.
type EventEmitter interface {
	K8sEventEmitter
	GenericEventEmitter
}

type EmitEventFuncs struct {
	record.EventRecorder
}

type K8sEventEmitter interface {
	EmitEventRollingDeployWait(obj, k8sObj object)
	EmitEventOnGetError(obj, getObj object, err error)
	EmitEventOnUpdate(obj, updateObj object, err error)
	EmitEventOnDelete(obj, deleteObj object, err error)
	EmitEventOnCreate(obj, createObj object, err error)
	EmitEventOnPatch(obj, patchObj object, err error)
	EmitEventOnList(obj object, listObj objectList, err error)
}

// GenericEventEmitter can be used for any case where the state change isn't handled by reader,writer or any custom event.
type GenericEventEmitter interface {
	EmitEventGeneric(obj object, eventReason, msg string, err error)
}

// EmitEventGeneric shall emit a generic event
func (e EmitEventFuncs) EmitEventGeneric(obj object, eventReason, msg string, err error) {
	if err != nil {
		e.Event(obj, corev1.EventTypeWarning, eventReason, err.Error())
	} else if msg != "" {
		e.Event(obj, corev1.EventTypeNormal, eventReason, msg)

	}
}

func (e EmitEventFuncs) EmitEventRollingDeployWait(obj, k8sObj object) {
	if detectType(k8sObj) == "*v1.StatefulSet" {
		msg := fmt.Sprintf("StatefulSet roll out is in progress CurrentRevision[%s] != UpdateRevision[%s]", k8sObj.(*appsv1.StatefulSet).Status.CurrentRevision, k8sObj.(*appsv1.StatefulSet).Status.UpdateRevision)
		e.Event(obj, v1.EventTypeNormal, string(rollingDeployWait), msg)
	} else if detectType(k8sObj) == "*v1.Deployment" {
		msg := fmt.Sprintf("Deployment[%s] roll out is in progress in namespace [%s], ReadyReplicas [%d] != Current Replicas [%d]", k8sObj.(*appsv1.Deployment).Name, k8sObj.GetNamespace(), k8sObj.(*appsv1.Deployment).Status.ReadyReplicas, k8sObj.(*appsv1.Deployment).Status.Replicas)
		e.Event(obj, v1.EventTypeNormal, string(rollingDeployWait), msg)
	}
}

// EmitEventOnCreate shall emit event on CREATE operation
func (e EmitEventFuncs) EmitEventOnCreate(obj, createObj object, err error) {
	if err != nil {
		errMsg := fmt.Errorf("Error creating object [%s] in namespace [%s:%s] due to [%s]", createObj.GetName(), createObj.GetObjectKind().GroupVersionKind().Kind, createObj.GetNamespace(), err.Error())
		e.Event(obj, corev1.EventTypeWarning, string(matrixoneCreateFail), errMsg.Error())
	} else {
		msg := fmt.Sprintf("Successfully created object [%s:%s] in namespace [%s]", createObj.GetName(), createObj.GetObjectKind().GroupVersionKind().Kind, createObj.GetNamespace())
		e.Event(obj, corev1.EventTypeNormal, string(matrixoneCreateSuccess), msg)
	}
}

// EmitEventOnPatch shall emit event on PATCH operation
func (e EmitEventFuncs) EmitEventOnPatch(obj, patchObj object, err error) {
	if err != nil {
		errMsg := fmt.Errorf("error patching object [%s:%s] in namespace [%s] due to [%s]", patchObj.GetName(), patchObj.GetObjectKind().GroupVersionKind().Kind, patchObj.GetNamespace(), err.Error())
		e.Event(obj, v1.EventTypeWarning, string(matrixonePatchFail), errMsg.Error())
	} else {
		msg := fmt.Sprintf("successfully patched object [%s:%s] in namespace [%s]", patchObj.GetName(), patchObj.GetObjectKind().GroupVersionKind().Kind, patchObj.GetNamespace())
		e.Event(obj, v1.EventTypeNormal, string(matrixonePatchSuccess), msg)
	}
}

// EmitEventOnUpdate shall emit event on UPDATE operation
func (e EmitEventFuncs) EmitEventOnUpdate(obj, updateObj object, err error) {
	if err != nil {
		errMsg := fmt.Errorf("failed to update [%s:%s] due to [%s]", updateObj.GetName(), updateObj.GetObjectKind().GroupVersionKind().Kind, err.Error())
		e.Event(obj, v1.EventTypeWarning, string(matrixoneUpdateFail), errMsg.Error())
	} else {
		msg := fmt.Sprintf("updated [%s:%s].", updateObj.GetName(), updateObj.GetObjectKind().GroupVersionKind().Kind)
		e.Event(obj, v1.EventTypeNormal, string(matrixoneUpdateSuccess), msg)
	}
}

// EmitEventOnGetError shall emit event on GET err operation
func (e EmitEventFuncs) EmitEventOnGetError(obj, getObj object, err error) {
	getErr := fmt.Errorf("failed to get [Object:%s] due to [%s]", getObj.GetName(), err.Error())
	e.Event(obj, v1.EventTypeWarning, string(matrixoneObjectGetFail), getErr.Error())
}

//  EmitEventOnList shall emit event on LIST err operation
func (e EmitEventFuncs) EmitEventOnList(obj object, listObj objectList, err error) {
	if err != nil {
		errMsg := fmt.Errorf("error listing object [%s] in namespace [%s] due to [%s]", listObj.GetObjectKind().GroupVersionKind().Kind, obj.GetNamespace(), err.Error())
		e.Event(obj, v1.EventTypeWarning, string(matrixoneObjectListFail), errMsg.Error())
	}
}

// EmitEventOnDelete shall emit event on DELETE operation
func (e EmitEventFuncs) EmitEventOnDelete(obj, deleteObj object, err error) {
	if err != nil {
		errMsg := fmt.Errorf("Error deleting object [%s:%s] in namespace [%s] due to [%s]", deleteObj.GetObjectKind().GroupVersionKind().Kind, deleteObj.GetName(), deleteObj.GetNamespace(), err.Error())
		e.Event(obj, v1.EventTypeWarning, string(matrixoneDeleteFail), errMsg.Error())
	} else {
		msg := fmt.Sprintf("Successfully deleted object [%s:%s] in namespace [%s]", deleteObj.GetName(), deleteObj.GetObjectKind().GroupVersionKind().Kind, deleteObj.GetNamespace())
		e.Event(obj, v1.EventTypeNormal, string(matrixoneDeleteSuccess), msg)
	}
}

func detectType(obj object) string { return reflect.TypeOf(obj).String() }
