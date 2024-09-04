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

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1validation "k8s.io/apimachinery/pkg/apis/meta/v1/validation"
	"k8s.io/apimachinery/pkg/conversion"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/kubernetes/pkg/apis/core"
	apiscorev1 "k8s.io/kubernetes/pkg/apis/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	"github.com/matrixorigin/matrixone-operator/pkg/webhook/convertor"
	"github.com/matrixorigin/matrixone-operator/pkg/webhook/corevalidation"
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
	// validate Resources
	errs = append(errs, validateWithConvert(&c.Resources, parent.Child("resources"),
		apiscorev1.Convert_v1_ResourceRequirements_To_core_ResourceRequirements,
		func(rrs *core.ResourceRequirements, path *field.Path) field.ErrorList {
			return corevalidation.ValidateResourceRequirements(rrs, makeClaimNameSet(rrs.Claims), path, corevalidation.PodValidationOptions{})
		})...)

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
	if _, ok := podSet.GetSemVer(); !ok {
		errs = append(errs, field.Invalid(path.Child("semanticVersion"), podSet.SemanticVersion, "a valid semanticVersion must be set or the image tag must be valid sematic version instead"))
	}

	if podSet.ExportToPrometheus != nil && *podSet.ExportToPrometheus {
		promDiscoverySchemePath := path.Child("promDiscoveryScheme")
		if podSet.PromDiscoveryScheme == nil {
			errs = append(errs, field.Invalid(promDiscoverySchemePath, "", "ExportToPrometheus is enabled but PromDiscoveryScheme is not provided"))
		} else if *podSet.PromDiscoveryScheme != v1alpha1.PromDiscoverySchemePod && *podSet.PromDiscoveryScheme != v1alpha1.PromDiscoverySchemeService {
			errs = append(errs, field.Invalid(promDiscoverySchemePath, *podSet.PromDiscoveryScheme,
				fmt.Sprintf("invalid PromDiscoveryScheme, must be %s or %s", v1alpha1.PromDiscoverySchemePod, v1alpha1.PromDiscoverySchemeService)))
		}
	}

	errs = append(errs, validateMainContainer(&podSet.MainContainer, path)...)
	errs = append(errs, metav1validation.ValidateLabels(podSet.NodeSelector, path.Child("nodeSelector"))...)

	return errs
}

