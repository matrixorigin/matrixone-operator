// Copyright 2023 Matrix Origin
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
	"github.com/matrixorigin/controller-runtime/pkg/fake"
	recon "github.com/matrixorigin/controller-runtime/pkg/reconciler"
	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	. "github.com/onsi/gomega"
	kruisev1 "github.com/openkruise/kruise-api/apps/v1beta1"
	kruisepolicy "github.com/openkruise/kruise-api/policy/v1alpha1"
	corev1 "k8s.io/api/core/v1"
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
			LogService: v1alpha1.LogSetSpec{
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
			DN: v1alpha1.DNSetSpec{
				PodSet: v1alpha1.PodSet{
					Replicas: 2,
				},
			},
			TP: v1alpha1.CNSetSpec{
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
		expect  func(g *WithT, mo *v1alpha1.MatrixOneCluster, err error, c client.Client)

		expectAction func(g *WithT, action recon.Action[*v1alpha1.MatrixOneCluster])
	}{{
		name:    "create",
		objects: nil,
		mo:      tpl.DeepCopy(),
		expect: func(g *WithT, mo *v1alpha1.MatrixOneCluster, _ error, c client.Client) {
			g.Expect(c.Get(ctx, types.NamespacedName{Namespace: "default", Name: "test"}, &v1alpha1.LogSet{})).To(Succeed())
			g.Expect(c.Get(ctx, types.NamespacedName{Namespace: "default", Name: "test"}, &v1alpha1.DNSet{})).To(Succeed())
			g.Expect(c.Get(ctx, types.NamespacedName{Namespace: "default", Name: "test-tp"}, &v1alpha1.CNSet{})).To(Succeed())
			g.Expect(recon.IsReady(mo)).To(BeFalse())
			g.Expect(recon.IsSynced(mo)).To(BeFalse())
		},
	}, {
		name: "ready",
		mo: func() *v1alpha1.MatrixOneCluster {
			mo := tpl.DeepCopy()
			mo.Status.CredentialRef = &corev1.LocalObjectReference{Name: "test"}
			return mo
		}(),
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
		expect: func(g *WithT, mo *v1alpha1.MatrixOneCluster, err error, c client.Client) {
			g.Expect(recon.IsReady(&mo.Status)).To(BeTrue())
			g.Expect(err).To(Succeed())
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
		expect: func(g *WithT, mo *v1alpha1.MatrixOneCluster, err error, c client.Client) {
			g.Expect(recon.IsSynced(&mo.Status)).To(BeFalse())
			cond, ok := recon.GetCondition(&mo.Status, recon.ConditionTypeReady)
			g.Expect(ok).To(BeTrue())
			g.Expect(cond.Reason).To(Equal("DNSetNotReady"))
		},
	}, {
		name: "LogSetNotSynced",
		mo:   tpl.DeepCopy(),
		objects: []client.Object{
			&v1alpha1.LogSet{
				ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "test"},
				Status: v1alpha1.LogSetStatus{
					ConditionalStatus: v1alpha1.ConditionalStatus{Conditions: []metav1.Condition{{
						Type:   recon.ConditionTypeSynced,
						Status: metav1.ConditionFalse,
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
		expect: func(g *WithT, mo *v1alpha1.MatrixOneCluster, err error, c client.Client) {
			g.Expect(recon.IsSynced(&mo.Status)).To(BeFalse())
			cond, ok := recon.GetCondition(&mo.Status, recon.ConditionTypeSynced)
			g.Expect(ok).To(BeTrue())
			g.Expect(cond.Reason).To(Equal("LogServiceNotSynced"))
		},
	}, {
		name: "initializeDB",
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
		expect: func(g *WithT, mo *v1alpha1.MatrixOneCluster, err error, c client.Client) {
			g.Expect(recon.IsReady(&mo.Status)).To(BeFalse())
		},
		expectAction: func(g *WithT, action recon.Action[*v1alpha1.MatrixOneCluster]) {
			g.Expect(action.String()).To(ContainSubstring("Initialize"))
		},
	}, {
		name: "inheritOrOverrideGlobalNodeSelector",
		mo: func() *v1alpha1.MatrixOneCluster {
			m := tpl.DeepCopy()
			m.Spec.NodeSelector = map[string]string{
				"global-label": "global-value",
			}
			m.Spec.TP.NodeSelector = map[string]string{
				"local-label": "local-value",
			}
			return m
		}(),
		objects: nil,
		expect: func(g *WithT, _ *v1alpha1.MatrixOneCluster, err error, c client.Client) {
			dn := &v1alpha1.DNSet{}
			g.Expect(c.Get(ctx, types.NamespacedName{Namespace: "default", Name: "test"}, dn)).To(Succeed())
			g.Expect(dn.Spec.NodeSelector).To(Equal(map[string]string{"global-label": "global-value"}))
			ls := &v1alpha1.LogSet{}
			g.Expect(c.Get(ctx, types.NamespacedName{Namespace: "default", Name: "test"}, ls)).To(Succeed())
			g.Expect(ls.Spec.NodeSelector).To(Equal(map[string]string{"global-label": "global-value"}))
			cn := &v1alpha1.CNSet{}
			g.Expect(c.Get(ctx, types.NamespacedName{Namespace: "default", Name: "test-tp"}, cn)).To(Succeed())
			g.Expect(cn.Spec.NodeSelector).To(Equal(map[string]string{"local-label": "local-value"}))
		},
	}, {
		name: "setImagePullPolicy",
		mo: func() *v1alpha1.MatrixOneCluster {
			m := tpl.DeepCopy()
			policy := corev1.PullIfNotPresent
			m.Spec.ImagePullPolicy = &policy
			return m
		}(),
		objects: nil,
		expect: func(g *WithT, _ *v1alpha1.MatrixOneCluster, err error, c client.Client) {
			dn := &v1alpha1.DNSet{}
			g.Expect(c.Get(ctx, types.NamespacedName{Namespace: "default", Name: "test"}, dn)).To(Succeed())
			g.Expect(*dn.Spec.Overlay.ImagePullPolicy).To(Equal(corev1.PullIfNotPresent))
			ls := &v1alpha1.LogSet{}
			g.Expect(c.Get(ctx, types.NamespacedName{Namespace: "default", Name: "test"}, ls)).To(Succeed())
			g.Expect(*ls.Spec.Overlay.ImagePullPolicy).To(Equal(corev1.PullIfNotPresent))
			cn := &v1alpha1.CNSet{}
			g.Expect(c.Get(ctx, types.NamespacedName{Namespace: "default", Name: "test-tp"}, cn)).To(Succeed())
			g.Expect(*cn.Spec.Overlay.ImagePullPolicy).To(Equal(corev1.PullIfNotPresent))
		},
	}, {
		name: "syncRetentionPolicy",
		mo: func() *v1alpha1.MatrixOneCluster {
			m := tpl.DeepCopy()
			policy := v1alpha1.PVCRetentionPolicyRetain
			m.Spec.LogService.PVCRetentionPolicy = &policy
			return m
		}(),
		objects: nil,
		expect: func(g *WithT, _ *v1alpha1.MatrixOneCluster, err error, c client.Client) {
			ls := &v1alpha1.LogSet{}
			g.Expect(c.Get(ctx, types.NamespacedName{Namespace: "default", Name: "test"}, ls)).To(Succeed())
			g.Expect(*ls.Spec.PVCRetentionPolicy).To(Equal(v1alpha1.PVCRetentionPolicyRetain))
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
			action, err := r.Observe(ctx)
			tt.expect(g, tt.mo, err, cli)
			if tt.expectAction != nil {
				tt.expectAction(g, action)
			}
		})
	}
}

func TestMatrixOneClusterActor_Initialize(t *testing.T) {
	s := newScheme()
	tests := []struct {
		name   string
		mo     *v1alpha1.MatrixOneCluster
		expect func(g *GomegaWithT, cli client.Client, mo *v1alpha1.MatrixOneCluster)
	}{
		{
			name: "basic",
			mo: &v1alpha1.MatrixOneCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test",
					Name:      "test",
				},
			},
			expect: func(g *GomegaWithT, cli client.Client, mo *v1alpha1.MatrixOneCluster) {
				sec := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-credential",
						Namespace: "test",
					},
				}
				g.Expect(cli.Get(context.TODO(), client.ObjectKeyFromObject(sec), sec)).To(Succeed())
				g.Expect(mo.Status.CredentialRef).NotTo(BeNil())
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewGomegaWithT(t)
			r := &MatrixOneClusterActor{}
			cli := fake.KubeClientBuilder().WithScheme(s).WithObjects(tt.mo).Build()
			mockCtrl := gomock.NewController(t)
			eventEmitter := fake.NewMockEventEmitter(mockCtrl)
			ctx := fake.NewContext(tt.mo, cli, eventEmitter)
			err := r.Initialize(ctx)
			g.Expect(err).To(Succeed())
			tt.expect(g, cli, tt.mo)
		})
	}
}

func newScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(v1alpha1.AddToScheme(scheme))
	utilruntime.Must(kruisev1.AddToScheme(scheme))
	utilruntime.Must(kruisepolicy.AddToScheme(scheme))
	return scheme
}
