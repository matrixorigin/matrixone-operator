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
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
)

const (
	// MatrixOneClusterNameMaxLength is the maximum length of a MatrixOneCluster name
	MatrixOneClusterNameMaxLength = 46
)

// log is for logging in this package.
var moLog = logf.Log.WithName("mo-cluster")

type matrixOneClusterWebhook struct{}

func (matrixOneClusterWebhook) setupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&v1alpha1.MatrixOneCluster{}).
		WithDefaulter(&matrixOneClusterDefaulter{
			cn:         &cnSetDefaulter{},
			dn:         &dnSetDefaulter{},
			logService: &logSetDefaulter{},
		}).
		WithValidator(&matrixOneClusterValidator{}).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-core-matrixorigin-io-v1alpha1-matrixonecluster,mutating=true,failurePolicy=fail,sideEffects=None,groups=core.matrixorigin.io,resources=matrixoneclusters,verbs=create;update,versions=v1alpha1,name=mmatrixonecluster.kb.io,admissionReviewVersions=v1;v1beta1

// matrixOneClusterDefaulter implements webhook.Defaulter so a webhook will be registered for v1alpha1.MatrixOneCluster
type matrixOneClusterDefaulter struct {
	cn         *cnSetDefaulter
	dn         *dnSetDefaulter
	logService *logSetDefaulter
}

var _ webhook.CustomDefaulter = &matrixOneClusterDefaulter{}

func (m *matrixOneClusterDefaulter) Default(ctx context.Context, obj runtime.Object) error {
	moc, ok := obj.(*v1alpha1.MatrixOneCluster)
	if !ok {
		return unexpectedKindError("MatrixOneCluster", obj)
	}
	m.logService.DefaultSpec(&moc.Spec.LogService)
	if moc.Spec.DN != nil {
		m.dn.DefaultSpec(moc.Spec.DN)
	}
	if moc.Spec.TN != nil {
		m.dn.DefaultSpec(moc.Spec.TN)
	}
	if moc.Spec.TP != nil {
		m.cn.DefaultSpec(moc.Spec.TP)
	}
	if moc.Spec.AP != nil {
		m.cn.DefaultSpec(moc.Spec.AP)
	}
	for i := range moc.Spec.CNGroups {
		m.cn.DefaultSpec(&moc.Spec.CNGroups[i].CNSetSpec)
	}
	return nil
}

// +kubebuilder:webhook:path=/validate-core-matrixorigin-io-v1alpha1-matrixonecluster,mutating=false,failurePolicy=fail,sideEffects=None,groups=core.matrixorigin.io,resources=matrixoneclusters,verbs=create;update,versions=v1alpha1,name=vmatrixonecluster.kb.io,admissionReviewVersions=v1;v1beta1

// matrixOneClusterValidator implements webhook.Validator so a webhook will be registered for v1alpha1.MatrixOneCluster
type matrixOneClusterValidator struct {
	cn         *cnSetValidator
	dn         *dnSetValidator
	logService *logSetValidator
}

var _ webhook.CustomValidator = &matrixOneClusterValidator{}

func (m *matrixOneClusterValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (warnings admission.Warnings, err error) {
	moc, ok := obj.(*v1alpha1.MatrixOneCluster)
	if !ok {
		return nil, unexpectedKindError("MatrixOneCluster", obj)
	}
	var errs field.ErrorList
	if len(moc.Name) > MatrixOneClusterNameMaxLength {
		errs = append(errs, field.Invalid(field.NewPath("metadata").Child("name"), moc.Name, fmt.Sprintf("must be no more than %d characters", MatrixOneClusterNameMaxLength)))
	}
	dns1035ErrorList := validation.IsDNS1035Label(moc.Name)
	for _, errMsg := range dns1035ErrorList {
		errs = append(errs, field.Invalid(field.NewPath("metadata").Child("name"), moc.Name, errMsg))
	}
	if moc.Spec.DN == nil && moc.Spec.TN == nil {
		errs = append(errs, field.Invalid(field.NewPath("spec").Child("tn"), "", ".spec.tn must be set"))
	}
	if moc.Spec.DN != nil && moc.Spec.TN != nil {
		errs = append(errs, field.Invalid(field.NewPath("spec").Child("dn"), "", "legacy component .spec.dn cannot be set when .spec.tn is set"))
	}
	errs = append(errs, m.validateMutateCommon(moc)...)
	//errs = append(errs, r.Spec.LogService.ValidateCreate(LogSetKey(r))...)
	errs = append(errs, m.logService.ValidateSpecCreate(v1alpha1.LogSetKey(moc), &moc.Spec.LogService)...)
	return nil, invalidOrNil(errs, moc)
}

func (m *matrixOneClusterValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (warnings admission.Warnings, err error) {
	var errs field.ErrorList
	moc := newObj.(*v1alpha1.MatrixOneCluster)
	errs = append(errs, m.validateMutateCommon(moc)...)

	old := oldObj.(*v1alpha1.MatrixOneCluster)
	errs = append(errs, m.logService.ValidateSpecUpdate(&old.Spec.LogService, &moc.Spec.LogService, v1alpha1.LogSetKey(moc))...)
	return nil, invalidOrNil(errs, moc)
}

func (m *matrixOneClusterValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (warnings admission.Warnings, err error) {
	return nil, nil
}

func (m *matrixOneClusterValidator) validateMutateCommon(moc *v1alpha1.MatrixOneCluster) field.ErrorList {
	var errs field.ErrorList
	errs = append(errs, m.dn.ValidateSpecCreate(moc.GetTN())...)
	groups := map[string]bool{}
	if moc.Spec.TP != nil {
		errs = append(errs, m.cn.ValidateSpecCreate(moc.Spec.TP)...)
		groups["tp"] = true
	}
	if moc.Spec.AP != nil {
		errs = append(errs, m.cn.ValidateSpecCreate(moc.Spec.AP)...)
		groups["ap"] = true
	}

	for i, cn := range moc.Spec.CNGroups {
		errs = append(errs, m.validateCNGroup(cn, field.NewPath("spec").Child("cnGroups").Index(i))...)
		if groups[cn.Name] {
			errs = append(errs, field.Invalid(field.NewPath("spec").Child("cnGroups").Index(i).Child("name"), cn.Name, "name must be unique"))
		}
		groups[cn.Name] = true
	}
	if moc.Spec.Version == "" {
		errs = append(errs, field.Invalid(field.NewPath("spec").Child("version"), "", "version must be set"))
	}
	return errs
}

func (m *matrixOneClusterValidator) validateCNGroup(g v1alpha1.CNGroup, parent *field.Path) field.ErrorList {
	var errs field.ErrorList
	if es := validation.IsDNS1123Subdomain(g.Name); es != nil {
		for _, err := range es {
			errs = append(errs, field.Invalid(parent.Child("name"), g.Name, err))
		}
	}
	errs = append(errs, m.cn.ValidateSpecCreate(&g.CNSetSpec)...)
	return errs
}
