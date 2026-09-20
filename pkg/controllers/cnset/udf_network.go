// Copyright 2026 Matrix Origin
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
package cnset

import (
	"fmt"
	recon "github.com/matrixorigin/controller-runtime/pkg/reconciler"
	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	"github.com/matrixorigin/matrixone-operator/pkg/controllers/common"
	kruisev1alpha1 "github.com/openkruise/kruise-api/apps/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Fresh installs add neither grants nor isolation to the CN platform network.
// An old allowlist cannot simply be deleted: if it was the last selecting
// ingress policy that would make CN non-isolated. Migrate it to a zero-grant
// isolation anchor only after a platform baseline covers CN service protocols.
// Keep that anchor across toggles and platform policy deletion; finalization
// removes it only after the workload's Pods have gone.
func (c *Actor) syncUDFWorkerNetworkPolicy(ctx *recon.Context[*v1alpha1.CNSet]) error {
	legacy := &networkingv1.NetworkPolicy{}
	if err := ctx.Get(client.ObjectKey{Namespace: ctx.Obj.Namespace, Name: udfWorkerNetworkPolicyName(ctx.Obj)}, legacy); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}
	if !metav1.IsControlledBy(legacy, ctx.Obj) {
		return nil
	}
	if len(legacy.Spec.Ingress) == 0 {
		return nil
	}
	policies := &networkingv1.NetworkPolicyList{}
	if err := ctx.List(policies, client.InNamespace(ctx.Obj.Namespace)); err != nil {
		return err
	}
	pods := &corev1.PodList{}
	if err := ctx.List(pods, client.InNamespace(ctx.Obj.Namespace), client.MatchingLabels(common.SubResourceLabels(ctx.Obj))); err != nil {
		return err
	}
	// Require coverage of stable template labels too, not just a transient Pod
	// label that will disappear on the next CloneSet rollout.
	desired := &kruisev1alpha1.CloneSet{}
	desired.Spec.Template.Labels = common.SubResourceLabels(ctx.Obj)
	if err := syncPodMeta(ctx.Obj, desired); err != nil {
		return err
	}
	targets := []map[string]string{common.SubResourceLabels(ctx.Obj), desired.Spec.Template.Labels}
	for i := range pods.Items {
		targets = append(targets, pods.Items[i].Labels)
	}
	for _, target := range targets {
		if !platformCNIngressCovered(policies.Items, legacy.Name, target) {
			return fmt.Errorf("UDFNetworkPolicyMigrationRequired: install a platform ingress policy selecting stable CNSet labels and permitting CN TCP services plus Gossip UDP/6005 before retiring %s/%s; Python must not widen platform grants", legacy.Namespace, legacy.Name)
		}
	}
	return ctx.Patch(legacy, func() error { legacy.Spec.Ingress = nil; return nil })
}

type cnIngressPort struct {
	protocol corev1.Protocol
	port     int32
}

// This is a conservative migration coverage check, not an authorization engine.
// Sources remain exactly those chosen by the platform. Named ports cannot prove
// coverage here; supply numeric ports/ranges or an all-ports platform rule.
func platformCNIngressCovered(policies []networkingv1.NetworkPolicy, legacyName string, target map[string]string) bool {
	required := map[cnIngressPort]bool{}
	for _, port := range []int32{6001, 6002, 6003, 6004, 6005, 6006, 7001} {
		required[cnIngressPort{corev1.ProtocolTCP, port}] = false
	}
	required[cnIngressPort{corev1.ProtocolUDP, 6005}] = false
	for _, policy := range policies {
		if policy.Name == legacyName || policy.DeletionTimestamp != nil {
			continue
		}
		ingress := len(policy.Spec.PolicyTypes) == 0
		for _, kind := range policy.Spec.PolicyTypes {
			if kind == networkingv1.PolicyTypeIngress {
				ingress = true
			}
		}
		if !ingress {
			continue
		}
		// Negative constraints can match an incomplete label map but fail after
		// a rollout adds labels. Only positive stable constraints are supported.
		stable := true
		for _, expression := range policy.Spec.PodSelector.MatchExpressions {
			if expression.Operator != metav1.LabelSelectorOpIn && expression.Operator != metav1.LabelSelectorOpExists {
				stable = false
			}
		}
		if !stable {
			continue
		}
		selector, err := metav1.LabelSelectorAsSelector(&policy.Spec.PodSelector)
		if err != nil || !selector.Matches(labels.Set(target)) {
			continue
		}
		for _, rule := range policy.Spec.Ingress {
			for key := range required {
				if len(rule.Ports) == 0 {
					required[key] = true
					continue
				}
				for _, port := range rule.Ports {
					protocol := corev1.ProtocolTCP
					if port.Protocol != nil {
						protocol = *port.Protocol
					}
					if protocol != key.protocol {
						continue
					}
					if port.Port == nil {
						required[key] = true
						continue
					}
					if port.Port.Type != intstr.Int {
						continue
					}
					end := port.Port.IntVal
					if port.EndPort != nil {
						end = *port.EndPort
					}
					if port.Port.IntVal <= key.port && key.port <= end {
						required[key] = true
					}
				}
			}
		}
	}
	for _, covered := range required {
		if !covered {
			return false
		}
	}
	return true
}

func (c *Actor) hasLiveCNPods(ctx *recon.Context[*v1alpha1.CNSet]) (bool, error) {
	pods := &corev1.PodList{}
	if err := ctx.List(pods, client.InNamespace(ctx.Obj.Namespace), client.MatchingLabels(common.SubResourceLabels(ctx.Obj))); err != nil {
		return false, err
	}
	return len(pods.Items) != 0, nil
}
