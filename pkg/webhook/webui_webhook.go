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

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
)

type webUIWebhook struct{}

func (webUIWebhook) setupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&v1alpha1.WebUI{}).
		WithDefaulter(&webUIDefaulter{}).
		WithValidator(&webUIValidator{}).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-core-matrixorigin-io-v1alpha1-webui,mutating=true,failurePolicy=fail,sideEffects=None,groups=core.matrixorigin.io,resources=webuis,verbs=create;update,versions=v1alpha1,name=mwebui.kb.io,admissionReviewVersions={v1,v1beta1}

// webUIDefaulter implements webhook.Defaulter so a webhook will be registered for the v1alpha1.WebUI
type webUIDefaulter struct{}

var _ webhook.CustomDefaulter = &webUIDefaulter{}

func (w *webUIDefaulter) Default(_ context.Context, obj runtime.Object) error {
	return nil
}

// +kubebuilder:webhook:path=/validate-core-matrixorigin-io-v1alpha1-webui,mutating=false,failurePolicy=fail,sideEffects=None,groups=core.matrixorigin.io,resources=webuis,verbs=create;update,versions=v1alpha1,name=vwebui.kb.io,admissionReviewVersions={v1,v1beta1}

// webUIValidator implements webhook.Validator so a webhook will be registered for the v1alpha1.WebUI
type webUIValidator struct{}

var _ webhook.CustomValidator = &webUIValidator{}

func (w *webUIValidator) ValidateCreate(_ context.Context, obj runtime.Object) (warnings admission.Warnings, err error) {
	webui, ok := obj.(*v1alpha1.WebUI)
	if !ok {
		return nil, unexpectedKindError("WebUI", obj)
	}
	var errs field.ErrorList
	errs = append(errs, validateMainContainer(&webui.Spec.MainContainer, field.NewPath("spec"))...)
	return nil, invalidOrNil(errs, webui)
}

func (w *webUIValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (warnings admission.Warnings, err error) {
	warnings, err = w.ValidateCreate(ctx, newObj)
	if err != nil {
		return warnings, err
	}
	return warnings, nil
}

func (w *webUIValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (warnings admission.Warnings, err error) {
	return nil, nil
}
