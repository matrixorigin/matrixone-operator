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
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
)

func invalidOrNil(allErrs field.ErrorList, r client.Object) error {
	if len(allErrs) == 0 {
		return nil
	}
	return apierrors.NewInvalid(r.GetObjectKind().GroupVersionKind().GroupKind(), r.GetName(), allErrs)
}

func validateLogSetRef(ref *v1alpha1.LogSetRef, parent *field.Path) field.ErrorList {
	var errs field.ErrorList
	if ref.LogSet == nil && ref.ExternalLogSet == nil {
		errs = append(errs, field.Invalid(parent, nil, "one of deps.logSet or deps.externalLogSet must be set"))
	}
	return errs
}

func validateMainContainer(c *v1alpha1.MainContainer, parent *field.Path) field.ErrorList {
	var errs field.ErrorList
	if c.Image == "" {
		errs = append(errs, field.Invalid(parent.Child("image"), c.Image, "image must be set"))
	}
	return errs
}

func validateContainerResource(r *corev1.ResourceRequirements, parent *field.Path) field.ErrorList {
	// TODO: use kubernetes/api/validation.ValidatePodSpec to perform through Validation after we migrate
	// webhooks out of api package
	return nil
}

func validateVolume(v *v1alpha1.Volume, parent *field.Path) field.ErrorList {
	var errs field.ErrorList
	if v.Size.IsZero() {
		errs = append(errs, field.Invalid(parent.Child("size"), v.Size, "size must not be zero"))
	}
	return errs
}

func validateGoMemLimitPercent(memPercent *int, path *field.Path) field.ErrorList {
	if memPercent == nil {
		return nil
	}
	var errs field.ErrorList
	if *memPercent <= 0 || *memPercent > 100 {
		errs = append(errs, field.Invalid(path, memPercent, "memoryLimitPercent value must be in interval (0, 100]"))
	}
	return errs
}
