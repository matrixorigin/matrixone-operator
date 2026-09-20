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
	"sort"

	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// syncUDFWorkerClusterStatus aggregates only the bounded UDF status exposed by
// generated CNSets. It deliberately does not make the MatrixOneCluster ready:
// same-Pod capability readiness belongs to the Python route and must not remove
// an otherwise healthy CN from ordinary SQL service.
func syncUDFWorkerClusterStatus(
	mo *v1alpha1.MatrixOneCluster,
	groups []v1alpha1.CNGroup,
	current []v1alpha1.CNSet,
	desired map[string]*v1alpha1.CNSet,
) {
	policy := mo.Spec.UDFWorker
	status := &mo.Status.UDFWorker
	status.Topology = policy.EffectiveTopology()
	status.Generation = ""
	status.DesiredWorkers = 0
	status.ReadyWorkers = 0

	if policy == nil || !policy.Enabled {
		status.Conditions = retainUDFWorkerClusterConditions(status.Conditions, false)
		setClusterUDFWorkerCondition(mo, status, v1alpha1.UDFWorkerConditionEnabled, metav1.ConditionFalse,
			"Disabled", "Python UDF worker is disabled")
		setClusterUDFWorkerCondition(mo, status, v1alpha1.UDFWorkerConditionProvisioned, metav1.ConditionFalse,
			"Disabled", "no Python UDF worker is rendered")
		return
	}
	status.Conditions = retainUDFWorkerClusterConditions(status.Conditions, true)

	for _, group := range groups {
		status.DesiredWorkers += group.Replicas
	}

	sets := make(map[string]*v1alpha1.CNSet, len(current))
	// Generation describes the desired policy. Ready counts/conditions below
	// include only observations of the exact children returned by reconciliation.
	status.Generation = v1alpha1.UDFWorkerPolicyGeneration(policy)
	for i := range current {
		cn := &current[i]
		if !currentCNSetUDFWorkerStatus(cn, desired[cn.Name], status.Generation) {
			continue
		}
		sets[cn.Name] = cn
		status.ReadyWorkers += cn.Status.UDFWorker.ReadyWorkers
	}

	setClusterUDFWorkerCondition(mo, status, v1alpha1.UDFWorkerConditionEnabled, metav1.ConditionTrue,
		"PolicyAccepted", "the cluster Python UDF policy is enabled")

	provisioned, reason, message := allCNSetUDFWorkerCondition(sets, desired, v1alpha1.UDFWorkerConditionProvisioned)
	setClusterUDFWorkerCondition(mo, status, v1alpha1.UDFWorkerConditionProvisioned, conditionStatus(provisioned), reason, message)
	for _, typ := range []string{
		v1alpha1.UDFWorkerConditionDependencyReady,
		v1alpha1.UDFWorkerConditionCapacityReady,
		v1alpha1.UDFWorkerConditionClientConfigReady,
		v1alpha1.UDFWorkerConditionCapabilityReady,
		v1alpha1.UDFWorkerConditionRouteReady,
	} {
		ready, conditionReason, conditionMessage := allCNSetUDFWorkerCondition(sets, desired, typ)
		setClusterUDFWorkerCondition(mo, status, typ, conditionStatus(ready), conditionReason, conditionMessage)
	}

	degraded := false
	degradedReason := "AllCNSetUDFWorkerConditionsReady"
	degradedMessage := "all generated CNSets report a non-degraded Python UDF state"
	if len(sets) != len(sortedDesiredNames(desired)) || len(desired) == 0 {
		degraded = true
		degradedReason = "CNSetStatusIncomplete"
		degradedMessage = "one or more generated CNSets have not reported Python UDF status"
	} else {
		for _, name := range sortedDesiredNames(desired) {
			if condition := findUDFWorkerCondition(sets[name].Status.UDFWorker.Conditions, v1alpha1.UDFWorkerConditionDegraded); condition != nil && condition.Status == metav1.ConditionTrue {
				degraded = true
				degradedReason = condition.Reason
				degradedMessage = fmt.Sprintf("CNSet %s: %s", name, condition.Message)
				break
			}
		}
	}
	setClusterUDFWorkerCondition(mo, status, v1alpha1.UDFWorkerConditionDegraded, conditionStatus(degraded), degradedReason, degradedMessage)
	setClusterUDFWorkerCondition(mo, status, v1alpha1.UDFWorkerConditionDraining, metav1.ConditionFalse,
		"NoDrainRequested", "the Operator has not requested a Python UDF drain")
}

