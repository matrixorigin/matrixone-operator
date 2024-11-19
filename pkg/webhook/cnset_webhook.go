// Copyright 2024 Matrix Origin
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

package webhook

import (
	"context"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
)

const (
	defaultStoreDrainTimeout = 5 * time.Minute

	defaultMaxSurge       = 1
	defaultMaxUnavailable = 0
)

type cnSetWebhook struct{}

func (cnSetWebhook) setupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&v1alpha1.CNSet{}).
		WithDefaulter(&cnSetDefaulter{}).
		WithValidator(&cnSetValidator{}).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-core-matrixorigin-io-v1alpha1-cnset,mutating=true,failurePolicy=fail,sideEffects=None,groups=core.matrixorigin.io,resources=cnsets,verbs=create;update,versions=v1alpha1,name=mcnset.kb.io,admissionReviewVersions={v1,v1beta1}

// cnSetDefaulter implements webhook.CustomDefaulter so a webhook will be registered for v1alpha1.CNSet
type cnSetDefaulter struct{}

var _ webhook.CustomDefaulter = &cnSetDefaulter{}

func (c *cnSetDefaulter) Default(_ context.Context, obj runtime.Object) error {
	cnSet, ok := obj.(*v1alpha1.CNSet)
	if !ok {
		return unexpectedKindError("CNSet", obj)
	}

	c.DefaultSpec(&cnSet.Spec)
	if cnSet.Spec.Role == "" {
		cnSet.Spec.Role = v1alpha1.CNRoleTP
	}
	return nil
}

func (c *cnSetDefaulter) DefaultSpec(spec *v1alpha1.CNSetSpec) {
	if spec.ServiceType == "" {
		spec.ServiceType = corev1.ServiceTypeClusterIP
	}
	if spec.Resources.Requests.Memory().Value() != 0 && spec.SharedStorageCache.MemoryCacheSize == nil {
		// default memory cache size to 50% request memory
		size := spec.Resources.Requests.Memory().DeepCopy()
		size.Set(size.Value() / 2)
		spec.SharedStorageCache.MemoryCacheSize = &size
	}
	if spec.CacheVolume != nil && spec.SharedStorageCache.DiskCacheSize == nil {
		// default disk cache size based on the cache volume total size
		spec.SharedStorageCache.DiskCacheSize = defaultDiskCacheSize(&spec.CacheVolume.Size)
	}
	if spec.ScalingConfig.StoreDrainEnabled != nil && *spec.ScalingConfig.StoreDrainEnabled {
		if spec.ScalingConfig.StoreDrainTimeout == nil {
			spec.ScalingConfig.StoreDrainTimeout = &metav1.Duration{Duration: defaultStoreDrainTimeout}
		}
	}
	if spec.UpdateStrategy.MaxSurge == nil {
		maxSurge := intstr.FromInt(defaultMaxSurge)
		spec.UpdateStrategy.MaxSurge = &maxSurge
	}
	if spec.UpdateStrategy.MaxUnavailable == nil {
		maxUnavailable := intstr.FromInt(defaultMaxUnavailable)
		spec.UpdateStrategy.MaxUnavailable = &maxUnavailable
	}
	setDefaultServiceArgs(spec)
	setPodSetDefaults(&spec.PodSet)
}

// +kubebuilder:webhook:path=/validate-core-matrixorigin-io-v1alpha1-cnset,mutating=false,failurePolicy=fail,sideEffects=None,groups=core.matrixorigin.io,resources=cnsets,verbs=create;update,versions=v1alpha1,name=vcnset.kb.io,admissionReviewVersions={v1,v1beta1}

// cnSetValidator implements webhook.CustomValidator so a webhook will be registered for v1alpha1.CNSet
type cnSetValidator struct{}

var _ webhook.CustomValidator = &cnSetValidator{}

func (c *cnSetValidator) ValidateCreate(_ context.Context, obj runtime.Object) (warnings admission.Warnings, err error) {
	cnSet, ok := obj.(*v1alpha1.CNSet)
	if !ok {
		return nil, unexpectedKindError("CNSet", obj)
	}
	var errs field.ErrorList
	errs = append(errs, validateLogSetRef(&cnSet.Deps.LogSetRef, field.NewPath("deps"))...)
	errs = append(errs, c.ValidateSpecCreate(&cnSet.Spec)...)
	errs = append(errs, validatePodSet(&cnSet.Spec.PodSet, field.NewPath("spec"))...)
	return nil, invalidOrNil(errs, cnSet)
}

func (c *cnSetValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (warnings admission.Warnings, err error) {
	var errs field.ErrorList
	warnings, err = c.ValidateCreate(ctx, newObj)
	if err != nil {
		return warnings, err
	}
	oldCN, ok := oldObj.(*v1alpha1.CNSet)
	if !ok {
		return nil, unexpectedKindError("CNSet", oldObj)
	}
	newCN, ok := newObj.(*v1alpha1.CNSet)
	if !ok {
		return nil, unexpectedKindError("CNSet", newObj)
	}
	errs = append(errs, validatePodSetUpdate(&oldCN.Spec.PodSet, &newCN.Spec.PodSet, field.NewPath("spec"))...)
	errs = append(errs, validateVolumeUpdate(oldCN.Spec.CacheVolume, newCN.Spec.CacheVolume, field.NewPath("spec").Child("cacheVolume"))...)
	return nil, invalidOrNil(errs, newCN)
}

func (c *cnSetValidator) ValidateDelete(_ context.Context, _ runtime.Object) (warnings admission.Warnings, err error) {
	return nil, nil
}

func (c *cnSetValidator) ValidateSpecCreate(spec *v1alpha1.CNSetSpec) field.ErrorList {
	var errs field.ErrorList
	if spec.CacheVolume != nil {
		errs = append(errs, validateVolume(spec.CacheVolume, field.NewPath("spec").Child("cacheVolume"))...)
	}
	if spec.ServiceType == corev1.ServiceTypeExternalName {
		errs = append(errs, field.Invalid(field.NewPath("spec").Child("serviceType"), spec.ServiceType, "must be one of [ClusterIP, NodePort, LoadBalancer]"))
	}
	if spec.NodePort != nil && spec.ServiceType == corev1.ServiceTypeClusterIP {
		errs = append(errs, field.Invalid(field.NewPath("spec").Child("nodePort"), spec.NodePort, "cannot set node port when serviceType is ClusterIP"))
	}
	for i, l := range spec.Labels {
		if l.Key == "" {
			errs = append(errs, field.Invalid(field.NewPath("spec").Child("cnLabels").Index(i).Child("key"), spec.Labels[i], "label key cannot be empty"))
		}
		if len(l.Values) == 0 {
			errs = append(errs, field.Invalid(field.NewPath("spec").Child("cnLabels").Index(i).Child("values"), spec.Labels[i], "label values cannot be empty"))
		}
		for j, v := range l.Values {
			if v == "" {
				errs = append(errs, field.Invalid(field.NewPath("spec").Child("cnLabels").Index(i).Child("values").Index(j), spec.Labels[i].Values, "label value cannot be empty string"))
			}
		}
	}
	errs = append(errs, validateGoMemLimitPercent(spec.MemoryLimitPercent, field.NewPath("spec").Child("memoryLimitPercent"))...)
	return errs
}
