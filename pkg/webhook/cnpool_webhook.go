// Copyright 2025-2026 Matrix Origin
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

	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	"github.com/matrixorigin/matrixone-operator/pkg/controllers/common"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

type cnPoolWebhook struct{}

func (cnPoolWebhook) setupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&v1alpha1.CNPool{}).
		WithDefaulter(&cnPoolDefaulter{}).
		WithValidator(&cnPoolValidator{}).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-core-matrixorigin-io-v1alpha1-cnpool,mutating=true,failurePolicy=fail,sideEffects=None,groups=core.matrixorigin.io,resources=cnpools,verbs=create;update,versions=v1alpha1,name=mcnpool.kb.io,admissionReviewVersions={v1,v1beta1}

type cnPoolDefaulter struct{}

var _ webhook.CustomDefaulter = &cnPoolDefaulter{}

func (c *cnPoolDefaulter) Default(_ context.Context, obj runtime.Object) error {
	p, ok := obj.(*v1alpha1.CNPool)
	if !ok {
		return unexpectedKindError("CNPool", obj)
	}
	(&cnSetDefaulter{}).DefaultSpec(&p.Spec.Template)
	return nil
}

// +kubebuilder:webhook:path=/validate-core-matrixorigin-io-v1alpha1-cnpool,mutating=false,failurePolicy=fail,sideEffects=None,groups=core.matrixorigin.io,resources=cnpools,verbs=create;update,versions=v1alpha1,name=vcnpool.kb.io,admissionReviewVersions={v1,v1beta1}

type cnPoolValidator struct{}

var _ webhook.CustomValidator = &cnPoolValidator{}

func (c *cnPoolValidator) ValidateCreate(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	p, ok := obj.(*v1alpha1.CNPool)
	if !ok {
		return nil, unexpectedKindError("CNPool", obj)
	}
	return nil, invalidOrNil(c.validate(p), p)
}

func (c *cnPoolValidator) ValidateUpdate(ctx context.Context, _, newObj runtime.Object) (admission.Warnings, error) {
	return c.ValidateCreate(ctx, newObj)
}

func (c *cnPoolValidator) ValidateDelete(_ context.Context, _ runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

func (c *cnPoolValidator) validate(p *v1alpha1.CNPool) field.ErrorList {
	var errs field.ErrorList
	root := field.NewPath("spec")
	errs = append(errs, validateLogSetRef(&p.Spec.Deps.LogSetRef, root.Child("deps"))...)
	cn := &cnSetValidator{}
	errs = append(errs, cn.ValidateSpecCreateAt(&p.Spec.Template, root.Child("template"))...)
	errs = append(errs, validatePodSet(&p.Spec.Template.PodSet, root.Child("template"))...)
	if policy := p.Spec.Template.UDFWorker; policy != nil && policy.Enabled {
		for key := range p.Spec.PodLabels {
			if common.IsUDFWorkerControllerOwnedPodLabel(key) {
				errs = append(errs, field.Forbidden(root.Child("podLabels").Key(key),
					"PythonEnabledCNPoolDisallowsControllerOwnedPodLabel"))
			}
		}
	}
	if p.Spec.Strategy.ScaleStrategy.MaxIdle < 0 {
		errs = append(errs, field.Invalid(root.Child("strategy", "scaleStrategy", "maxIdle"),
			p.Spec.Strategy.ScaleStrategy.MaxIdle, "maxIdle must not be negative"))
	}
	if p.Spec.Strategy.ScaleStrategy.MaxPods != nil && *p.Spec.Strategy.ScaleStrategy.MaxPods < 0 {
		errs = append(errs, field.Invalid(root.Child("strategy", "scaleStrategy", "maxPods"),
			*p.Spec.Strategy.ScaleStrategy.MaxPods, "maxPods must not be negative"))
	}
	return errs
}
