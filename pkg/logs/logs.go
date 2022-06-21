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

package logs

import (
	"github.com/go-logr/logr"
	"github.com/matrixorigin/matrixone-operator/pkg/apis/matrixone/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	ObjectName      = "object_name"
	ObjectNamespace = "object_namespace"
	ObjectKind      = "object_kind"
	ObjectLabels    = "object_labels"
	ObjectUID       = "object_uid"
	ObjectVersion   = "object_version"
)

func WithObject(l logr.Logger, obj metav1.Object) logr.Logger {
	var gvk schema.GroupVersionKind

	if rtObj, ok := obj.(runtime.Object); ok {
		gvks, _, _ := v1alpha1.MoScheme.ObjectKinds(rtObj)
		if len(gvks) > 0 {
			gvk = gvks[0]
		}
	}

	return l.WithValues(
		ObjectName, obj.GetName(),
		ObjectNamespace, obj.GetNamespace(),
		ObjectLabels, obj.GetLabels(),
		ObjectUID, obj.GetUID(),
		ObjectKind, gvk.Kind,
		ObjectVersion, gvk.Version,
	)
}
