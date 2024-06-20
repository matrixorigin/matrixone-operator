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
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
)

type proxySetWebhook struct{}

func (proxySetWebhook) setupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&v1alpha1.ProxySet{}).
		WithDefaulter(&proxySetDefaulter{}).
		WithValidator(&proxySetValidator{}).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-core-matrixorigin-io-v1alpha1-proxyset,mutating=true,failurePolicy=fail,sideEffects=None,groups=core.matrixorigin.io,resources=proxysets,verbs=create;update,versions=v1alpha1,name=mproxyset.kb.io,admissionReviewVersions={v1,v1beta1}

// proxySetDefaulter implements webhook.CustomDefaulter so a webhook will be registered for v1alpha1.ProxySet
type proxySetDefaulter struct{}

var _ webhook.CustomDefaulter = &proxySetDefaulter{}

func (p *proxySetDefaulter) Default(_ context.Context, obj runtime.Object) error {
	proxySet, ok := obj.(*v1alpha1.ProxySet)
	if !ok {
		return unexpectedKindError("ProxySet", obj)
	}
	p.DefaultSpec(&proxySet.Spec)
	return nil
}

func (p *proxySetDefaulter) DefaultSpec(spec *v1alpha1.ProxySetSpec) {
	setDefaultServiceArgs(spec)
}

// +kubebuilder:webhook:path=/validate-core-matrixorigin-io-v1alpha1-proxyset,mutating=false,failurePolicy=fail,sideEffects=None,groups=core.matrixorigin.io,resources=proxysets,verbs=create;update,versions=v1alpha1,name=vproxyset.kb.io,admissionReviewVersions={v1,v1beta1}

// proxySetValidator implements webhook.Validator so a webhook will be registered for v1alpha1.ProxySet
type proxySetValidator struct{}

var _ webhook.CustomValidator = &proxySetValidator{}

func (p proxySetValidator) ValidateCreate(_ context.Context, _ runtime.Object) (warnings admission.Warnings, err error) {
	return nil, nil
}

func (p proxySetValidator) ValidateUpdate(_ context.Context, _, _ runtime.Object) (warnings admission.Warnings, err error) {
	return nil, nil
}

func (p proxySetValidator) ValidateDelete(_ context.Context, _ runtime.Object) (warnings admission.Warnings, err error) {
	return nil, nil
}
