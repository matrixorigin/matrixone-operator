// Copyright 2022 Matrix Origin
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

package reconciler

import (
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// TODO(aylei): event.go is modified from pkg/controller/event.go, remove the original one once we migrate to
// mo-runtime
type EventReason string

const (
	rollingDeployWait EventReason = "RollingDeployWait"
	createSuccess     EventReason = "CreateSuccess"
	createFail        EventReason = "CreateFail"
	patchFail         EventReason = "PatchFail"
	patchSuccess      EventReason = "PatchSuccess"
	objectGetFail     EventReason = "ObjectGetFail"
	updateFail        EventReason = "UpdateFail"
	updateSuccess     EventReason = "UpdateSuccess"
	objectListFail    EventReason = "ObjectListFail"
	deleteFail        EventReason = "DeleteFail"
	deleteSuccess     EventReason = "DeleteSuccess"
)

//  EventEmitter Interface is a wrapper interface for all the emitter interface  operator shall support.
type EventEmitter interface {
	K8sEventEmitter
	GenericEventEmitter
}

// EmitEventWrapper captures the object being reconciled and associated all emitted events with
// that object
type EmitEventWrapper struct {
	record.EventRecorder
	// subject is the object that being reconciled, which is the subject of the emitted events
	subject client.Object
}

type K8sEventEmitter interface {
	EmitEventRollingDeployWait(k8sObj client.Object)
	EmitEventOnGetError(getObj client.Object, err error)
	EmitEventOnUpdate(updateObj client.Object, err error)
	EmitEventOnDelete(deleteObj client.Object, err error)
	EmitEventOnCreate(createObj client.Object, err error)
	EmitEventOnPatch(patchObj client.Object, err error)
	EmitEventOnList(listObj client.ObjectList, err error)
}

// GenericEventEmitter can be used for any case where the state change isn't handled by reader,writer or any custom event.
type GenericEventEmitter interface {
	EmitEventGeneric(eventReason, msg string, err error)
}

// EmitEventGeneric shall emit a generic event
func (e *EmitEventWrapper) EmitEventGeneric(eventReason, msg string, err error) {
	if err != nil {
		e.Event(e.subject, corev1.EventTypeWarning, eventReason, err.Error())
	} else if msg != "" {
		e.Event(e.subject, corev1.EventTypeNormal, eventReason, msg)
	}
}

func (e *EmitEventWrapper) EmitEventRollingDeployWait(k8sObj client.Object) {
	switch v := k8sObj.(type) {
	case *appsv1.StatefulSet:
		msg := fmt.Sprintf("StatefulSet roll out is in progress CurrentRevision[%s] != UpdateRevision[%s]", v.Status.CurrentRevision, v.Status.UpdateRevision)
		e.Event(e.subject, v1.EventTypeNormal, string(rollingDeployWait), msg)
	case *appsv1.Deployment:
		msg := fmt.Sprintf("Deployment[%s] roll out is in progress in namespace [%s], ReadyReplicas [%d] != Current Replicas [%d]", v.Name, k8sObj.GetNamespace(), v.Status.ReadyReplicas, v.Status.Replicas)
		e.Event(e.subject, v1.EventTypeNormal, string(rollingDeployWait), msg)
	default:
		e.Event(e.subject, v1.EventTypeNormal, string(rollingDeployWait), "RollingDeployWait")
	}
}

// EmitEventOnCreate shall emit event on CREATE operation
func (e *EmitEventWrapper) EmitEventOnCreate(createObj client.Object, err error) {
	if err != nil {
		errMsg := fmt.Errorf("Error creating object [%s:%s] in namespace [%s] due to [%s]", createObj.GetName(), createObj.GetObjectKind().GroupVersionKind().Kind, createObj.GetNamespace(), err.Error())
		e.Event(e.subject, corev1.EventTypeWarning, string(createFail), errMsg.Error())
	} else {
		msg := fmt.Sprintf("Successfully created object [%s:%s] in namespace [%s]", createObj.GetName(), createObj.GetObjectKind().GroupVersionKind().Kind, createObj.GetNamespace())
		e.Event(e.subject, corev1.EventTypeNormal, string(createSuccess), msg)
	}
}

// EmitEventOnPatch shall emit event on PATCH operation
func (e *EmitEventWrapper) EmitEventOnPatch(patchObj client.Object, err error) {
	if err != nil {
		errMsg := fmt.Errorf("error patching object [%s:%s] in namespace [%s] due to [%s]", patchObj.GetName(), patchObj.GetObjectKind().GroupVersionKind().Kind, patchObj.GetNamespace(), err.Error())
		e.Event(e.subject, v1.EventTypeWarning, string(patchFail), errMsg.Error())
	} else {
		msg := fmt.Sprintf("successfully patched object [%s:%s] in namespace [%s]", patchObj.GetName(), patchObj.GetObjectKind().GroupVersionKind().Kind, patchObj.GetNamespace())
		e.Event(e.subject, v1.EventTypeNormal, string(patchSuccess), msg)
	}
}

// EmitEventOnUpdate shall emit event on UPDATE operation
func (e *EmitEventWrapper) EmitEventOnUpdate(updateObj client.Object, err error) {
	if err != nil {
		errMsg := fmt.Errorf("failed to update [%s:%s] due to [%s]", updateObj.GetName(), updateObj.GetObjectKind().GroupVersionKind().Kind, err.Error())
		e.Event(e.subject, v1.EventTypeWarning, string(updateFail), errMsg.Error())
	} else {
		msg := fmt.Sprintf("updated [%s:%s].", updateObj.GetName(), updateObj.GetObjectKind().GroupVersionKind().Kind)
		e.Event(e.subject, v1.EventTypeNormal, string(updateSuccess), msg)
	}
}

// EmitEventOnGetError shall emit event on GET err operation
func (e *EmitEventWrapper) EmitEventOnGetError(getObj client.Object, err error) {
	getErr := fmt.Errorf("failed to get [Object:%s] due to [%s]", getObj.GetName(), err.Error())
	e.Event(e.subject, v1.EventTypeWarning, string(objectGetFail), getErr.Error())
}

//  EmitEventOnList shall emit event on LIST err operation
func (e *EmitEventWrapper) EmitEventOnList(listObj client.ObjectList, err error) {
	if err != nil {
		errMsg := fmt.Errorf("error listing object [%s] in namespace [%s] due to [%s]", listObj.GetObjectKind().GroupVersionKind().Kind, e.subject.GetNamespace(), err.Error())
		e.Event(e.subject, v1.EventTypeWarning, string(objectListFail), errMsg.Error())
	}
}

// EmitEventOnDelete shall emit event on DELETE operation
func (e *EmitEventWrapper) EmitEventOnDelete(deleteObj client.Object, err error) {
	if err != nil {
		errMsg := fmt.Errorf("Error deleting object [%s:%s] in namespace [%s] due to [%s]", deleteObj.GetObjectKind().GroupVersionKind().Kind, deleteObj.GetName(), deleteObj.GetNamespace(), err.Error())
		e.Event(e.subject, v1.EventTypeWarning, string(deleteFail), errMsg.Error())
	} else {
		msg := fmt.Sprintf("Successfully deleted object [%s:%s] in namespace [%s]", deleteObj.GetName(), deleteObj.GetObjectKind().GroupVersionKind().Kind, deleteObj.GetNamespace())
		e.Event(e.subject, v1.EventTypeNormal, string(deleteSuccess), msg)
	}
}
