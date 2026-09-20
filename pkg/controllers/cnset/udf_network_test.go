// Copyright 2026 Matrix Origin
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
package cnset

import (
	reconfake "github.com/matrixorigin/controller-runtime/pkg/fake"
	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	"github.com/matrixorigin/matrixone-operator/pkg/controllers/common"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strings"
	"testing"
)

func platformIngressRules() []networkingv1.NetworkPolicyIngressRule {
	ports := []networkingv1.NetworkPolicyPort{}
	tcp, udp := corev1.ProtocolTCP, corev1.ProtocolUDP
	for _, number := range []int{6001, 6002, 6003, 6004, 6005, 6006, 7001} {
		p := intstr.FromInt(number)
		ports = append(ports, networkingv1.NetworkPolicyPort{Protocol: &tcp, Port: &p})
	}
	gossip := intstr.FromInt(6005)
	ports = append(ports, networkingv1.NetworkPolicyPort{Protocol: &udp, Port: &gossip})
	return []networkingv1.NetworkPolicyIngressRule{{From: []networkingv1.NetworkPolicyPeer{{PodSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"platform-trusted": "true"}}}}, Ports: ports}}
}
func TestLegacyUDFNetworkMigrationRequiresPlatformCoverage(t *testing.T) {
	for _, scenario := range []string{"no-platform", "missing-udp", "wrong-selector", "supported"} {
		t.Run(scenario, func(t *testing.T) {
			cn := &v1alpha1.CNSet{TypeMeta: metav1.TypeMeta{APIVersion: v1alpha1.GroupVersion.String(), Kind: "CNSet"}, ObjectMeta: metav1.ObjectMeta{Name: "cn", Namespace: "ns", UID: "cn-uid"}}
			legacy := &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: udfWorkerNetworkPolicyName(cn), Namespace: "ns", OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(cn, v1alpha1.GroupVersion.WithKind("CNSet"))}}, Spec: networkingv1.NetworkPolicySpec{PodSelector: metav1.LabelSelector{MatchLabels: common.SubResourceLabels(cn)}, PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress}, Ingress: platformIngressRules()}}
			cli := reconfake.KubeClientBuilder().WithScheme(newScheme()).WithObjects(legacy).Build()
			ctx := reconfake.NewContext(cn, cli, nil)
			platform := legacy.DeepCopy()
			platform.Name = "platform"
			platform.ResourceVersion = ""
			platform.UID = ""
			platform.OwnerReferences = nil
			if scenario == "missing-udp" {
				platform.Spec.Ingress[0].Ports = platform.Spec.Ingress[0].Ports[:7]
			}
			if scenario == "wrong-selector" {
				platform.Spec.PodSelector.MatchLabels = map[string]string{"transient-pod": "old"}
			}
			if scenario != "no-platform" {
				if err := cli.Create(ctx, platform); err != nil {
					t.Fatal(err)
				}
			}
			err := (&Actor{}).syncUDFWorkerNetworkPolicy(ctx)
			if scenario == "supported" {
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil || !strings.Contains(err.Error(), "UDFNetworkPolicyMigrationRequired") {
				t.Fatalf("migration not blocked: %v", err)
			}
			current := &networkingv1.NetworkPolicy{}
			if err := cli.Get(ctx, client.ObjectKeyFromObject(legacy), current); err != nil {
				t.Fatal("lost last isolation policy", err)
			}
			if scenario == "supported" {
				if len(current.Spec.Ingress) != 0 {
					t.Fatal("retained broad legacy grants")
				}
				if err := cli.Delete(ctx, platform); err != nil {
					t.Fatal(err)
				}
				if err := (&Actor{}).syncUDFWorkerNetworkPolicy(ctx); err != nil {
					t.Fatal(err)
				}
				if err := cli.Get(ctx, client.ObjectKeyFromObject(legacy), current); err != nil {
					t.Fatal("platform disappearance removed isolation anchor")
				}
			} else if len(current.Spec.Ingress) == 0 {
				t.Fatal("removed grants before platform coverage")
			}
		})
	}
}
func TestFreshPythonEnablementCreatesNoNetworkPolicy(t *testing.T) {
	cn := &v1alpha1.CNSet{ObjectMeta: metav1.ObjectMeta{Name: "cn", Namespace: "ns"}}
	cn.Spec.UDFWorker = cnSetUDFWorkerPolicyForTest()
	cli := reconfake.KubeClientBuilder().WithScheme(newScheme()).Build()
	ctx := reconfake.NewContext(cn, cli, nil)
	if err := (&Actor{}).syncUDFWorkerNetworkPolicy(ctx); err != nil {
		t.Fatal(err)
	}
	list := &networkingv1.NetworkPolicyList{}
	if err := cli.List(ctx, list); err != nil {
		t.Fatal(err)
	}
	if len(list.Items) != 0 {
		t.Fatal("Python changed CN isolation")
	}
}

func TestUDFNetworkCoverageRejectsNegativeRolloutSelector(t *testing.T) {
	p := networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "platform"}, Spec: networkingv1.NetworkPolicySpec{PodSelector: metav1.LabelSelector{MatchExpressions: []metav1.LabelSelectorRequirement{{Key: v1alpha1.UDFWorkerEnabledLabel, Operator: metav1.LabelSelectorOpDoesNotExist}}}, Ingress: platformIngressRules()}}
	if platformCNIngressCovered([]networkingv1.NetworkPolicy{p}, "legacy", map[string]string{}) {
		t.Fatal("accepted selector invalidated by enabled template")
	}
}
func TestFinalizeRetainsIsolationForCNOnlyPod(t *testing.T) {
	cn := &v1alpha1.CNSet{TypeMeta: metav1.TypeMeta{APIVersion: v1alpha1.GroupVersion.String(), Kind: "CNSet"}, ObjectMeta: metav1.ObjectMeta{Name: "cn", Namespace: "ns", UID: "cn-uid"}}
	anchor := &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: udfWorkerNetworkPolicyName(cn), Namespace: "ns", OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(cn, v1alpha1.GroupVersion.WithKind("CNSet"))}}, Spec: networkingv1.NetworkPolicySpec{PodSelector: metav1.LabelSelector{MatchLabels: common.SubResourceLabels(cn)}, PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress}}}
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "disabled-cn", Namespace: "ns", Labels: common.SubResourceLabels(cn)}, Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "mo"}}}}
	cli := reconfake.KubeClientBuilder().WithScheme(newScheme()).WithObjects(anchor, pod).Build()
	ctx := reconfake.NewContext(cn, cli, nil)
	done, err := (&Actor{}).Finalize(ctx)
	if err != nil || done {
		t.Fatalf("did not wait for CN-only pod: %v %v", done, err)
	}
	if err := cli.Get(ctx, client.ObjectKeyFromObject(anchor), &networkingv1.NetworkPolicy{}); err != nil {
		t.Fatal("isolation anchor prematurely deleted", err)
	}
	if err := cli.Delete(ctx, pod); err != nil {
		t.Fatal(err)
	}
	done, err = (&Actor{}).Finalize(ctx)
	if err != nil || !done {
		t.Fatalf("finalize after all Pods gone: %v %v", done, err)
	}
}
