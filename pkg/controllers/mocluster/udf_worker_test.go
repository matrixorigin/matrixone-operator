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
	"context"
	"reflect"
	"strings"
	"testing"

	reconfake "github.com/matrixorigin/controller-runtime/pkg/fake"
	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	"github.com/matrixorigin/matrixone-operator/pkg/controllers/common"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestSyncUDFWorkerClusterStatusAggregatesCNSetState(t *testing.T) {
	policy := &v1alpha1.UDFWorkerPolicy{Enabled: true, Topology: v1alpha1.UDFWorkerTopologyPaired}
	mo := &v1alpha1.MatrixOneCluster{Spec: v1alpha1.MatrixOneClusterSpec{UDFWorker: policy}}
	groups := []v1alpha1.CNGroup{
		{Name: "tp", CNSetSpec: v1alpha1.CNSetSpec{PodSet: v1alpha1.PodSet{Replicas: 2}}},
		{Name: "ap", CNSetSpec: v1alpha1.CNSetSpec{PodSet: v1alpha1.PodSet{Replicas: 1}}},
	}
	current := []v1alpha1.CNSet{
		{ObjectMeta: metav1.ObjectMeta{Name: "mo-tp"}, Status: v1alpha1.CNSetStatus{UDFWorker: readyUDFWorkerStatus(2, v1alpha1.UDFWorkerPolicyGeneration(policy))}},
		{ObjectMeta: metav1.ObjectMeta{Name: "mo-ap"}, Status: v1alpha1.CNSetStatus{UDFWorker: waitingUDFWorkerStatus(v1alpha1.UDFWorkerPolicyGeneration(policy))}},
	}
	desired := desiredUDFSetsForTest(current, policy)

	syncUDFWorkerClusterStatus(mo, groups, current, desired)
	if mo.Status.UDFWorker.DesiredWorkers != 3 || mo.Status.UDFWorker.ReadyWorkers != 2 {
		t.Fatalf("worker counts = %d/%d, want 3/2", mo.Status.UDFWorker.DesiredWorkers, mo.Status.UDFWorker.ReadyWorkers)
	}
	if mo.Status.UDFWorker.Generation != v1alpha1.UDFWorkerPolicyGeneration(policy) {
		t.Fatalf("generation = %q, want gen-a", mo.Status.UDFWorker.Generation)
	}
	capability := findUDFWorkerCondition(mo.Status.UDFWorker.Conditions, v1alpha1.UDFWorkerConditionCapabilityReady)
	if capability == nil || capability.Status != metav1.ConditionFalse || capability.Reason != "RuntimeStatusBridgeUnavailable" {
		t.Fatalf("capability condition = %+v, want bridge unavailable", capability)
	}
	degraded := findUDFWorkerCondition(mo.Status.UDFWorker.Conditions, v1alpha1.UDFWorkerConditionDegraded)
	if degraded == nil || degraded.Status != metav1.ConditionTrue || !strings.Contains(degraded.Message, "mo-ap") {
		t.Fatalf("degraded condition = %+v, want mo-ap detail", degraded)
	}
}

func TestValidateUDFWorkerSourcesRejectsDuplicateCNGroupNames(t *testing.T) {
	mo := &v1alpha1.MatrixOneCluster{}
	groups := []v1alpha1.CNGroup{
		{Name: "tp"},
		{Name: "tp"},
	}
	if err := validateUDFWorkerSources(mo, groups); err == nil || !strings.Contains(err.Error(), "DuplicateCNGroupName") {
		t.Fatalf("duplicate group names were accepted: %v", err)
	}
}

func TestSyncUDFWorkerClusterStatusDisabled(t *testing.T) {
	mo := &v1alpha1.MatrixOneCluster{}
	syncUDFWorkerClusterStatus(mo, nil, nil, nil)
	if mo.Status.UDFWorker.Topology != v1alpha1.UDFWorkerTopologyDisabled {
		t.Fatalf("topology = %q, want disabled", mo.Status.UDFWorker.Topology)
	}
	condition := findUDFWorkerCondition(mo.Status.UDFWorker.Conditions, v1alpha1.UDFWorkerConditionEnabled)
	if condition == nil || condition.Status != metav1.ConditionFalse {
		t.Fatalf("enabled condition = %+v, want false", condition)
	}
}

