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

package logs

import (
	"context"

	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/klog/v2"
	"k8s.io/klog/v2/klogr"
)

var (
	Log = klogr.New().WithName("matrixone-operator")
)

const (
	// Following analog to https://github.com/kubernetes/community/blob/master/contributors/devel/sig-instrumentation/logging.md
	ErrorLevel        = 0
	WarnLevel         = 1
	InfoLevel         = 2
	ExtendedInfoLevel = 3
	DebugLevel        = 4
	TraceLevel        = 5

	ResourceName      = "resource_name"
	ResourceNamespace = "resource_namespace"
	ResourceKind      = "resource_kind"
	ResourceVersion   = "resource_version"
	ResourceUID       = "resource_uid"
)

func Info(msg string) {
	klog.Info(msg)
}

func V(level int) klog.Verbose {
	return klog.V(klog.Level(level))
}

func WithResource(l logr.Logger, obj metav1.Object, scheme *runtime.Scheme) logr.Logger {
	var gvk schema.GroupVersionKind

	if runtimeObj, ok := obj.(runtime.Object); ok {
		gvks, _, _ := scheme.ObjectKinds(runtimeObj)
		if len(gvks) > 0 {
			gvk = gvks[0]
		}
	}

	return l.WithValues(
		ResourceName, obj.GetName(),
		ResourceNamespace, obj.GetNamespace(),
		ResourceKind, gvk.Kind,
		ResourceVersion, gvk.Version,
	)
}

func FromContext(ctx context.Context, names ...string) logr.Logger {
	l, err := logr.FromContext(ctx)
	if err != nil {
		l = Log
	}
	for _, n := range names {
		l = l.WithName(n)
	}
	return l
}

func NewContext(ctx context.Context, l logr.Logger, names ...string) context.Context {
	for _, n := range names {
		l = l.WithName(n)
	}
	return logr.NewContext(ctx, l)
}