func validateOverlays(overlay *v1alpha1.Overlay, path *field.Path) field.ErrorList {
	var errs field.ErrorList

	errs = append(errs, validateMainContainerOverlay(&overlay.MainContainerOverlay, path)...)
	// validate Volumes
	var vols map[string]core.VolumeSource
	errs = append(errs, validateSliceWithConvert(overlay.Volumes, path.Child("volumes"),
		apiscorev1.Convert_v1_Volume_To_core_Volume,
		func(volumes []core.Volume, subPath *field.Path) (errList field.ErrorList) {
			vols, errList = corevalidation.ValidateVolumes(volumes, nil, subPath, corevalidation.PodValidationOptions{})
			return errList
		})...)

	// validate PVCs
	errs = append(errs, validateSliceWithConvert(overlay.VolumeClaims, path.Child("volumeClaims"),
		apiscorev1.Convert_v1_PersistentVolumeClaim_To_core_PersistentVolumeClaim, validateVolumeClaims)...)

	// validate InitContainers
	errs = append(errs, validateSliceWithConvert(overlay.InitContainers, path.Child("initContainers"),
		apiscorev1.Convert_v1_Container_To_core_Container,
		func(containers []core.Container, subPath *field.Path) field.ErrorList {
			return corevalidation.ValidateInitContainers(containers, vols, subPath)
		},
	)...)
	// validate SidecarContainers
	errs = append(errs, validateSliceWithConvert(overlay.SidecarContainers, path.Child("sidecarContainers"),
		apiscorev1.Convert_v1_Container_To_core_Container,
		func(containers []core.Container, subPath *field.Path) field.ErrorList {
			return corevalidation.ValidateInitContainers(containers, vols, subPath)
		},
	)...)

	if len(overlay.ServiceAccountName) > 0 {
		for _, msg := range corevalidation.ValidateServiceAccountName(overlay.ServiceAccountName, false) {
			errs = append(errs, field.Invalid(path.Child("serviceAccountName"), overlay.ServiceAccountName, msg))
		}
	}

	// validate SecurityContext
	errs = append(errs, validateWithConvert(overlay.SecurityContext, path.Child("securityContext"),
		apiscorev1.Convert_v1_PodSecurityContext_To_core_PodSecurityContext,
		func(psc *core.PodSecurityContext, subPath *field.Path) field.ErrorList {
			return corevalidation.ValidatePodSecurityContext(psc, &core.PodSpec{}, path, subPath, corevalidation.PodValidationOptions{})
		},
	)...)

	// validate ImagePullSecrets
	errs = append(errs, validateSliceWithConvert(overlay.ImagePullSecrets, path.Child("imagePullSecrets"),
		apiscorev1.Convert_v1_LocalObjectReference_To_core_LocalObjectReference, corevalidation.ValidateImagePullSecrets)...)

	// validate Affinity
	errs = append(errs, validateWithConvert(overlay.Affinity, path.Child("affinity"),
		apiscorev1.Convert_v1_Affinity_To_core_Affinity, corevalidation.ValidateAffinity)...)

	// validate Tolerations
	errs = append(errs, validateSliceWithConvert(overlay.Tolerations, path.Child("tolerations"),
		apiscorev1.Convert_v1_Toleration_To_core_Toleration, corevalidation.ValidateTolerations)...)

	if len(overlay.PriorityClassName) > 0 {
		for _, msg := range corevalidation.ValidatePriorityClassName(overlay.PriorityClassName, false) {
			errs = append(errs, field.Invalid(path.Child("priorityClassName"), overlay.PriorityClassName, msg))
		}
	}
	if overlay.TerminationGracePeriodSeconds != nil && *overlay.TerminationGracePeriodSeconds < 0 {
		errs = append(errs, field.Invalid(path.Child("terminationGracePeriodSeconds"), *overlay.TerminationGracePeriodSeconds, "must be greater than 0"))
	}

	// validate HostAlias
	errs = append(errs, validateSliceWithConvert(overlay.HostAliases, path.Child("hostAliases"),
		apiscorev1.Convert_v1_HostAlias_To_core_HostAlias, corevalidation.ValidateHostAliases)...)

	// validate TopologySpreadConstraints
	errs = append(errs, validateSliceWithConvert(overlay.TopologySpreadConstraints, path.Child("topologySpreadConstraints"),
		apiscorev1.Convert_v1_TopologySpreadConstraint_To_core_TopologySpreadConstraint, corevalidation.ValidateTopologySpreadConstraints)...)

	if overlay.RuntimeClassName != nil {
		errs = append(errs, corevalidation.ValidateRuntimeClassName(*overlay.RuntimeClassName, path.Child("runtimeClassName"))...)
	}

	// validate DNSConfig
	errs = append(errs, validateWithConvert(overlay.DNSConfig, path.Child("dnsConfig"),
		apiscorev1.Convert_v1_PodDNSConfig_To_core_PodDNSConfig, corevalidation.ValidatePodDNSConfig)...)

	errs = append(errs, metav1validation.ValidateLabels(overlay.PodLabels, path.Child("podLabels"))...)
	errs = append(errs, corevalidation.ValidateAnnotations(overlay.PodAnnotations, path.Child("podAnnotations"))...)

	return errs
}

