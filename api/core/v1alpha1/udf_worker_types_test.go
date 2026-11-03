// Copyright 2025-2026 Matrix Origin
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0

package v1alpha1

import (
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func validUDFWorkerPolicyForTest() *UDFWorkerPolicy {
	return &UDFWorkerPolicy{
		Enabled:  true,
		Topology: UDFWorkerTopologyPaired,
		Launcher: UDFWorkerLauncherPythonImage,
		Worker: UDFWorkerSpec{
			Image: "registry.example/udf-worker@sha256:" + strings.Repeat("a", 64),
			Port:  50051,
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("200m"),
					corev1.ResourceMemory: resource.MustParse("512Mi"),
				},
				Limits: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("1"),
					corev1.ResourceMemory: resource.MustParse("1Gi"),
				},
			},
		},
		Client: UDFClientConfig{AllowUnisolated: true},
	}
}

func TestUDFWorkerPolicyValidate(t *testing.T) {
	tests := []struct {
		name string
		edit func(*UDFWorkerPolicy)
		want string
	}{
		{name: "valid", edit: func(_ *UDFWorkerPolicy) {}, want: ""},
		{name: "pool is not silently treated as paired", edit: func(p *UDFWorkerPolicy) {
			p.Topology = UDFWorkerTopologyPool
		}, want: "UnsupportedUDFWorkerTopology"},
		{name: "mo service cannot become a sidecar", edit: func(p *UDFWorkerPolicy) {
			p.Launcher = UDFWorkerLauncherMOService
		}, want: "UnsupportedSamePodMoServiceLauncher"},
		{name: "mutable image rejected", edit: func(p *UDFWorkerPolicy) {
			p.Worker.Image = "registry.example/udf-worker:latest"
		}, want: "UDFWorkerImageMustUseDigest"},
		{name: "image whitespace rejected", edit: func(p *UDFWorkerPolicy) {
			p.Worker.Image = "registry.example/udf-worker @sha256:" + strings.Repeat("a", 64)
		}, want: "UDFWorkerImageMustUseDigest"},
		{name: "unisolated gate required", edit: func(p *UDFWorkerPolicy) {
			p.Client.AllowUnisolated = false
		}, want: "UDFWorkerRequiresAllowUnisolated"},
		{name: "limits required", edit: func(p *UDFWorkerPolicy) {
			p.Worker.Resources.Limits = nil
		}, want: "UDFWorkerResourceLimitsRequired"},
		{name: "infinite limits rejected", edit: func(p *UDFWorkerPolicy) {
			q, err := resource.ParseQuantity("1e1000")
			if err != nil {
				panic(err)
			}
			p.Worker.Resources.Limits[corev1.ResourceMemory] = q
		}, want: "UDFWorkerResourceLimitsRequired"},
		{name: "negative requests rejected", edit: func(p *UDFWorkerPolicy) {
			p.Worker.Resources.Requests[corev1.ResourceCPU] = *resource.NewQuantity(-1, resource.DecimalSI)
		}, want: "UDFWorkerRequestsOutOfRange"},
		{name: "underflowed limit rejected", edit: func(p *UDFWorkerPolicy) {
			q, err := resource.ParseQuantity("1e-100000")
			if err != nil {
				panic(err)
			}
			p.Worker.Resources.Limits[corev1.ResourceMemory] = q
		}, want: "UDFWorkerLimitBelowRequest"},
		{name: "paired policy cannot carry pool settings", edit: func(p *UDFWorkerPolicy) {
			p.Pool = &UDFWorkerPoolSpec{Replicas: 1}
		}, want: "UDFWorkerPoolConfigNotAllowedForPaired"},
		{name: "same pod cannot carry mo service settings", edit: func(p *UDFWorkerPolicy) {
			p.Worker.MOService = &UDFWorkerMOServiceSpec{Path: "/mo-service"}
		}, want: "UnsupportedSamePodMoServiceConfig"},
		{name: "worker port cannot collide with CN SQL", edit: func(p *UDFWorkerPolicy) {
			p.Worker.Port = CNUDFWorkerReservedSQLPort
		}, want: "UDFWorkerPortConflictsWithCN"},
		{name: "worker port cannot collide with CN internal service", edit: func(p *UDFWorkerPolicy) {
			p.Worker.Port = CNUDFWorkerReservedPortBase + CNUDFWorkerReservedPortSlots - 1
		}, want: "UDFWorkerPortConflictsWithCN"},
		{name: "worker port cannot collide with CN metrics", edit: func(p *UDFWorkerPolicy) {
			p.Worker.Port = CNUDFWorkerReservedMetricsPort
		}, want: "UDFWorkerPortConflictsWithCN"},
		{name: "disabled policy still validates client bounds", edit: func(p *UDFWorkerPolicy) {
			p.Enabled = false
			p.Worker.Image = "old-demo:latest"
			p.Client.MaxBatchBytes = -1
		}, want: "UDFClientMaxBatchBytesOutOfRange"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy := validUDFWorkerPolicyForTest()
			tt.edit(policy)
			err := policy.Validate()
			if tt.want == "" {
				if err != nil {
					t.Fatalf("Validate() error = %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Validate() error = %v, want substring %q", err, tt.want)
			}
		})
	}
}

