// Copyright 2025 Matrix Origin
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

package corevalidation

import (
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/kubernetes/pkg/apis/core"
)

func ValidateTopologySpreadConstraints(constraints []core.TopologySpreadConstraint, fldPath *field.Path) field.ErrorList {
	return validateTopologySpreadConstraints(constraints, fldPath, PodValidationOptions{})
}

func ValidateAffinity(affinity *core.Affinity, fldPath *field.Path) field.ErrorList {
	return validateAffinity(affinity, PodValidationOptions{}, fldPath)
}

func ValidatePullPolicy(policy core.PullPolicy, fldPath *field.Path) field.ErrorList {
	return validatePullPolicy(policy, fldPath)
}

func ValidateProbe(probe *core.Probe, fldPath *field.Path) field.ErrorList {
	return validateProbe(probe, fldPath)
}

func ValidateLifecycle(lifecycle *core.Lifecycle, fldPath *field.Path) field.ErrorList {
	return validateLifecycle(lifecycle, fldPath)
}

func ValidateInitContainers(containers []core.Container, volumes map[string]core.VolumeSource, fldPath *field.Path) field.ErrorList {
	return validateInitContainers(containers, nil, volumes, nil, fldPath, PodValidationOptions{})
}

func ValidateImagePullSecrets(imagePullSecrets []core.LocalObjectReference, fldPath *field.Path) field.ErrorList {
	return validateImagePullSecrets(imagePullSecrets, fldPath)
}

func ValidatePodDNSConfig(dnsConfig *core.PodDNSConfig, fldPath *field.Path) field.ErrorList {
	return validatePodDNSConfig(dnsConfig, nil, fldPath, PodValidationOptions{})
}
