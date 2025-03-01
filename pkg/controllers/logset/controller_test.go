// Copyright 2025 Matrix Origin
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
package logset

import (
	"context"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/matrixorigin/controller-runtime/pkg/fake"
	recon "github.com/matrixorigin/controller-runtime/pkg/reconciler"
	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	"github.com/matrixorigin/matrixone-operator/pkg/controllers/common"
	kruisev1alpha1 "github.com/openkruise/kruise-api/apps/v1alpha1"
	kruisev1 "github.com/openkruise/kruise-api/apps/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/onsi/gomega"
)

func TestLogSetActor_Observe(t *testing.T) {
	s := newScheme()
	tpl := &v1alpha1.LogSet{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "test",
		},
		Spec: v1alpha1.LogSetSpec{
			PodSet: v1alpha1.PodSet{
				MainContainer: v1alpha1.MainContainer{
					Image: "test:latest",
				},
				Replicas: 3,
			},
			InitialConfig: v1alpha1.InitialConfig{
				LogShards:        pointer.Int(1),
				DNShards:         pointer.Int(1),
				LogShardReplicas: pointer.Int(3),
			},
		},
	}
	labels := common.SubResourceLabels(tpl)
	tests := []struct {
		name   string
		logset *v1alpha1.LogSet
		client client.Client
		expect func(g *WithT, cli client.Client, action recon.Action[*v1alpha1.LogSet], err error)
	}{{
		name:   "create when resource not exist",
		logset: tpl,
		client: &fake.Client{
			Client: fake.KubeClientBuilder().WithScheme(s).Build(),
		},
		expect: func(g *WithT, _ client.Client, action recon.Action[*v1alpha1.LogSet], err error) {
			g.Expect(err).To(BeNil())
			g.Expect(action.String()).To(ContainSubstring("Create"))
		},
	}, {
		name:   "update when resource not update ot date",
		logset: tpl,
		client: &fake.Client{
			Client: fake.KubeClientBuilder().WithScheme(s).WithObjects(
				&kruisev1.StatefulSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-log",
						Namespace: "default",
					},
					Spec: kruisev1.StatefulSetSpec{
						Replicas: pointer.Int32(3),
						Template: corev1.PodTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Labels: labels,
							},
							Spec: corev1.PodSpec{
								Containers: []corev1.Container{{
									Name:  "main",
									Image: "test:latest",
								}},
								Volumes: []corev1.Volume{},
							},
						},
						ServiceName: "test-svc",
					},
				},
				&corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-log-discovery",
						Namespace: "default",
					},
				},
			).Build(),
		},
		expect: func(g *WithT, _ client.Client, action recon.Action[*v1alpha1.LogSet], err error) {
			g.Expect(err).To(BeNil())
			g.Expect(action.String()).To(ContainSubstring("Update"))
		},
	}, {
		name:   "scale out",
		logset: tpl,
		client: &fake.Client{
			MockPatch: func(ctx context.Context, obj runtime.Object, patch client.Patch, opts ...client.PatchOption) error {
				return nil
			},
			Client: fake.KubeClientBuilder().WithScheme(s).WithObjects(
				&kruisev1.StatefulSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-log",
						Namespace: "default",
					},
					Spec: kruisev1.StatefulSetSpec{
						Replicas: pointer.Int32(0),
						Template: corev1.PodTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{},
							Spec:       corev1.PodSpec{},
						},
						ServiceName: "test-svc",
					},
				},
				&corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-log-discovery",
						Namespace: "default",
					},
				},
			).Build(),
		},
		expect: func(g *WithT, _ client.Client, action recon.Action[*v1alpha1.LogSet], err error) {
			g.Expect(err).To(BeNil())
			g.Expect(action.String()).To(ContainSubstring("Scale"))
		},
	}, {
		name: "failover",
		logset: func() *v1alpha1.LogSet {
			ls := tpl.DeepCopy()
			ls.Status.FailedStores = []v1alpha1.Store{{
				PodName:            "test-log-0",
				Phase:              v1alpha1.StorePhaseDown,
				LastTransitionTime: metav1.Time{Time: time.Now().Add(-24 * time.Hour)},
			}}
			return ls
		}(),
		client: &fake.Client{
			Client: fake.KubeClientBuilder().WithScheme(s).WithObjects(
				&kruisev1.StatefulSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-log",
						Namespace: "default",
					},
					Spec: kruisev1.StatefulSetSpec{
						Replicas: pointer.Int32(3),
						Template: corev1.PodTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{},
							Spec:       corev1.PodSpec{},
						},
						ServiceName: "test-svc",
					},
				},
				&corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-log-discovery",
						Namespace: "default",
					},
				},
				fake.UnreadyPod(metav1.ObjectMeta{
					Name:      "test-log-0",
					Namespace: "default",
					Labels:    labels,
				}),
				fake.ReadyPod(metav1.ObjectMeta{
					Name:      "test-log-1",
					Namespace: "default",
					Labels:    labels,
				}),
				fake.ReadyPod(metav1.ObjectMeta{
					Name:      "test-log-2",
					Namespace: "default",
					Labels:    labels,
				}),
			).Build(),
		},
		expect: func(g *WithT, cli client.Client, action recon.Action[*v1alpha1.LogSet], err error) {
			g.Expect(err).To(BeNil())
			g.Expect(action.String()).To(ContainSubstring("Repair"))
			asts := &kruisev1.StatefulSet{}
			g.Expect(cli.Get(context.TODO(), types.NamespacedName{Namespace: "default", Name: "test-log"}, asts)).To(Succeed())
			g.Expect(asts.Spec.ReserveOrdinals).To(ConsistOf(0))
		},
	}, {
		name: "should not failover if minority limit is reached",
		logset: func() *v1alpha1.LogSet {
			ls := tpl.DeepCopy()
			ls.Status.FailedStores = []v1alpha1.Store{{
				PodName:            "test-log-0",
				Phase:              v1alpha1.StorePhaseDown,
				LastTransitionTime: metav1.Time{Time: time.Now().Add(-24 * time.Hour)},
			}}
			return ls
		}(),
		client: &fake.Client{
			Client: fake.KubeClientBuilder().WithScheme(s).WithObjects(
				&kruisev1.StatefulSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-log",
						Namespace: "default",
					},
					Spec: kruisev1.StatefulSetSpec{
						Replicas: pointer.Int32(0),
						Template: corev1.PodTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{},
							Spec:       corev1.PodSpec{},
						},
						ServiceName:     "test-svc",
						ReserveOrdinals: []int{1},
					},
				},
				&corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-log-discovery",
						Namespace: "default",
					},
				},
				fake.UnreadyPod(metav1.ObjectMeta{
					Name:      "test-log-0",
					Namespace: "default",
					Labels:    labels,
				}),
				fake.ReadyPod(metav1.ObjectMeta{
					Name:      "test-log-2",
					Namespace: "default",
					Labels:    labels,
				}),
				fake.ReadyPod(metav1.ObjectMeta{
					Name:      "test-log-3",
					Namespace: "default",
					Labels:    labels,
				}),
			).Build(),
		},
		expect: func(g *WithT, cli client.Client, action recon.Action[*v1alpha1.LogSet], err error) {
			g.Expect(err).To(BeNil())
			g.Expect(action.String()).To(ContainSubstring("Repair"))
			asts := &kruisev1.StatefulSet{}
			g.Expect(cli.Get(context.TODO(), types.NamespacedName{Namespace: "default", Name: "test-log"}, asts)).To(Succeed())
			g.Expect(asts.Spec.ReserveOrdinals).To(ConsistOf(1))
		},
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewGomegaWithT(t)
			r := &Actor{
				FailoverEnabled: true,
			}
			mockCtrl := gomock.NewController(t)
			eventEmitter := fake.NewMockEventEmitter(mockCtrl)
			ls := tt.logset.DeepCopy()
			g.Expect(tt.client.Create(context.TODO(), ls)).To(Succeed())
			ctx := fake.NewContext(ls, tt.client, eventEmitter)
			action, err := r.Observe(ctx)
			if action != nil {
				g.Expect(action(ctx)).To(Succeed())
			}
			tt.expect(g, tt.client, action, err)
		})
	}
}

