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

type dnSetWebhook struct{}

func (dnSetWebhook) setupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&v1alpha1.DNSet{}).
		WithDefaulter(&dnSetDefaulter{}).
		WithValidator(&dnSetValidator{}).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-core-matrixorigin-io-v1alpha1-dnset,mutating=true,failurePolicy=fail,sideEffects=None,groups=core.matrixorigin.io,resources=dnsets,verbs=create;update,versions=v1alpha1,name=mdnset.kb.io,admissionReviewVersions={v1,v1beta1}

// dnSetDefaulter implements webhook.CustomDefaulter so a webhook will be registered for the v1alpha1.DNSet
type dnSetDefaulter struct{}

var _ webhook.CustomDefaulter = &dnSetDefaulter{}

func (d *dnSetDefaulter) Default(_ context.Context, obj runtime.Object) error {
	dnSet, ok := obj.(*v1alpha1.DNSet)
	if !ok {
		return unexpectedKindError("DNSet", obj)
	}
	d.DefaultSpec(&dnSet.Spec)
	return nil
}

func (d *dnSetDefaulter) DefaultSpec(spec *v1alpha1.DNSetSpec) {
	if spec.Resources.Requests.Memory() != nil && spec.SharedStorageCache.MemoryCacheSize == nil {
		// default memory cache size to 50% request memory
		size := spec.Resources.Requests.Memory().DeepCopy()
		size.Set(size.Value() / 2)
		spec.SharedStorageCache.MemoryCacheSize = &size
	}
	if spec.CacheVolume != nil && spec.SharedStorageCache.DiskCacheSize == nil {
		// default disk cache size based on the cache volume total size
		spec.SharedStorageCache.DiskCacheSize = defaultDiskCacheSize(&spec.CacheVolume.Size)
	}
	setDefaultServiceArgs(spec)
	setPodSetDefaults(&spec.PodSet)
}

// +kubebuilder:webhook:path=/validate-core-matrixorigin-io-v1alpha1-dnset,mutating=false,failurePolicy=fail,sideEffects=None,groups=core.matrixorigin.io,resources=dnsets,verbs=create;update,versions=v1alpha1,name=vdnset.kb.io,admissionReviewVersions={v1,v1beta1}

// dnSetValidator implements webhook.CustomValidator so a webhook will be registered for v1alpha1.DNSet
type dnSetValidator struct{}

var _ webhook.CustomValidator = &dnSetValidator{}

func (d *dnSetValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (warnings admission.Warnings, err error) {
	dnSet, ok := obj.(*v1alpha1.DNSet)
	if !ok {
		return nil, unexpectedKindError("DNSet", obj)
	}
	var errs field.ErrorList
	errs = append(errs, validateLogSetRef(&dnSet.Deps.LogSetRef, field.NewPath("deps"))...)
	errs = append(errs, d.ValidateSpecCreate(&dnSet.Spec)...)
	errs = append(errs, validateMainContainer(&dnSet.Spec.MainContainer, field.NewPath("spec"))...)
	errs = append(errs, d.validateConfig(dnSet.Spec.Config, field.NewPath("spec").Child("config"))...)
	return nil, invalidOrNil(errs, dnSet)
}

func (d *dnSetValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (warnings admission.Warnings, err error) {
	warnings, err = d.ValidateCreate(ctx, newObj)
	if err != nil {
		return warnings, err
	}
	return warnings, nil
}

func (d *dnSetValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (warnings admission.Warnings, err error) {
	return nil, nil
}

func (d *dnSetValidator) ValidateSpecCreate(spec *v1alpha1.DNSetSpec) field.ErrorList {
	var errs field.ErrorList
	if spec.CacheVolume != nil {
		errs = append(errs, validateVolume(spec.CacheVolume, field.NewPath("spec").Child("cacheVolume"))...)
	}
	errs = append(errs, validateGoMemLimitPercent(spec.MemoryLimitPercent, field.NewPath("spec").Child("memoryLimitPercent"))...)
	return errs
}

func (d *dnSetValidator) validateConfig(c *v1alpha1.TomlConfig, path *field.Path) field.ErrorList {
	var errs field.ErrorList
	if c == nil {
		return errs
	}
	if c.Get("tn") != nil && c.Get("dn") != nil {
		errs = append(errs, field.Invalid(path, c, "[tn] and [dn] cannot be set at the same time"))
	}
	return errs
}
