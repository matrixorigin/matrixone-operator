// Copyright 2025-2026 Matrix Origin
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0

package webhook

import (
	"context"
	"fmt"
	reconfake "github.com/matrixorigin/controller-runtime/pkg/fake"
	"github.com/matrixorigin/matrixone-operator/pkg/controllers/cnpool"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strings"
	"testing"

	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func TestCNPoolWebhookValidatesNestedUDFWorkerPolicy(t *testing.T) {
	p := &v1alpha1.CNPool{}
	p.Spec.Template.UDFWorker = webhookTestPolicy()
	p.Spec.Template.MainContainer.Image = "matrixone:1.2.0"
	p.Spec.Template.SemanticVersion = ptrString("1.2.0")
	p.Spec.Deps.ExternalLogSet = &v1alpha1.ExternalLogSet{HAKeeperEndpoint: "log:32001"}
	p.Spec.Template.Resources = corev1.ResourceRequirements{Limits: corev1.ResourceList{
		corev1.ResourceCPU: resource.MustParse("1"), corev1.ResourceMemory: resource.MustParse("1Gi"),
	}}
	// Remove the unrelated required PodSet failure so the assertion below
	// specifically exercises the nested policy path.
	if errs := (&cnPoolValidator{}).validate(p); len(errs) != 0 {
		t.Fatalf("valid nested policy rejected: %v", errs)
	}

	p.Spec.Template.UDFWorker.Launcher = v1alpha1.UDFWorkerLauncherMOService
	errs := (&cnPoolValidator{}).validate(p)
	if len(errs) == 0 || !strings.Contains(errs.ToAggregate().Error(), "UnsupportedSamePodMoServiceLauncher") {
		t.Fatalf("nested policy errors = %v", errs)
	}
}

func ptrString(v string) *string { return &v }

// Exercise the real pool producer through the actual CNSet defaulter/validator,
// without starting another API server for this deterministic revision transition.
func TestCNPoolRevisionMigrationPassesCNSetAdmission(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		t.Run(fmt.Sprintf("python=%t", enabled), func(t *testing.T) {
			p := &v1alpha1.CNPool{ObjectMeta: metav1.ObjectMeta{Name: "pool", Namespace: "ns", UID: "pool-uid"}}
			p.Spec.Template.Image = "matrixone:1.2.0"
			p.Spec.Template.SemanticVersion = ptrString("1.2.0")
			p.Spec.Template.Resources = corev1.ResourceRequirements{Limits: corev1.ResourceList{
				corev1.ResourceCPU: resource.MustParse("1"), corev1.ResourceMemory: resource.MustParse("1Gi"),
			}}
			p.Spec.Deps.ExternalLogSet = &v1alpha1.ExternalLogSet{HAKeeperEndpoint: "log:32001"}
			if enabled {
				p.Spec.Template.UDFWorker = webhookTestPolicy()
			}
			scheme := runtime.NewScheme()
			if err := v1alpha1.AddToScheme(scheme); err != nil {
				t.Fatal(err)
			}
			if err := corev1.AddToScheme(scheme); err != nil {
				t.Fatal(err)
			}
			base := reconfake.KubeClientBuilder().WithScheme(scheme).WithObjects(p).WithStatusSubresource(&v1alpha1.CNSet{}).Build()
			cli := &poolAdmissionClient{Client: base}
			ctx := reconfake.NewContext(p, cli, nil)
			actor := &cnpool.Actor{}
			if err := actor.Sync(ctx); err != nil {
				t.Fatal(err)
			}
			sets := &v1alpha1.CNSetList{}
			if err := cli.List(context.Background(), sets); err != nil {
				t.Fatal(err)
			}
			if len(sets.Items) != 1 {
				t.Fatalf("initial sets: %d", len(sets.Items))
			}
			old := sets.Items[0].DeepCopy()
			old.Status.Replicas = 1 // retain A until its existing Pod disappears
			if err := cli.Status().Update(context.Background(), old); err != nil {
				t.Fatal(err)
			}
			if enabled {
				p.Spec.Template.UDFWorker.Worker.Image = "registry.example/udf-worker@sha256:" + strings.Repeat("c", 64)
			} else {
				p.Spec.Template.Image = "matrixone:1.2.1"
			}
			if err := actor.Sync(ctx); err != nil {
				t.Fatalf("revision transition rejected: %v", err)
			}
			if err := cli.List(context.Background(), sets); err != nil {
				t.Fatal(err)
			}
			if len(sets.Items) != 2 {
				t.Fatalf("new revision not created: %d", len(sets.Items))
			}
			if err := cli.Get(context.Background(), client.ObjectKeyFromObject(old), old); err != nil {
				t.Fatal(err)
			}
			if old.Labels[v1alpha1.PodOutdatedLabel] != "y" {
				t.Fatal("legacy revision lost its claim exclusion")
			}
			if old.Spec.Overlay != nil {
				if _, ok := old.Spec.Overlay.PodLabels[v1alpha1.PodOutdatedLabel]; ok {
					t.Fatal("controller state leaked into user overlay")
				}
			}
			if cli.creates != 2 || cli.updates == 0 {
				t.Fatalf("admission was not exercised: create=%d update=%d", cli.creates, cli.updates)
			}
			// Selecting A again must clear its old exclusion through the same
			// producer/validator path, not only in the Pod renderer.
			if enabled {
				p.Spec.Template.UDFWorker = webhookTestPolicy()
			} else {
				p.Spec.Template.Image = "matrixone:1.2.0"
			}
			if err := actor.Sync(ctx); err != nil {
				t.Fatalf("revision rollback rejected: %v", err)
			}
			if err := cli.Get(context.Background(), client.ObjectKeyFromObject(old), old); err != nil {
				t.Fatal(err)
			}
			if _, ok := old.Labels[v1alpha1.PodOutdatedLabel]; ok {
				t.Fatal("current revision retained outdated metadata")
			}

		})
	}
}

type poolAdmissionClient struct {
	client.Client
	creates, updates int
}

func (c *poolAdmissionClient) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	if cn, ok := obj.(*v1alpha1.CNSet); ok {
		if err := (&cnSetDefaulter{}).Default(ctx, cn); err != nil {
			return err
		}
		if _, err := (&cnSetValidator{}).ValidateCreate(ctx, cn); err != nil {
			return err
		}
		c.creates++
	}
	return c.Client.Create(ctx, obj, opts...)
}

func (c *poolAdmissionClient) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	if cn, ok := obj.(*v1alpha1.CNSet); ok {
		old := &v1alpha1.CNSet{}
		if err := c.Client.Get(ctx, client.ObjectKeyFromObject(cn), old); err != nil {
			return err
		}
		if err := (&cnSetDefaulter{}).Default(ctx, cn); err != nil {
			return err
		}
		if _, err := (&cnSetValidator{}).ValidateUpdate(ctx, old, cn); err != nil {
			return err
		}
		c.updates++
	}
	return c.Client.Update(ctx, obj, opts...)
}
