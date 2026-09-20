// Copyright 2025-2026 Matrix Origin
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0

package webhook

import (
	"strings"
	"testing"

	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func webhookTestPolicy() *v1alpha1.UDFWorkerPolicy {
	return &v1alpha1.UDFWorkerPolicy{
		Enabled:  true,
		Topology: v1alpha1.UDFWorkerTopologyPaired,
		Launcher: v1alpha1.UDFWorkerLauncherPythonImage,
		Worker: v1alpha1.UDFWorkerSpec{
			Image: "registry.example/udf-worker@sha256:" + strings.Repeat("b", 64),
			Resources: corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("1"),
					corev1.ResourceMemory: resource.MustParse("1Gi"),
				},
			},
		},
		Client: v1alpha1.UDFClientConfig{AllowUnisolated: true},
	}
}

func TestValidateUDFWorkerPolicy(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*v1alpha1.CNSetSpec)
		want   string
	}{
		{name: "legacy demo is rejected", mutate: func(s *v1alpha1.CNSetSpec) {
			s.PythonUdfSidecar = &v1alpha1.PythonUdfSidecar{}
		}, want: "LegacyPythonUdfSidecarUnsupported"},
		{name: "arbitrary client config is rejected", mutate: func(s *v1alpha1.CNSetSpec) {
			s.Config = v1alpha1.NewTomlConfig(map[string]interface{}{
				"cn": map[string]interface{}{"python-udf-client": map[string]interface{}{"enabled": true}},
			})
		}, want: "PythonClientConfigManagedByUDFWorkerPolicy"},
		{name: "sidecar overlay is rejected", mutate: func(s *v1alpha1.CNSetSpec) {
			s.Overlay = &v1alpha1.Overlay{SidecarContainers: []corev1.Container{{Name: "untrusted"}}}
		}, want: "PythonEnabledCNSetDisallowsOverlaySidecars"},
		{name: "shared pid is rejected", mutate: func(s *v1alpha1.CNSetSpec) {
			v := true
			s.Overlay = &v1alpha1.Overlay{ShareProcessNamespace: &v}
		}, want: "PythonEnabledCNSetDisallowsSharedProcessNamespace"},
		{name: "selector label overlay is rejected", mutate: func(s *v1alpha1.CNSetSpec) {
			s.Overlay = &v1alpha1.Overlay{PodLabels: map[string]string{"matrixorigin.io/component": "Other"}}
		}, want: "PythonEnabledCNSetDisallowsControllerOwnedPodLabel"},
		{name: "claim ownership label overlay is rejected", mutate: func(s *v1alpha1.CNSetSpec) {
			s.Overlay = &v1alpha1.Overlay{PodLabels: map[string]string{v1alpha1.ClaimSetNameLabel: "claimset"}}
		}, want: "PythonEnabledCNSetDisallowsControllerOwnedPodLabel"},
		{name: "kruise lifecycle label overlay is rejected", mutate: func(s *v1alpha1.CNSetSpec) {
			s.Overlay = &v1alpha1.Overlay{PodLabels: map[string]string{"lifecycle.apps.kruise.io/state": "Normal"}}
		}, want: "PythonEnabledCNSetDisallowsControllerOwnedPodLabel"},
		{name: "lifecycle annotation overlay is rejected", mutate: func(s *v1alpha1.CNSetSpec) {
			s.Overlay = &v1alpha1.Overlay{PodAnnotations: map[string]string{"matrixorigin.io/config-suffix": "old"}}
		}, want: "PythonEnabledCNSetDisallowsControllerOwnedPodAnnotation"},
		{name: "same pod mo service is rejected", mutate: func(s *v1alpha1.CNSetSpec) {
			s.UDFWorker.Launcher = v1alpha1.UDFWorkerLauncherMOService
		}, want: "UnsupportedSamePodMoServiceLauncher"},
		{name: "CN port collision is rejected", mutate: func(s *v1alpha1.CNSetSpec) {
			s.UDFWorker.Worker.Port = v1alpha1.CNUDFWorkerReservedMetricsPort
		}, want: "UDFWorkerPortConflictsWithCN"},
		{name: "missing CPU limit is rejected without panic", mutate: func(s *v1alpha1.CNSetSpec) {
			delete(s.UDFWorker.Worker.Resources.Limits, corev1.ResourceCPU)
		}, want: "UDFWorkerResourceLimitsRequired"},
		{name: "missing memory limit is rejected without panic", mutate: func(s *v1alpha1.CNSetSpec) {
			delete(s.UDFWorker.Worker.Resources.Limits, corev1.ResourceMemory)
		}, want: "UDFWorkerResourceLimitsRequired"},
		{name: "negative request is rejected", mutate: func(s *v1alpha1.CNSetSpec) {
			s.UDFWorker.Worker.Resources.Requests = corev1.ResourceList{
				corev1.ResourceCPU: *resource.NewQuantity(-1, resource.DecimalSI),
			}
		}, want: "UDFWorkerRequestsOutOfRange"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := v1alpha1.CNSetSpec{ConfigThatChangeCNSpec: v1alpha1.ConfigThatChangeCNSpec{UDFWorker: webhookTestPolicy()}}
			tt.mutate(&spec)
			errs := validateUDFWorkerPolicy(&spec, field.NewPath("spec"))
			if len(errs) == 0 || !strings.Contains(errs.ToAggregate().Error(), tt.want) {
				t.Fatalf("validation errors = %v, want %q", errs, tt.want)
			}
		})
	}
}

func TestDefaultUDFWorkerPolicy(t *testing.T) {
	p := webhookTestPolicy()
	defaultUDFWorkerPolicy(p)
	if p.Worker.Port != v1alpha1.ContainerUDFWorkerDefaultPort {
		t.Fatalf("port = %d, want %d", p.Worker.Port, v1alpha1.ContainerUDFWorkerDefaultPort)
	}
	if p.Worker.ImagePullPolicy != corev1.PullIfNotPresent {
		t.Fatalf("pull policy = %q, want %q", p.Worker.ImagePullPolicy, corev1.PullIfNotPresent)
	}
}

func TestUDFWorkerPolicyAllowsExplicitEmptySidecarList(t *testing.T) {
	spec := v1alpha1.CNSetSpec{
		ConfigThatChangeCNSpec: v1alpha1.ConfigThatChangeCNSpec{UDFWorker: webhookTestPolicy()},
		PodSet:                 v1alpha1.PodSet{Overlay: &v1alpha1.Overlay{SidecarContainers: []corev1.Container{}}},
	}
	if errs := validateUDFWorkerPolicy(&spec, field.NewPath("spec")); len(errs) != 0 {
		t.Fatalf("empty sidecar list should be structurally harmless, got %v", errs)
	}

	// A nil RuntimeClassName is the only current same-Pod default; a
	// user-provided class is a future security gate.
	if spec.Overlay.RuntimeClassName != nil {
		t.Fatalf("unexpected runtime class in test fixture: %v", spec.Overlay.RuntimeClassName)
	}
}