func TestLogSetActor_Create(t *testing.T) {
	type args struct {
		ctx *recon.Context[*v1alpha1.LogSet]
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &Actor{}
			if err := r.Create(tt.args.ctx); (err != nil) != tt.wantErr {
				t.Errorf("Create() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestLogSetActor_Finalize(t *testing.T) {
	s := newScheme()
	type args struct {
		ctx *recon.Context[*v1alpha1.LogSet]
	}
	tests := []struct {
		name   string
		logset *v1alpha1.LogSet
		client client.Client
		expect func(g *GomegaWithT, cli client.Client)
	}{
		{
			name: "deleteOrpanedPod",
			logset: &v1alpha1.LogSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "default",
				},
			},
			client: &fake.Client{
				Client: fake.KubeClientBuilder().WithScheme(s).WithObjects(
					&corev1.Pod{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "test-0",
							Namespace: "default",
							Labels: map[string]string{
								common.ActionRequiredLabelKey: common.ActionRequiredLabelValue,
								common.LogSetOwnerKey:         "test",
							},
							Finalizers: []string{failoverDeletionFinalizer},
						},
					},
				).Build(),
			},
			expect: func(g *GomegaWithT, cli client.Client) {
				pods := &corev1.PodList{}
				g.Expect(cli.List(context.TODO(), pods)).To(Succeed())
				g.Expect(pods.Items).To(BeEmpty())
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewGomegaWithT(t)
			r := &Actor{}
			mockCtrl := gomock.NewController(t)
			eventEmitter := fake.NewMockEventEmitter(mockCtrl)
			ctx := fake.NewContext(tt.logset, tt.client, eventEmitter)
			ok, err := r.Finalize(ctx)
			g.Expect(ok).To(BeFalse())
			g.Expect(err).To(Succeed())
			tt.expect(g, tt.client)
		})
	}
}

