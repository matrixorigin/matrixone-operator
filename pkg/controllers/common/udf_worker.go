// Copyright 2025-2026 Matrix Origin
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

package common

import (
	"fmt"

	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	"github.com/openkruise/kruise-api/apps/pub"
	kruisev1alpha1 "github.com/openkruise/kruise-api/apps/v1alpha1"
)

// ValidateUDFWorkerPodOverlay applies the controller-side structural allowlist
// for a Python-enabled same-Pod CN. Admission returns field-specific errors,
// while controllers use this error-only form on generated, restored, and
// cross-controller objects before they mutate children.
func ValidateUDFWorkerPodOverlay(overlay *v1alpha1.Overlay) error {
	if overlay == nil {
		return nil
	}
	for key := range overlay.PodLabels {
		if IsUDFWorkerControllerOwnedPodLabel(key) {
			return fmt.Errorf("PythonEnabledCNSetDisallowsControllerOwnedPodLabel: %s", key)
		}
	}
	for key := range overlay.PodAnnotations {
		if IsUDFWorkerControllerOwnedPodAnnotation(key) {
			return fmt.Errorf("PythonEnabledCNSetDisallowsControllerOwnedPodAnnotation: %s", key)
		}
	}
	switch {
	case len(overlay.SidecarContainers) > 0:
		return fmt.Errorf("PythonEnabledCNSetDisallowsOverlaySidecars")
	case overlay.ShareProcessNamespace != nil && *overlay.ShareProcessNamespace:
		return fmt.Errorf("PythonEnabledCNSetDisallowsSharedProcessNamespace")
	case len(overlay.Volumes) > 0:
		return fmt.Errorf("PythonEnabledCNSetDisallowsOverlayVolumes")
	case overlay.InitContainers != nil:
		return fmt.Errorf("PythonEnabledCNSetDisallowsOverlayInitContainers")
	case overlay.SecurityContext != nil:
		return fmt.Errorf("PythonEnabledCNSetDisallowsOverlaySecurityContext")
	case overlay.ServiceAccountName != "":
		return fmt.Errorf("PythonEnabledCNSetDisallowsOverlayServiceAccount")
	case overlay.RuntimeClassName != nil:
		return fmt.Errorf("PythonEnabledCNSetDisallowsUnapprovedRuntimeClass")
	default:
		return nil
	}
}

// IsUDFWorkerControllerOwnedPodLabel identifies Pod labels that are used for
// CN selection, Python worker fencing, or CNPool/CNClaim ownership. An
// enabled UDF policy cannot allow an overlay to forge any of them.
func IsUDFWorkerControllerOwnedPodLabel(key string) bool {
	switch key {
	case NamespaceLabelKey,
		InstanceLabelKey,
		ComponentLabelKey,
		MatrixoneClusterLabelKey,
		CNUUIDLabelKey,
		v1alpha1.UDFWorkerEnabledLabel,
		v1alpha1.CNPodPhaseLabel,
		v1alpha1.PodClaimedByLabel,
		v1alpha1.ClaimSetNameLabel,
		v1alpha1.PodOwnerNameLabel,
		v1alpha1.PodLastOwnerLabel,
		v1alpha1.PodOutdatedLabel,
		v1alpha1.PoolNameLabel,
		v1alpha1.DirectPodLabel:
		return true
	case pub.LifecycleStateKey,
		kruisev1alpha1.SpecifiedDeleteKey:
		return true
	default:
		return false
	}
}

// IsUDFWorkerControllerOwnedPodAnnotation identifies metadata that is written
// by the CN, pool, claim, or Operator controllers. User overlay annotations
// remain available for ordinary metadata, but an enabled Python Pod cannot
// rewrite lifecycle, generation, readiness, or reclaim state.
func IsUDFWorkerControllerOwnedPodAnnotation(key string) bool {
	switch key {
	case CNLabelAnnotation,
		ConfigSuffixAnno,
		v1alpha1.UDFWorkerStatusAnno,
		v1alpha1.UDFWorkerGenerationAnno,
		v1alpha1.OperatorVersionAnno,
		SemanticVersionAnno,
		PrometheusScrapeAnno,
		PrometheusPortAnno,
		PrometheusPathAnno,
		CNStateAnno,
		ReclaimedAt,
		v1alpha1.StoreDrainingStartAnno,
		v1alpha1.StoreConnectionAnno,
		v1alpha1.StoreScoreAnno,
		v1alpha1.StoreCordonAnno,
		v1alpha1.PodManagementPolicyAnno,
		v1alpha1.DeleteOnReclaimAnno,
		v1alpha1.InPlacePoolRollingAnnoKey:
		return true
	default:
		return false
	}
}