// A cache read can lag the successful child write. Fence the child identity and
// metadata generation as well as the policy hash; replicas/image changes need
// fresh observation even when the Python policy itself did not change.
func currentCNSetUDFWorkerStatus(cn, desired *v1alpha1.CNSet, generation string) bool {
	if desired == nil || cn.UID != desired.UID || cn.Generation != desired.Generation ||
		cn.DeletionTimestamp != nil || cn.Spec.Replicas != desired.Spec.Replicas ||
		v1alpha1.UDFWorkerPolicyGeneration(cn.Spec.UDFWorker) != generation ||
		v1alpha1.UDFWorkerPolicyGeneration(desired.Spec.UDFWorker) != generation ||
		cn.Status.UDFWorker.Generation != generation {
		return false
	}
	for _, typ := range []string{
		v1alpha1.UDFWorkerConditionProvisioned,
		v1alpha1.UDFWorkerConditionDependencyReady,
		v1alpha1.UDFWorkerConditionCapacityReady,
		v1alpha1.UDFWorkerConditionClientConfigReady,
		v1alpha1.UDFWorkerConditionCapabilityReady,
		v1alpha1.UDFWorkerConditionRouteReady,
		v1alpha1.UDFWorkerConditionDegraded,
	} {
		condition := findUDFWorkerCondition(cn.Status.UDFWorker.Conditions, typ)
		if condition == nil || condition.ObservedGeneration != cn.Generation || condition.Status == metav1.ConditionUnknown {
			return false
		}
	}
	return true
}

func retainUDFWorkerClusterConditions(conditions []metav1.Condition, enabled bool) []metav1.Condition {
	allowed := map[string]struct{}{
		v1alpha1.UDFWorkerConditionEnabled:     {},
		v1alpha1.UDFWorkerConditionProvisioned: {},
	}
	if enabled {
		for _, typ := range []string{
			v1alpha1.UDFWorkerConditionDependencyReady,
			v1alpha1.UDFWorkerConditionCapacityReady,
			v1alpha1.UDFWorkerConditionClientConfigReady,
			v1alpha1.UDFWorkerConditionCapabilityReady,
			v1alpha1.UDFWorkerConditionRouteReady,
			v1alpha1.UDFWorkerConditionDegraded,
			v1alpha1.UDFWorkerConditionDraining,
		} {
			allowed[typ] = struct{}{}
		}
	}
	kept := conditions[:0]
	for _, condition := range conditions {
		if _, ok := allowed[condition.Type]; ok {
			kept = append(kept, condition)
		}
	}
	return kept
}

func allCNSetUDFWorkerCondition(sets map[string]*v1alpha1.CNSet, desired map[string]*v1alpha1.CNSet, typ string) (bool, string, string) {
	if len(desired) == 0 {
		return false, "NoCNSet", "no generated CNSet is available for the enabled Python UDF policy"
	}
	for _, name := range sortedDesiredNames(desired) {
		cn, ok := sets[name]
		if !ok {
			return false, "CNSetStatusMissing", fmt.Sprintf("CNSet %s has not reported Python UDF status", name)
		}
		condition := findUDFWorkerCondition(cn.Status.UDFWorker.Conditions, typ)
		if condition == nil {
			return false, "CNSetConditionMissing", fmt.Sprintf("CNSet %s has no %s condition", name, typ)
		}
		if condition.Status != metav1.ConditionTrue {
			return false, condition.Reason, fmt.Sprintf("CNSet %s: %s", name, condition.Message)
		}
	}
	return true, "AllCNSetConditionsReady", "all generated CNSets report this condition ready"
}

func sortedDesiredNames(desired map[string]*v1alpha1.CNSet) []string {
	names := make([]string, 0, len(desired))
	for name, wanted := range desired {
		if wanted != nil {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

func conditionStatus(ready bool) metav1.ConditionStatus {
	if ready {
		return metav1.ConditionTrue
	}
	return metav1.ConditionFalse
}

func findUDFWorkerCondition(conditions []metav1.Condition, typ string) *metav1.Condition {
	for i := range conditions {
		if conditions[i].Type == typ {
			return &conditions[i]
		}
	}
	return nil
}

func setClusterUDFWorkerCondition(mo *v1alpha1.MatrixOneCluster, status *v1alpha1.UDFWorkerStatus, typ string, state metav1.ConditionStatus, reason, message string) {
	for i := range status.Conditions {
		if status.Conditions[i].Type != typ {
			continue
		}
		if status.Conditions[i].Status == state && status.Conditions[i].Reason == reason && status.Conditions[i].Message == message &&
			status.Conditions[i].ObservedGeneration == mo.Generation {
			return
		}
		status.Conditions[i] = metav1.Condition{
			Type:               typ,
			Status:             state,
			Reason:             reason,
			Message:            message,
			ObservedGeneration: mo.Generation,
			LastTransitionTime: metav1.Now(),
		}
		return
	}
	status.Conditions = append(status.Conditions, metav1.Condition{
		Type:               typ,
		Status:             state,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: mo.Generation,
		LastTransitionTime: metav1.Now(),
	})
}
