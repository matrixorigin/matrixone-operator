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
	"fmt"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

const (
	// MatrixOneClusterNameMaxLength is the maximum length of a MatrixOneCluster name
	MatrixOneClusterNameMaxLength = 46
)

// log is for logging in this package.
var moLog = logf.Log.WithName("mo-cluster")

func (r *MatrixOneCluster) setupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-core-matrixorigin-io-v1alpha1-matrixonecluster,mutating=true,failurePolicy=fail,sideEffects=None,groups=core.matrixorigin.io,resources=matrixoneclusters,verbs=create;update,versions=v1alpha1,name=mmatrixonecluster.kb.io,admissionReviewVersions=v1;v1beta1

var _ webhook.Defaulter = &MatrixOneCluster{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *MatrixOneCluster) Default() {
	r.Spec.LogService.Default()
	if r.Spec.DN != nil {
		r.Spec.DN.Default()
	}
	if r.Spec.TN != nil {
		r.Spec.TN.Default()
	}
	if r.Spec.TP != nil {
		r.Spec.TP.Default()
	}
	if r.Spec.AP != nil {
		r.Spec.AP.Default()
	}
	for i := range r.Spec.CNGroups {
		r.Spec.CNGroups[i].Default()
	}
}

// +kubebuilder:webhook:path=/validate-core-matrixorigin-io-v1alpha1-matrixonecluster,mutating=false,failurePolicy=fail,sideEffects=None,groups=core.matrixorigin.io,resources=matrixoneclusters,verbs=create;update,versions=v1alpha1,name=vmatrixonecluster.kb.io,admissionReviewVersions=v1;v1beta1

var _ webhook.Validator = &MatrixOneCluster{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *MatrixOneCluster) ValidateCreate() (admission.Warnings, error) {
	var errs field.ErrorList
	if len(r.Name) > MatrixOneClusterNameMaxLength {
		errs = append(errs, field.Invalid(field.NewPath("metadata").Child("name"), r.Name, fmt.Sprintf("must be no more than %d characters", MatrixOneClusterNameMaxLength)))
	}
	dns1035ErrorList := validation.IsDNS1035Label(r.Name)
	for _, errMsg := range dns1035ErrorList {
		errs = append(errs, field.Invalid(field.NewPath("metadata").Child("name"), r.Name, errMsg))
	}
	if r.Spec.DN == nil && r.Spec.TN == nil {
		errs = append(errs, field.Invalid(field.NewPath("spec").Child("tn"), "", ".spec.tn must be set"))
	}
	if r.Spec.DN != nil && r.Spec.TN != nil {
		errs = append(errs, field.Invalid(field.NewPath("spec").Child("dn"), "", "legacy component .spec.dn cannot be set when .spec.tn is set"))
	}
	errs = append(errs, r.validateMutateCommon()...)
	errs = append(errs, r.Spec.LogService.ValidateCreate(LogSetKey(r))...)
	return nil, invalidOrNil(errs, r)
}

func (r *MatrixOneCluster) ValidateUpdate(o runtime.Object) (admission.Warnings, error) {
	var errs field.ErrorList
	errs = append(errs, r.validateMutateCommon()...)

	old := o.(*MatrixOneCluster)
	errs = append(errs, r.Spec.LogService.ValidateUpdate(&old.Spec.LogService, LogSetKey(r))...)
	return nil, invalidOrNil(errs, r)
}

func (r *MatrixOneCluster) validateMutateCommon() field.ErrorList {
	var errs field.ErrorList
	errs = append(errs, r.GetTN().ValidateCreate()...)
	groups := map[string]bool{}
	if r.Spec.TP != nil {
		errs = append(errs, r.Spec.TP.ValidateCreate()...)
		groups["tp"] = true
	}
	if r.Spec.AP != nil {
		errs = append(errs, r.Spec.AP.ValidateCreate()...)
		groups["ap"] = true
	}
	for i, cn := range r.Spec.CNGroups {
		errs = append(errs, r.validateCNGroup(cn, field.NewPath("spec").Child("cnGroups").Index(i))...)
		if groups[cn.Name] {
			errs = append(errs, field.Invalid(field.NewPath("spec").Child("cnGroups").Index(i).Child("name"), cn.Name, "name must be unique"))
		}
		groups[cn.Name] = true
	}
	if r.Spec.Version == "" {
		errs = append(errs, field.Invalid(field.NewPath("spec").Child("version"), "", "version must be set"))
	}
	return errs
}

func (r *MatrixOneCluster) validateCNGroup(g CNGroup, parent *field.Path) field.ErrorList {
	var errs field.ErrorList
	if es := validation.IsDNS1123Subdomain(g.Name); es != nil {
		for _, err := range es {
			errs = append(errs, field.Invalid(parent.Child("name"), g.Name, err))
		}
	}
	errs = append(errs, g.CNSetSpec.ValidateCreate()...)
	return errs
}

func (r *MatrixOneCluster) ValidateDelete() (admission.Warnings, error) {
	return nil, nil
}