func TestSyncUDFWorkerClusterStatusIsStableAcrossReconcile(t *testing.T) {
	policy := &v1alpha1.UDFWorkerPolicy{Enabled: true, Topology: v1alpha1.UDFWorkerTopologyPaired}
	mo := &v1alpha1.MatrixOneCluster{Spec: v1alpha1.MatrixOneClusterSpec{UDFWorker: policy}}
	groups := []v1alpha1.CNGroup{{Name: "tp", CNSetSpec: v1alpha1.CNSetSpec{PodSet: v1alpha1.PodSet{Replicas: 1}}}}
	current := []v1alpha1.CNSet{{ObjectMeta: metav1.ObjectMeta{Name: "mo-tp"}, Status: v1alpha1.CNSetStatus{UDFWorker: readyUDFWorkerStatus(1, v1alpha1.UDFWorkerPolicyGeneration(policy))}}}
	desired := desiredUDFSetsForTest(current, policy)

	syncUDFWorkerClusterStatus(mo, groups, current, desired)
	want := mo.Status.UDFWorker.DeepCopy()
	syncUDFWorkerClusterStatus(mo, groups, current, desired)
	if !reflect.DeepEqual(&mo.Status.UDFWorker, want) {
		t.Fatalf("status changed during an identical reconcile: before=%#v after=%#v", want, mo.Status.UDFWorker)
	}
}

func TestSanitizeGeneratedCNSetOverlayRemovesClusterLabel(t *testing.T) {
	policy := &v1alpha1.UDFWorkerPolicy{Enabled: true, Topology: v1alpha1.UDFWorkerTopologyPaired}
	mo := &v1alpha1.MatrixOneCluster{Spec: v1alpha1.MatrixOneClusterSpec{UDFWorker: policy}}
	spec := &v1alpha1.CNSetSpec{PodSet: v1alpha1.PodSet{Overlay: &v1alpha1.Overlay{
		PodLabels: map[string]string{common.MatrixoneClusterLabelKey: "cluster-a", "example.com/label": "kept"},
	}}}
	sanitizeGeneratedCNSetOverlay(mo, spec)
	if _, ok := spec.Overlay.PodLabels[common.MatrixoneClusterLabelKey]; ok {
		t.Fatal("generated CNSet overlay must not carry the controller-owned cluster label")
	}
	if got := spec.Overlay.PodLabels["example.com/label"]; got != "kept" {
		t.Fatalf("user label = %q, want kept", got)
	}

	disabled := &v1alpha1.MatrixOneCluster{}
	disabledSpec := &v1alpha1.CNSetSpec{PodSet: v1alpha1.PodSet{Overlay: &v1alpha1.Overlay{
		PodLabels: map[string]string{common.MatrixoneClusterLabelKey: "kept-when-disabled"},
	}}}
	sanitizeGeneratedCNSetOverlay(disabled, disabledSpec)
	if got := disabledSpec.Overlay.PodLabels[common.MatrixoneClusterLabelKey]; got != "kept-when-disabled" {
		t.Fatalf("disabled policy changed overlay label to %q", got)
	}
}

