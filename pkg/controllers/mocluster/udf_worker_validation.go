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

package mocluster

import (
	"fmt"

	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	"github.com/matrixorigin/matrixone-operator/pkg/controllers/common"
)

// validateUDFWorkerSources protects the precedence rule even when a restored
// MatrixOneCluster reaches reconcile without going through its webhook. A
// cluster policy is the only source for generated CN groups; a group-level
// policy or retired Python configuration must never be silently discarded.
func validateUDFWorkerSources(mo *v1alpha1.MatrixOneCluster, groups []v1alpha1.CNGroup) error {
	if mo.Spec.UDFWorker != nil {
		if err := mo.Spec.UDFWorker.Validate(); err != nil {
			return err
		}
	}
	seenNames := make(map[string]struct{}, len(groups))
	for _, group := range groups {
		if _, exists := seenNames[group.Name]; exists {
			return fmt.Errorf("DuplicateCNGroupName: %s", group.Name)
		}
		seenNames[group.Name] = struct{}{}
		if group.UDFWorker != nil {
			return fmt.Errorf("CNGroupUDFWorkerPolicyMustComeFromMatrixOneCluster: %s", group.Name)
		}
		if err := group.CNSetSpec.ValidateUDFWorkerConfiguration(); err != nil {
			return err
		}
		if mo.Spec.UDFWorker != nil && mo.Spec.UDFWorker.Enabled {
			if err := common.ValidateUDFWorkerPodOverlay(group.CNSetSpec.Overlay); err != nil {
				return fmt.Errorf("invalid effective UDF worker overlay for CNGroup %s: %w", group.Name, err)
			}
		}
	}
	return nil
}

// sanitizeGeneratedCNSetOverlay removes controller-owned metadata that is
// needed on the rendered Pod but is not part of the CNSet user's effective
// overlay. MatrixOneCluster adds the cluster selector to generic component
// overlays for historical reasons; an enabled CNSet validates overlays before
// applying them, so leaving that label in the copied overlay would make the
// generated object fail its own fail-closed validation. The CNSet renderer
// projects the label from CNSet metadata after applying the user overlay.
func sanitizeGeneratedCNSetOverlay(mo *v1alpha1.MatrixOneCluster, spec *v1alpha1.CNSetSpec) {
	if mo == nil || spec == nil || mo.Spec.UDFWorker == nil || !mo.Spec.UDFWorker.Enabled || spec.Overlay == nil {
		return
	}
	delete(spec.Overlay.PodLabels, common.MatrixoneClusterLabelKey)
}