func TestWithResources_Repair(t *testing.T) {
	type fields struct {
		LogSetActor *Actor
		sts         *kruisev1.StatefulSet
	}
	type args struct {
		ctx *recon.Context[*v1alpha1.LogSet]
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &WithResources{
				Actor: tt.fields.LogSetActor,
				sts:   tt.fields.sts,
			}
			if err := r.Repair(tt.args.ctx); (err != nil) != tt.wantErr {
				t.Errorf("Repair() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestWithResources_Scale(t *testing.T) {
	type fields struct {
		LogSetActor *Actor
		sts         *kruisev1.StatefulSet
	}
	type args struct {
		ctx *recon.Context[*v1alpha1.LogSet]
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &WithResources{
				Actor: tt.fields.LogSetActor,
				sts:   tt.fields.sts,
			}
			if err := r.Scale(tt.args.ctx); (err != nil) != tt.wantErr {
				t.Errorf("Scale() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestWithResources_Update(t *testing.T) {
	type fields struct {
		LogSetActor *Actor
		sts         *kruisev1.StatefulSet
	}
	type args struct {
		ctx *recon.Context[*v1alpha1.LogSet]
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &WithResources{
				Actor: tt.fields.LogSetActor,
				sts:   tt.fields.sts,
			}
			if err := r.Update(tt.args.ctx); (err != nil) != tt.wantErr {
				t.Errorf("Update() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_syncPods(t *testing.T) {
	type args struct {
		ctx *recon.Context[*v1alpha1.LogSet]
		sts *kruisev1.StatefulSet
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := syncPods(tt.args.ctx, tt.args.sts); (err != nil) != tt.wantErr {
				t.Errorf("syncPods() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func newScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(v1alpha1.AddToScheme(scheme))
	utilruntime.Must(kruisev1.AddToScheme(scheme))
	utilruntime.Must(kruisev1alpha1.AddToScheme(scheme))
	return scheme
}
