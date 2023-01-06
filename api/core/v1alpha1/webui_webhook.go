// Copyright 2023 Matrix Origin
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

package v1alpha1

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

func (r *WebUI) setupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-core-matrixorigin-io-v1alpha1-webui,mutating=true,failurePolicy=fail,sideEffects=None,groups=core.matrixorigin.io,resources=webuis,verbs=create;update,versions=v1alpha1,name=mwebui.kb.io,admissionReviewVersions={v1,v1beta1}

var _ webhook.Defaulter = &WebUI{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *WebUI) Default() {
}

// +kubebuilder:webhook:path=/validate-core-matrixorigin-io-v1alpha1-webui,mutating=false,failurePolicy=fail,sideEffects=None,groups=core.matrixorigin.io,resources=webuis,verbs=create;update,versions=v1alpha1,name=vwebui.kb.io,admissionReviewVersions={v1,v1beta1}

var _ webhook.Validator = &WebUI{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *WebUI) ValidateCreate() error {
	var errs field.ErrorList
	errs = append(errs, validateMainContainer(&r.Spec.MainContainer, field.NewPath("spec"))...)
	return invalidOrNil(errs, r)
}

func (r *WebUI) ValidateUpdate(old runtime.Object) error {
	if err := r.ValidateCreate(); err != nil {
		return err
	}
	return nil
}

func (r *WebUI) ValidateDelete() error {
	return nil
}
