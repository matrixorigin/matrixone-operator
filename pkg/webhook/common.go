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
	"fmt"
	"github.com/blang/semver/v4"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1validation "k8s.io/apimachinery/pkg/apis/meta/v1/validation"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/kubernetes/pkg/apis/core"
	corevalidation "k8s.io/kubernetes/pkg/apis/core/validation"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	"github.com/matrixorigin/matrixone-operator/pkg/webhook/convertor"
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
	errs = append(errs, validateResourceRequirements(&c.Resources, parent.Child("resources"))...)
	return errs
}

func validateContainerResource(_ *corev1.ResourceRequirements, _ *field.Path) field.ErrorList {
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

func validatePodSet(podSet *v1alpha1.PodSet, path *field.Path) field.ErrorList {
	var errs field.ErrorList
	if podSet.ClusterDomain != "" {
		errs = append(errs, validation.IsFullyQualifiedDomainName(path.Child("clusterDomain"), podSet.ClusterDomain)...)
	}
	if podSet.Overlay != nil {
		errs = append(errs, validateOverlays(podSet.Overlay, path.Child("overlay"))...)
	}
	if podSet.SemanticVersion != nil {
		_, err := semver.Parse(*podSet.SemanticVersion)
		if err != nil {
			errs = append(errs, field.Invalid(path.Child("semanticVersion"), *podSet.SemanticVersion, err.Error()))
		}
	}
	if podSet.ExportToPrometheus != nil && *podSet.ExportToPrometheus {
		if podSet.PromDiscoveryScheme == nil {
			errs = append(errs, field.Invalid(path.Child("promDiscoveryScheme"), "", "ExportToPrometheus is enabled but PromDiscoveryScheme is not provided"))
		} else if *podSet.PromDiscoveryScheme != v1alpha1.PromDiscoverySchemePod && *podSet.PromDiscoveryScheme != v1alpha1.PromDiscoverySchemeService {
			errs = append(errs, field.Invalid(path.Child("promDiscoveryScheme"), *podSet.PromDiscoveryScheme,
				fmt.Sprintf("invalid PromDiscoveryScheme, must be %s or %s", v1alpha1.PromDiscoverySchemePod, v1alpha1.PromDiscoverySchemeService)))
		}
	}

	errs = append(errs, validateMainContainer(&podSet.MainContainer, path.Child("mainContainer"))...)
	errs = append(errs, metav1validation.ValidateLabels(podSet.NodeSelector, path.Child("nodeSelector"))...)

	return errs
}

func validateOverlays(overlay *v1alpha1.Overlay, path *field.Path) field.ErrorList {
	var errs field.ErrorList
	errs = append(errs, metav1validation.ValidateLabels(overlay.PodLabels, path.Child("podLabels"))...)
	errs = append(errs, corevalidation.ValidateAnnotations(overlay.PodAnnotations, path.Child("podAnnotations"))...)
	if overlay.RuntimeClassName != nil {
		errs = append(errs, corevalidation.ValidateRuntimeClassName(*overlay.RuntimeClassName, path.Child("runtimeClassName"))...)
	}
	errs = append(errs, validateTolerations(overlay.Tolerations, path.Child("tolerations"))...)

	return errs
}

func validateTolerations(v1tolerations []corev1.Toleration, path *field.Path) field.ErrorList {
	tolerations, err := convertor.ConvertTolerations(v1tolerations)
	if err != nil {
		return field.ErrorList{field.Invalid(path, v1tolerations, err.Error())}
	}

	return corevalidation.ValidateTolerations(tolerations, path)
}

func validateResourceRequirements(v1requirements *corev1.ResourceRequirements, path *field.Path) field.ErrorList {
	requirements, err := convertor.ConvertResourceRequirements(v1requirements)
	if err != nil {
		return field.ErrorList{field.Invalid(path, v1requirements, err.Error())}
	}

	return corevalidation.ValidateResourceRequirements(requirements, makeClaimNameSet(requirements.Claims), path, corevalidation.PodValidationOptions{})
}

func makeClaimNameSet(claims []core.ResourceClaim) sets.String {
	claimNames := sets.NewString()
	for _, claim := range claims {
		claimNames.Insert(claim.Name)
	}
	return claimNames
}