func TestIsCNPodPortReservedForUDFWorker(t *testing.T) {
	for _, tc := range []struct {
		port     int32
		reserved bool
	}{
		{port: CNUDFWorkerReservedSQLPort, reserved: true},
		{port: CNUDFWorkerReservedPortBase, reserved: true},
		{port: CNUDFWorkerReservedPortBase + CNUDFWorkerReservedPortSlots - 1, reserved: true},
		{port: CNUDFWorkerReservedSQLPort - 1, reserved: false},
		{port: CNUDFWorkerReservedPortBase + CNUDFWorkerReservedPortSlots, reserved: false},
		{port: CNUDFWorkerReservedMetricsPort, reserved: true},
		{port: 50051, reserved: false},
	} {
		if got := IsCNPodPortReservedForUDFWorker(tc.port); got != tc.reserved {
			t.Fatalf("IsCNPodPortReservedForUDFWorker(%d) = %v, want %v", tc.port, got, tc.reserved)
		}
	}
}

func TestUDFWorkerPolicyGenerationCanonicalizesEffectiveDefaults(t *testing.T) {
	base := validUDFWorkerPolicyForTest()
	base.Worker.Port = 0
	base.Worker.ImagePullPolicy = ""
	withDefaults := base.DeepCopy()
	withDefaults.Worker.Port = ContainerUDFWorkerDefaultPort
	withDefaults.Worker.ImagePullPolicy = corev1.PullIfNotPresent

	if got, want := UDFWorkerPolicyGeneration(base), UDFWorkerPolicyGeneration(withDefaults); got != want {
		t.Fatalf("effective-default policies have different generations: %q != %q", got, want)
	}

	zeroDuration := base.DeepCopy()
	zeroDuration.Client.RequestTimeout = &metav1.Duration{}
	zeroDuration.Client.TerminalRecordTTL = &metav1.Duration{}
	if got, want := UDFWorkerPolicyGeneration(base), UDFWorkerPolicyGeneration(zeroDuration); got != want {
		t.Fatalf("zero-duration defaults have different generations: %q != %q", got, want)
	}
}

func TestValidateUDFWorkerPodStatus(t *testing.T) {
	valid := &UDFWorkerPodStatus{
		PodUID:     "pod-uid",
		CNUUID:     "cn-uuid",
		Generation: "generation",
		Ready:      true,
		LeaseEpoch: 1,
	}
	if err := ValidateUDFWorkerPodStatus(valid); err != nil {
		t.Fatalf("valid status rejected: %v", err)
	}

	tests := []struct {
		name string
		edit func(*UDFWorkerPodStatus)
	}{
		{
			name: "unknown error class",
			edit: func(status *UDFWorkerPodStatus) { status.ErrorClass = "FutureStatus" },
		},
		{
			name: "oversized reason",
			edit: func(status *UDFWorkerPodStatus) { status.Reason = strings.Repeat("r", UDFWorkerStatusMaxStringBytes+1) },
		},
		{
			name: "oversized list",
			edit: func(status *UDFWorkerPodStatus) { status.Modes = make([]string, UDFWorkerStatusMaxListItems+1) },
		},
		{
			name: "oversized list entry",
			edit: func(status *UDFWorkerPodStatus) {
				status.Modes = []string{strings.Repeat("m", UDFWorkerStatusMaxListItemBytes+1)}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status := valid.DeepCopy()
			tt.edit(status)
			if err := ValidateUDFWorkerPodStatus(status); err == nil {
				t.Fatal("invalid status was accepted")
			}
		})
	}
}

func TestIsImmutableImageDigest(t *testing.T) {
	valid := "registry.example/udf-worker@sha256:" + strings.Repeat("a", 64)
	for name, image := range map[string]string{
		"valid":               valid,
		"tag":                 "registry.example/udf-worker:latest",
		"short digest":        "registry.example/udf-worker@sha256:" + strings.Repeat("a", 63),
		"non hex digest":      "registry.example/udf-worker@sha256:" + strings.Repeat("g", 64),
		"empty repository":    "@sha256:" + strings.Repeat("a", 64),
		"trailing characters": valid + "x",
		"leading whitespace":  " " + valid,
		"trailing whitespace": valid + " ",
	} {
		t.Run(name, func(t *testing.T) {
			if got := IsImmutableImageDigest(image); got != (name == "valid") {
				t.Fatalf("IsImmutableImageDigest(%q) = %v", image, got)
			}
		})
	}
}
