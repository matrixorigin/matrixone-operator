// Copyright 2022 Matrix Origin
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
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
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
	r.Spec.DN.Default()
	r.Spec.TP.Default()
	if r.Spec.AP != nil {
		r.Spec.AP.Default()
	}
}

// +kubebuilder:webhook:path=/validate-core-matrixorigin-io-v1alpha1-matrixonecluster,mutating=false,failurePolicy=fail,sideEffects=None,groups=core.matrixorigin.io,resources=matrixoneclusters,verbs=create;update,versions=v1alpha1,name=vmatrixonecluster.kb.io,admissionReviewVersions=v1;v1beta1

var _ webhook.Validator = &MatrixOneCluster{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *MatrixOneCluster) ValidateCreate() error {
	var errs field.ErrorList
	errs = append(errs, r.Spec.LogService.ValidateCreate()...)
	errs = append(errs, r.Spec.DN.ValidateCreate()...)
	errs = append(errs, r.Spec.TP.ValidateCreate()...)
	if r.Spec.AP != nil {
		errs = append(errs, r.Spec.AP.ValidateCreate()...)
	}
	if r.Spec.Version == "" {
		errs = append(errs, field.Invalid(field.NewPath("spec").Child("version"), "", "version must be set"))
	}
	return invalidOrNil(errs, r)
}

func (r *MatrixOneCluster) ValidateUpdate(o runtime.Object) error {
	if err := r.ValidateCreate(); err != nil {
		return err
	}
	var errs field.ErrorList
	old := o.(*MatrixOneCluster)
	errs = append(errs, r.Spec.LogService.ValidateUpdate(&old.Spec.LogService)...)
	return invalidOrNil(errs, r)
}

func (r *MatrixOneCluster) ValidateDelete() error {
	return nil
}
