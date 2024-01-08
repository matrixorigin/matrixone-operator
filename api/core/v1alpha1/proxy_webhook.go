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

package v1alpha1

import (
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

func (r *ProxySet) setupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-core-matrixorigin-io-v1alpha1-proxyset,mutating=true,failurePolicy=fail,sideEffects=None,groups=core.matrixorigin.io,resources=proxysets,verbs=create;update,versions=v1alpha1,name=mproxyset.kb.io,admissionReviewVersions={v1,v1beta1}

var _ webhook.Defaulter = &ProxySet{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *ProxySet) Default() {
	r.Spec.Default()
}

func (r *ProxySetSpec) Default() {
	setDefaultServiceArgs(r)
}

// +kubebuilder:webhook:path=/validate-core-matrixorigin-io-v1alpha1-proxyset,mutating=false,failurePolicy=fail,sideEffects=None,groups=core.matrixorigin.io,resources=proxysets,verbs=create;update,versions=v1alpha1,name=vproxyset.kb.io,admissionReviewVersions={v1,v1beta1}

var _ webhook.Validator = &ProxySet{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *ProxySet) ValidateCreate() (admission.Warnings, error) {
	return nil, nil
}

func (r *ProxySet) ValidateUpdate(old runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

func (r *ProxySet) ValidateDelete() (admission.Warnings, error) {
	return nil, nil
}