func validateMainContainerOverlay(overlay *v1alpha1.MainContainerOverlay, path *field.Path) field.ErrorList {
	errs := field.ErrorList{}
	// validate []EnvFrom
	errs = append(errs, validateSliceWithConvert(overlay.EnvFrom, path.Child("envFrom"),
		apiscorev1.Convert_v1_EnvFromSource_To_core_EnvFromSource, corevalidation.ValidateEnvFrom)...)
	// validate []Env
	errs = append(errs, validateSliceWithConvert(overlay.Env, path.Child("env"),
		apiscorev1.Convert_v1_EnvVar_To_core_EnvVar,
		func(vars []core.EnvVar, subPath *field.Path) field.ErrorList {
			return corevalidation.ValidateEnv(vars, subPath, corevalidation.PodValidationOptions{})
		})...)

	// validate ImagePullPolicy
	if overlay.ImagePullPolicy != nil {
		errs = append(errs, corevalidation.ValidatePullPolicy(core.PullPolicy(*overlay.ImagePullPolicy), path.Child("imagePullPolicy"))...)
	}
	// validate VolumeMounts
	errs = append(errs, validateSliceWithConvert(overlay.VolumeMounts, path.Child("volumeMounts"),
		apiscorev1.Convert_v1_VolumeMount_To_core_VolumeMount,
		func(mounts []core.VolumeMount, subPath *field.Path) field.ErrorList {
			// TODO: complete params with Container
			//return corevalidation.ValidateVolumeMounts(mounts, nil, nil, nil, path.Child("volumeMounts"))
			return nil
		})...)

	// validate LivenessProbe
	errs = append(errs, validateWithConvert(overlay.LivenessProbe, path.Child("livenessProbe"),
		apiscorev1.Convert_v1_Probe_To_core_Probe, corevalidation.ValidateProbe)...)
	// validate ReadinessProbe
	errs = append(errs, validateWithConvert(overlay.ReadinessProbe, path.Child("readinessProbe"),
		apiscorev1.Convert_v1_Probe_To_core_Probe, corevalidation.ValidateProbe)...)
	// validate StartupProbe
	errs = append(errs, validateWithConvert(overlay.StartupProbe, path.Child("startupProbe"),
		apiscorev1.Convert_v1_Probe_To_core_Probe, corevalidation.ValidateProbe)...)

	// validate Lifecycle
	errs = append(errs, validateWithConvert(overlay.Lifecycle, path.Child("lifecycle"),
		apiscorev1.Convert_v1_Lifecycle_To_core_Lifecycle, corevalidation.ValidateLifecycle)...)

	return errs
}

func validateSliceWithConvert[inT, outT convertor.Convertable](in []inT, path *field.Path, convertFn func(*inT, *outT, conversion.Scope) error, validateFn func([]outT, *field.Path) field.ErrorList) field.ErrorList {
	if len(in) == 0 {
		return nil
	}
	out, err := convertor.ConvertSlice(in, convertFn)
	if err != nil {
		return field.ErrorList{field.Invalid(path, in, err.Error())}
	}

	return validateFn(out, path)
}

func validateWithConvert[inT, outT convertor.Convertable](in *inT, path *field.Path, convertFn func(*inT, *outT, conversion.Scope) error, validateFn func(*outT, *field.Path) field.ErrorList) field.ErrorList {
	if in == nil {
		return nil
	}
	out, err := convertor.Convert(in, convertFn)
	if err != nil {
		return field.ErrorList{field.Invalid(path, in, err.Error())}
	}

	return validateFn(out, path)
}

func makeClaimNameSet(claims []core.ResourceClaim) sets.String {
	claimNames := sets.NewString()
	for _, claim := range claims {
		claimNames.Insert(claim.Name)
	}
	return claimNames
}

func validateVolumeClaims(pvcs []core.PersistentVolumeClaim, path *field.Path) field.ErrorList {
	var errs field.ErrorList
	for i, pvc := range pvcs {
		errs = append(errs, corevalidation.ValidatePersistentVolumeClaimSpec(&pvc.Spec, path.Index(i).Child("spec"), corevalidation.PersistentVolumeClaimSpecValidationOptions{})...)
	}
	return errs
}