func TestMatrixOneClusterPropagatesUDFPolicyWithoutOverlayClusterLabel(t *testing.T) {
	policy := &v1alpha1.UDFWorkerPolicy{
		Enabled:  true,
		Topology: v1alpha1.UDFWorkerTopologyPaired,
		Launcher: v1alpha1.UDFWorkerLauncherPythonImage,
		Worker: v1alpha1.UDFWorkerSpec{
			Image: "registry.example/udf-worker@sha256:" + strings.Repeat("a", 64),
			Resources: corev1.ResourceRequirements{Limits: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("1"),
				corev1.ResourceMemory: resource.MustParse("1Gi"),
			}},
		},
		Client: v1alpha1.UDFClientConfig{AllowUnisolated: true},
	}
	mo := &v1alpha1.MatrixOneCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster", Namespace: "ns"},
		Spec: v1alpha1.MatrixOneClusterSpec{
			UDFWorker: policy,
			TP:        &v1alpha1.CNSetSpec{PodSet: v1alpha1.PodSet{Replicas: 1}},
			TN:        &v1alpha1.DNSetSpec{PodSet: v1alpha1.PodSet{Replicas: 1}},
			LogService: v1alpha1.LogSetSpec{
				SharedStorage: v1alpha1.SharedStorageProvider{FileSystem: &v1alpha1.FileSystemProvider{Path: "/shared"}},
			},
			Version: "test",
		},
	}
	cli := reconfake.KubeClientBuilder().WithScheme(newScheme()).WithObjects(mo).WithStatusSubresource(mo).Build()
	ctx := reconfake.NewContext(mo, cli, nil)
	_, _ = (&MatrixOneClusterActor{}).Up(ctx)

	cn := &v1alpha1.CNSet{}
	if err := cli.Get(context.Background(), types.NamespacedName{Namespace: mo.Namespace, Name: "cluster-tp"}, cn); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(cn.Spec.UDFWorker, policy) {
		t.Fatalf("generated CNSet policy = %#v, want %#v", cn.Spec.UDFWorker, policy)
	}
	if cn.Spec.Overlay == nil {
		t.Fatal("generated CNSet overlay is nil")
	}
	if _, ok := cn.Spec.Overlay.PodLabels[common.MatrixoneClusterLabelKey]; ok {
		t.Fatal("generated enabled CNSet overlay must not contain the cluster controller label")
	}
	if got := cn.Labels[common.MatrixoneClusterLabelKey]; got != mo.Name {
		t.Fatalf("generated CNSet cluster label = %q, want %q", got, mo.Name)
	}
}

func readyUDFWorkerStatus(ready int32, generation string) v1alpha1.UDFWorkerStatus {
	return v1alpha1.UDFWorkerStatus{
		Generation:   generation,
		ReadyWorkers: ready,
		Conditions:   completeUDFWorkerConditions(true),
	}
}

func waitingUDFWorkerStatus(generation string) v1alpha1.UDFWorkerStatus {
	conditions := completeUDFWorkerConditions(false)
	return v1alpha1.UDFWorkerStatus{Generation: generation, ReadyWorkers: 0, Conditions: conditions}
}

func completeUDFWorkerConditions(capabilityReady bool) []metav1.Condition {
	status := metav1.ConditionTrue
	reason := "Ready"
	message := "ready"
	if !capabilityReady {
		status = metav1.ConditionFalse
		reason = "RuntimeStatusBridgeUnavailable"
		message = "bridge unavailable"
	}
	return []metav1.Condition{
		{Type: v1alpha1.UDFWorkerConditionProvisioned, Status: metav1.ConditionTrue, Reason: reason, Message: message},
		{Type: v1alpha1.UDFWorkerConditionDependencyReady, Status: metav1.ConditionTrue, Reason: "Ready", Message: "ready"},
		{Type: v1alpha1.UDFWorkerConditionCapacityReady, Status: metav1.ConditionTrue, Reason: "Ready", Message: "ready"},
		{Type: v1alpha1.UDFWorkerConditionClientConfigReady, Status: metav1.ConditionTrue, Reason: "Ready", Message: "ready"},
		{Type: v1alpha1.UDFWorkerConditionCapabilityReady, Status: status, Reason: reason, Message: message},
		{Type: v1alpha1.UDFWorkerConditionRouteReady, Status: status, Reason: reason, Message: message},
		{Type: v1alpha1.UDFWorkerConditionDegraded, Status: func() metav1.ConditionStatus {
			if capabilityReady {
				return metav1.ConditionFalse
			}
			return metav1.ConditionTrue
		}(), Reason: reason, Message: message},
	}
}

func desiredUDFSetsForTest(current []v1alpha1.CNSet, policy *v1alpha1.UDFWorkerPolicy) map[string]*v1alpha1.CNSet {
	desired := make(map[string]*v1alpha1.CNSet, len(current))
	for i := range current {
		current[i].Spec.UDFWorker = policy.DeepCopy()
		desired[current[i].Name] = current[i].DeepCopy()
	}
	return desired
}

