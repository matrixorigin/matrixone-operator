// Copyright 2022 Matrix Origin
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package mocluster

import (
	"context"
	"github.com/golang/mock/gomock"
	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	"github.com/matrixorigin/matrixone-operator/runtime/pkg/fake"
	recon "github.com/matrixorigin/matrixone-operator/runtime/pkg/reconciler"
	. "github.com/onsi/gomega"
	kruisev1 "github.com/openkruise/kruise-api/apps/v1beta1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"testing"
)

func TestMatrixOneClusterActor_Observe(t *testing.T) {
	s := newScheme()
	tpl := &v1alpha1.MatrixOneCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "default",
		},
		Spec: v1alpha1.MatrixOneClusterSpec{
			LogService: v1alpha1.LogSetBasic{
				PodSet: v1alpha1.PodSet{
					Replicas: 3,
				},
				Volume: v1alpha1.Volume{
					Size: resource.MustParse("10Gi"),
				},
				SharedStorage: v1alpha1.SharedStorageProvider{
					S3: &v1alpha1.S3Provider{Path: "test/data"},
				},
			},
			DN: v1alpha1.DNSetBasic{
				PodSet: v1alpha1.PodSet{
					Replicas: 2,
				},
			},
			TP: v1alpha1.CNSetBasic{
				PodSet: v1alpha1.PodSet{
					Replicas: 2,
				},
			},
			Version: "test",
		},
	}
	ctx := context.Background()
	tests := []struct {
		name    string
		objects []client.Object
		mo      *v1alpha1.MatrixOneCluster
		expect  func(g *WithT, mo *v1alpha1.MatrixOneCluster, c client.Client)
	}{{
		name:    "create",
		objects: nil,
		mo:      tpl.DeepCopy(),
		expect: func(g *WithT, mo *v1alpha1.MatrixOneCluster, c client.Client) {
			g.Expect(c.Get(ctx, types.NamespacedName{Namespace: "default", Name: "test"}, &v1alpha1.LogSet{})).To(Succeed())
			g.Expect(c.Get(ctx, types.NamespacedName{Namespace: "default", Name: "test"}, &v1alpha1.DNSet{})).To(Succeed())
			g.Expect(c.Get(ctx, types.NamespacedName{Namespace: "default", Name: "test-tp"}, &v1alpha1.CNSet{})).To(Succeed())
			g.Expect(recon.IsReady(mo)).To(BeFalse())
			g.Expect(recon.IsSynced(mo)).To(BeFalse())
		},
	}, {
		name: "ready",
		mo:   tpl.DeepCopy(),
		objects: []client.Object{
			&v1alpha1.LogSet{
				ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "test"},
				Status: v1alpha1.LogSetStatus{
					ConditionalStatus: v1alpha1.ConditionalStatus{Conditions: []metav1.Condition{{
						Type:   recon.ConditionTypeReady,
						Status: metav1.ConditionTrue,
					}}},
				},
			},
			&v1alpha1.DNSet{
				ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "test"},
				Status: v1alpha1.DNSetStatus{
					ConditionalStatus: v1alpha1.ConditionalStatus{Conditions: []metav1.Condition{{
						Type:   recon.ConditionTypeReady,
						Status: metav1.ConditionTrue,
					}}},
				},
			},
			&v1alpha1.CNSet{
				ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "test-tp"},
				Status: v1alpha1.CNSetStatus{
					ConditionalStatus: v1alpha1.ConditionalStatus{Conditions: []metav1.Condition{{
						Type:   recon.ConditionTypeReady,
						Status: metav1.ConditionTrue,
					}}},
				},
			},
		},
		expect: func(g *WithT, mo *v1alpha1.MatrixOneCluster, c client.Client) {
			g.Expect(recon.IsReady(&mo.Status)).To(BeTrue())
		},
	}, {
		name: "synced",
		mo:   tpl.DeepCopy(),
		objects: []client.Object{
			&v1alpha1.LogSet{
				ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "test"},
				Status: v1alpha1.LogSetStatus{
					ConditionalStatus: v1alpha1.ConditionalStatus{Conditions: []metav1.Condition{{
						Type:   recon.ConditionTypeSynced,
						Status: metav1.ConditionTrue,
					}}},
				},
			},
			&v1alpha1.DNSet{
				ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "test"},
				Status: v1alpha1.DNSetStatus{
					ConditionalStatus: v1alpha1.ConditionalStatus{Conditions: []metav1.Condition{{
						Type:   recon.ConditionTypeSynced,
						Status: metav1.ConditionTrue,
					}}},
				},
			},
			&v1alpha1.CNSet{
				ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "test-tp"},
				Status: v1alpha1.CNSetStatus{
					ConditionalStatus: v1alpha1.ConditionalStatus{Conditions: []metav1.Condition{{
						Type:   recon.ConditionTypeSynced,
						Status: metav1.ConditionTrue,
					}}},
				},
			},
		},
		expect: func(g *WithT, mo *v1alpha1.MatrixOneCluster, c client.Client) {
			g.Expect(recon.IsSynced(&mo.Status)).To(BeTrue())
		},
	}, {
		name: "DNNotReady",
		mo:   tpl.DeepCopy(),
		objects: []client.Object{
			&v1alpha1.LogSet{
				ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "test"},
				Status: v1alpha1.LogSetStatus{
					ConditionalStatus: v1alpha1.ConditionalStatus{Conditions: []metav1.Condition{{
						Type:   recon.ConditionTypeReady,
						Status: metav1.ConditionTrue,
					}}},
				},
			},
			&v1alpha1.DNSet{
				ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "test"},
				Status: v1alpha1.DNSetStatus{
					ConditionalStatus: v1alpha1.ConditionalStatus{Conditions: []metav1.Condition{{
						Type:   recon.ConditionTypeReady,
						Status: metav1.ConditionFalse,
					}}},
				},
			},
			&v1alpha1.CNSet{
				ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "test-tp"},
				Status: v1alpha1.CNSetStatus{
					ConditionalStatus: v1alpha1.ConditionalStatus{Conditions: []metav1.Condition{{
						Type:   recon.ConditionTypeReady,
						Status: metav1.ConditionTrue,
					}}},
				},
			},
		},
		expect: func(g *WithT, mo *v1alpha1.MatrixOneCluster, c client.Client) {
			g.Expect(recon.IsSynced(&mo.Status)).To(BeFalse())
			cond, ok := recon.GetCondition(&mo.Status, recon.ConditionTypeReady)
			g.Expect(ok).To(BeTrue())
			g.Expect(cond.Reason).To(Equal("DNSetNotReady"))
		},
	}, {
		name: "DNNotSynced",
		mo:   tpl.DeepCopy(),
		objects: []client.Object{
			&v1alpha1.LogSet{
				ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "test"},
				Status: v1alpha1.LogSetStatus{
					ConditionalStatus: v1alpha1.ConditionalStatus{Conditions: []metav1.Condition{{
						Type:   recon.ConditionTypeSynced,
						Status: metav1.ConditionTrue,
					}}},
				},
			},
			&v1alpha1.DNSet{
				ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "test"},
				Status: v1alpha1.DNSetStatus{
					ConditionalStatus: v1alpha1.ConditionalStatus{Conditions: []metav1.Condition{{
						Type:   recon.ConditionTypeSynced,
						Status: metav1.ConditionFalse,
					}}},
				},
			},
			&v1alpha1.CNSet{
				ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "test-tp"},
				Status: v1alpha1.CNSetStatus{
					ConditionalStatus: v1alpha1.ConditionalStatus{Conditions: []metav1.Condition{{
						Type:   recon.ConditionTypeSynced,
						Status: metav1.ConditionTrue,
					}}},
				},
			},
		},
		expect: func(g *WithT, mo *v1alpha1.MatrixOneCluster, c client.Client) {
			g.Expect(recon.IsSynced(&mo.Status)).To(BeFalse())
			cond, ok := recon.GetCondition(&mo.Status, recon.ConditionTypeSynced)
			g.Expect(ok).To(BeTrue())
			g.Expect(cond.Reason).To(Equal("DNSetNotSynced"))
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cli := fake.KubeClientBuilder().WithScheme(s).WithObjects(tt.objects...).Build()
			g := NewGomegaWithT(t)
			r := &MatrixOneClusterActor{}
			mockCtrl := gomock.NewController(t)
			eventEmitter := fake.NewMockEventEmitter(mockCtrl)
			ctx := fake.NewContext(tt.mo, cli, eventEmitter)
			_, err := r.Observe(ctx)
			g.Expect(err).To(Succeed())
			tt.expect(g, tt.mo, cli)
		})
	}
}

func newScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(v1alpha1.AddToScheme(scheme))
	utilruntime.Must(kruisev1.AddToScheme(scheme))
	return scheme
}
