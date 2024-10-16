// Copyright 2024 Matrix Origin
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

package webhook

import (
	"context"
	"fmt"

	"github.com/go-errors/errors"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
)

// DefaultArgs alias to v1alpha1.DefaultArgs
type DefaultArgs = v1alpha1.DefaultArgs

// setDefaultServiceArgs set default args for service, we only set default args when there is service args config in service spec
func setDefaultServiceArgs(object interface{}) {
	if ServiceDefaultArgs == nil {
		return
	}
	switch obj := object.(type) {
	case *v1alpha1.LogSetSpec:
		// set default arguments only when user does not set any arguments
		if len(obj.ServiceArgs) == 0 {
			obj.ServiceArgs = ServiceDefaultArgs.LogService
		}
	case *v1alpha1.DNSetSpec:
		if len(obj.ServiceArgs) == 0 {
			obj.ServiceArgs = ServiceDefaultArgs.DN
		}
	case *v1alpha1.CNSetSpec:
		if len(obj.ServiceArgs) == 0 {
			obj.ServiceArgs = ServiceDefaultArgs.CN
		}
	case *v1alpha1.ProxySetSpec:
		if len(obj.ServiceArgs) == 0 {
			obj.ServiceArgs = ServiceDefaultArgs.Proxy
		}
	default:
		moLog.Error(fmt.Errorf("unknown type:%T", object), "expected types: *LogSetSpec, *DNSetSpec, *CNSetSpec")
		return
	}
}

// setPodSetDefaults set default values in pod set
func setPodSetDefaults(s *v1alpha1.PodSet) {
	if s.Overlay == nil {
		s.Overlay = &v1alpha1.Overlay{}
	}
	s.Overlay.Env = appendIfNotExist(s.Overlay.Env, corev1.EnvVar{Name: v1alpha1.EnvGoDebug, Value: v1alpha1.DefaultGODebug}, func(v corev1.EnvVar) string {
		return v.Name
	})
	s.OperatorVersion = pointer.String(v1alpha1.LatestOpVersion.String())
	if s.ExportToPrometheus != nil && *s.ExportToPrometheus {
		if s.PromDiscoveryScheme == nil {
			s.PromDiscoveryScheme = (*v1alpha1.PromDiscoveryScheme)(pointer.String(string(v1alpha1.PromDiscoverySchemeService)))
		}
	}
}

func appendIfNotExist[K comparable, V any](list []V, elem V, keyFunc func(V) K) []V {
	for _, o := range list {
		if keyFunc(o) == keyFunc(elem) {
			return list
		}
	}
	return append(list, elem)
}

func defaultDiskCacheSize(total *resource.Quantity) *resource.Quantity {
	// shrink the total size since a small amount of space will be used for filesystem and metadata
	shrunk := total.Value() * 9 / 10
	return resource.NewQuantity(shrunk, total.Format)
}

func unexpectedKindError(expected string, obj runtime.Object) error {
	return errors.Errorf("expected %s but received %T", expected, obj)
}

func setDefaultOperatorVersion(ctx context.Context, podSet *v1alpha1.PodSet) error {
	req, err := admission.RequestFromContext(ctx)
	if err != nil {
		return err
	}
	if req.AdmissionRequest.Operation == admissionv1.Create && podSet.OperatorVersion == nil {
		podSet.OperatorVersion = pointer.String(v1alpha1.LatestOpVersion.String())
	}
	return nil
}