func TestClusterUDFStatusRejectsStaleChildObservations(t *testing.T) {
	policy := &v1alpha1.UDFWorkerPolicy{Enabled: true, Topology: v1alpha1.UDFWorkerTopologyPaired}
	generation := v1alpha1.UDFWorkerPolicyGeneration(policy)
	for _, tc := range []struct {
		name   string
		mutate func(*v1alpha1.CNSet)
	}{
		{"old status policy", func(cn *v1alpha1.CNSet) { cn.Status.UDFWorker.Generation = "old" }},
		{"cached old policy", func(cn *v1alpha1.CNSet) {
			cn.Spec.UDFWorker.Worker.Port = 50052
			cn.Status.UDFWorker.Generation = v1alpha1.UDFWorkerPolicyGeneration(cn.Spec.UDFWorker)
		}},
		{"unobserved generation", func(cn *v1alpha1.CNSet) {
			for i := range cn.Status.UDFWorker.Conditions {
				cn.Status.UDFWorker.Conditions[i].ObservedGeneration--
			}
		}},
		{"cached old child", func(cn *v1alpha1.CNSet) {
			cn.Generation--
			for i := range cn.Status.UDFWorker.Conditions {
				cn.Status.UDFWorker.Conditions[i].ObservedGeneration = cn.Generation
			}
		}},
		{"same policy replica change", func(cn *v1alpha1.CNSet) { cn.Spec.Replicas = 2 }},
		{"replaced child UID", func(cn *v1alpha1.CNSet) { cn.UID = "previous" }},
		{"missing degraded", func(cn *v1alpha1.CNSet) { cn.Status.UDFWorker.Conditions = cn.Status.UDFWorker.Conditions[:6] }},
		{"unknown degraded", func(cn *v1alpha1.CNSet) { cn.Status.UDFWorker.Conditions[6].Status = metav1.ConditionUnknown }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mo := &v1alpha1.MatrixOneCluster{ObjectMeta: metav1.ObjectMeta{Generation: 17}, Spec: v1alpha1.MatrixOneClusterSpec{UDFWorker: policy}}
			groups := []v1alpha1.CNGroup{{CNSetSpec: v1alpha1.CNSetSpec{PodSet: v1alpha1.PodSet{Replicas: 2}}}}
			current := []v1alpha1.CNSet{}
			for _, name := range []string{"mo-a", "mo-b"} {
				cn := v1alpha1.CNSet{ObjectMeta: metav1.ObjectMeta{Name: name, UID: types.UID(name), Generation: 3}}
				cn.Spec.Replicas = 1
				cn.Status.UDFWorker = readyUDFWorkerStatus(1, generation)
				for i := range cn.Status.UDFWorker.Conditions {
					cn.Status.UDFWorker.Conditions[i].ObservedGeneration = cn.Generation
				}
				current = append(current, cn)
			}
			desired := desiredUDFSetsForTest(current, policy)
			tc.mutate(&current[0])
			syncUDFWorkerClusterStatus(mo, groups, current, desired)
			if mo.Status.UDFWorker.ReadyWorkers != 1 || mo.Status.UDFWorker.Generation != generation {
				t.Fatalf("stale child contributed readiness: %+v", mo.Status.UDFWorker)
			}
			for _, typ := range []string{v1alpha1.UDFWorkerConditionCapabilityReady, v1alpha1.UDFWorkerConditionRouteReady, v1alpha1.UDFWorkerConditionDegraded} {
				c := findUDFWorkerCondition(mo.Status.UDFWorker.Conditions, typ)
				want := metav1.ConditionFalse
				if typ == v1alpha1.UDFWorkerConditionDegraded {
					want = metav1.ConditionTrue
				}
				if c == nil || c.Status != want || c.ObservedGeneration != mo.Generation {
					t.Fatalf("stale %s: %+v", typ, c)
				}
			}
			current[0] = *desired["mo-a"].DeepCopy()
			syncUDFWorkerClusterStatus(mo, groups, current, desired)
			if mo.Status.UDFWorker.ReadyWorkers != 2 {
				t.Fatalf("recovery count: %+v", mo.Status.UDFWorker)
			}
			if c := findUDFWorkerCondition(mo.Status.UDFWorker.Conditions, v1alpha1.UDFWorkerConditionRouteReady); c.Status != metav1.ConditionTrue {
				t.Fatalf("recovery: %+v", c)
			}
			if c := findUDFWorkerCondition(mo.Status.UDFWorker.Conditions, v1alpha1.UDFWorkerConditionDegraded); c.Status != metav1.ConditionFalse {
				t.Fatalf("recovery: %+v", c)
			}
		})
	}
}
