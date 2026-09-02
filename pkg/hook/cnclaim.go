// Copyright 2025 Matrix Origin
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

package hook

import (
	"context"

	"github.com/go-logr/logr"
	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

type CNClaimHook struct {
	Client client.Client
	Logger logr.Logger
}

func NewCNClaimHook(cli client.Client, logger logr.Logger) *CNClaimHook {
	return &CNClaimHook{
		Client: cli,
		Logger: logger,
	}
}

func (h *CNClaimHook) ValidateCreate(_ context.Context, _ runtime.Object) (warnings admission.Warnings, err error) {
	return nil, nil
}

func (h *CNClaimHook) ValidateUpdate(_ context.Context, oldObj, newObj runtime.Object) (warnings admission.Warnings, err error) {
	var errs field.ErrorList
	oldClaim, ok := oldObj.(*v1alpha1.CNClaim)
	if !ok {
		return nil, nil
	}
	newClaim, ok := newObj.(*v1alpha1.CNClaim)
	if !ok {
		return nil, nil
	}
	if oldClaim.Spec.PodName != "" && oldClaim.Spec.PodName != newClaim.Spec.PodName {
		errs = append(errs, field.Invalid(field.NewPath("spec", "podName"), newClaim.Spec.PodName, "podName is immutable"))
	}
	return nil, invalidOrNil(errs, newClaim)
}

func (h *CNClaimHook) ValidateDelete(_ context.Context, _ runtime.Object) (warnings admission.Warnings, err error) {
	return nil, nil
}

func (h *CNClaimHook) Setup(mgr manager.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&v1alpha1.CNClaim{}).
		WithValidator(h).
		Complete()
}

func invalidOrNil(allErrs field.ErrorList, r client.Object) error {
	if len(allErrs) == 0 {
		return nil
	}
	return apierrors.NewInvalid(r.GetObjectKind().GroupVersionKind().GroupKind(), r.GetName(), allErrs)
}
