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

func (r *DNSet) setupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-core-matrixorigin-io-v1alpha1-dnset,mutating=true,failurePolicy=fail,sideEffects=None,groups=core.matrixorigin.io,resources=dnsets,verbs=create;update,versions=v1alpha1,name=mdnset.kb.io,admissionReviewVersions={v1,v1beta1}

var _ webhook.Defaulter = &DNSet{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *DNSet) Default() {
	r.Spec.DNSetBasic.Default()
}

func (r *DNSetBasic) Default() {
	if r.Resources.Requests.Memory() != nil && r.SharedStorageCache.MemoryCacheSize == nil {
		// default memory cache size to 50% request memory
		size := r.Resources.Requests.Memory().DeepCopy()
		size.Set(size.Value() / 2)
		r.SharedStorageCache.MemoryCacheSize = &size
	}
	if r.CacheVolume != nil && r.SharedStorageCache.DiskCacheSize == nil {
		// default disk cache size to the cache volume size
		r.SharedStorageCache.DiskCacheSize = &r.CacheVolume.Size
	}
}

// +kubebuilder:webhook:path=/validate-core-matrixorigin-io-v1alpha1-dnset,mutating=false,failurePolicy=fail,sideEffects=None,groups=core.matrixorigin.io,resources=dnsets,verbs=create;update,versions=v1alpha1,name=vdnset.kb.io,admissionReviewVersions={v1,v1beta1}

var _ webhook.Validator = &DNSet{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *DNSet) ValidateCreate() error {
	var errs field.ErrorList
	errs = append(errs, validateLogSetRef(&r.Deps.LogSetRef, field.NewPath("deps"))...)
	errs = append(errs, r.Spec.DNSetBasic.ValidateCreate()...)
	errs = append(errs, validateMainContainer(&r.Spec.MainContainer, field.NewPath("spec"))...)
	return invalidOrNil(errs, r)
}

func (r *DNSet) ValidateUpdate(old runtime.Object) error {
	if err := r.ValidateCreate(); err != nil {
		return err
	}
	return nil
}

func (r *DNSet) ValidateDelete() error {
	return nil
}

func (r *DNSetBasic) ValidateCreate() field.ErrorList {
	var errs field.ErrorList
	if r.CacheVolume != nil {
		errs = append(errs, validateVolume(r.CacheVolume, field.NewPath("spec").Child("cacheVolume"))...)
	}
	return errs
}
