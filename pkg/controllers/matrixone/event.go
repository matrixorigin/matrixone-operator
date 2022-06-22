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

	"github.com/matrixorigin/matrixone-operator/pkg/actor"
	"github.com/matrixorigin/matrixone-operator/pkg/state"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/record"
)

type EventEmitter interface {
	K8sEventEmitter
	GenericEventEmitter
}

type EmitEventFuncs struct {
	record.EventRecorder
	actor.ActorFunc
	state.StateTransFunc
}

type K8sEventEmitter interface {
	EmitEventHandler(
		sObj, tObj object,
		actionType matrixoneActionType,
		err error)
	EmitEventOnList(obj object, listObj objectList, err error)
}

// GenericEventEmitter can be used for any case where the state change isn't handled by reader,writer or any custom event.
type GenericEventEmitter interface {
	EmitEventGeneric(obj object, actionType matrixoneActionType, msg string, err error)
}

// EmitEventGeneric shall emit a generic event
func (e EmitEventFuncs) EmitEventGeneric(obj object, actionType matrixoneActionType, msg string, err error) {
	if err != nil {
		e.Event(obj, corev1.EventTypeWarning, string(actionType), err.Error())
	} else if msg != "" {
		e.Event(obj, corev1.EventTypeNormal, string(actionType), msg)
	}
}

func (e EmitEventFuncs) EmitEventHandler(
	sObj, tObj object,
	actionType matrixoneActionType,
	err error) {
	if err != nil {
		fmsg := fmt.Errorf("[%s:%s] for object [%s:%s:%s] in namespace [%s]",
			actionType,
			matrixoneActionFailed,
			tObj.GetName(),
			tObj.GetObjectKind().GroupVersionKind().Kind,
			tObj.GetUID(),
			tObj.GetNamespace())
		e.Event(sObj, corev1.EventTypeWarning, string(actionType), fmsg.Error())
	} else {
		smsg := fmt.Sprintf("[%s:%s] for object [%s:%s:%s] in namespace [%s]",
			actionType,
			matrixoneActionSuccessed,
			tObj.GetName(),
			tObj.GetObjectKind().GroupVersionKind().Kind,
			tObj.GetUID(),
			tObj.GetNamespace())
		e.Event(sObj, corev1.EventTypeNormal, string(actionType), smsg)

	}
}

//  EmitEventOnList shall emit event on LIST err operation
func (e EmitEventFuncs) EmitEventOnList(obj object, listObj objectList, err error) {
	if err != nil {
		errMsg := fmt.Errorf("error listing object [%s] in namespace [%s] due to [%s]",
			listObj.GetObjectKind().GroupVersionKind().Kind,
			obj.GetNamespace(),
			err.Error())
		e.Event(obj, v1.EventTypeWarning, string(matrixoneList), errMsg.Error())
	}
}
