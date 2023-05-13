// Copyright 2023 Matrix Origin
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

func (r *CNSet) setupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-core-matrixorigin-io-v1alpha1-cnset,mutating=true,failurePolicy=fail,sideEffects=None,groups=core.matrixorigin.io,resources=cnsets,verbs=create;update,versions=v1alpha1,name=mcnset.kb.io,admissionReviewVersions={v1,v1beta1}

var _ webhook.Defaulter = &CNSet{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *CNSet) Default() {
	r.Spec.Default()
	if r.Spec.Role == "" {
		r.Spec.Role = CNRoleTP
	}
}

func (r *CNSetSpec) Default() {
	if r.ServiceType == "" {
		r.ServiceType = corev1.ServiceTypeClusterIP
	}
	if r.Resources.Requests.Memory() != nil && r.SharedStorageCache.MemoryCacheSize == nil {
		// default memory cache size to 50% request memory
		size := r.Resources.Requests.Memory().DeepCopy()
		size.Set(size.Value() / 2)
		r.SharedStorageCache.MemoryCacheSize = &size
	}
	if r.CacheVolume != nil && r.SharedStorageCache.DiskCacheSize == nil {
		// default disk cache size based on the cache volume total size
		r.SharedStorageCache.DiskCacheSize = defaultDiskCacheSize(&r.CacheVolume.Size)
	}
	setDefaultServiceArgs(r)
}

// +kubebuilder:webhook:path=/validate-core-matrixorigin-io-v1alpha1-cnset,mutating=false,failurePolicy=fail,sideEffects=None,groups=core.matrixorigin.io,resources=cnsets,verbs=create;update,versions=v1alpha1,name=vcnset.kb.io,admissionReviewVersions={v1,v1beta1}

var _ webhook.Validator = &CNSet{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *CNSet) ValidateCreate() error {
	var errs field.ErrorList
	errs = append(errs, validateLogSetRef(&r.Deps.LogSetRef, field.NewPath("deps"))...)
	errs = append(errs, r.Spec.ValidateCreate()...)
	errs = append(errs, validateMainContainer(&r.Spec.MainContainer, field.NewPath("spec"))...)
	return invalidOrNil(errs, r)
}

func (r *CNSet) ValidateUpdate(old runtime.Object) error {
	if err := r.ValidateCreate(); err != nil {
		return err
	}
	return nil
}

func (r *CNSet) ValidateDelete() error {
	return nil
}

func (r *CNSetSpec) ValidateCreate() field.ErrorList {
	var errs field.ErrorList
	if r.CacheVolume != nil {
		errs = append(errs, validateVolume(r.CacheVolume, field.NewPath("spec").Child("cacheVolume"))...)
	}
	if r.ServiceType == corev1.ServiceTypeExternalName {
		errs = append(errs, field.Invalid(field.NewPath("spec").Child("serviceType"), r.ServiceType, "must be one of [ClusterIP, NodePort, LoadBalancer]"))
	}
	if r.NodePort != nil && r.ServiceType == corev1.ServiceTypeClusterIP {
		errs = append(errs, field.Invalid(field.NewPath("spec").Child("nodePort"), r.NodePort, "cannot set node port when serviceType is ClusterIP"))
	}
	for i, l := range r.Labels {
		if l.Key == "" {
			errs = append(errs, field.Invalid(field.NewPath("spec").Child("cnLabels").Index(i).Child("key"), r.Labels[i], "label key cannot be empty"))
		}
		if len(l.Values) == 0 {
			errs = append(errs, field.Invalid(field.NewPath("spec").Child("cnLabels").Index(i).Child("values"), r.Labels[i], "label values cannot be empty"))
		}
		for j, v := range l.Values {
			if v == "" {
				errs = append(errs, field.Invalid(field.NewPath("spec").Child("cnLabels").Index(i).Child("values").Index(j), r.Labels[i].Values, "label value cannot be empty string"))
			}
		}
	}
	return errs
}
