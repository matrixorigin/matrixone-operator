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

package cnset

import (
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	"github.com/matrixorigin/matrixone-operator/pkg/controllers/common"
	"github.com/matrixorigin/matrixone-operator/runtime/pkg/fake"
	recon "github.com/matrixorigin/matrixone-operator/runtime/pkg/reconciler"
	. "github.com/onsi/gomega"
	kruisev1 "github.com/openkruise/kruise-api/apps/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestCNSetActor_Observe(t *testing.T) {
	s := newScheme()
	tpl := &v1alpha1.CNSet{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "test",
		},
		Spec: v1alpha1.CNSetSpec{
			CNSetBasic: v1alpha1.CNSetBasic{
				PodSet: v1alpha1.PodSet{
					MainContainer: v1alpha1.MainContainer{
						Image: "test:latest",
					},
					Replicas: 1,
				},
				ServiceType: corev1.ServiceTypeClusterIP,
				// TODO: add configuration of cn
			},
		},
	}
	labels := common.SubResourceLabels(tpl)
	n := stsName(tpl)
	svc := svcName(tpl)
	tests := []struct {
		name   string
		cnset  *v1alpha1.CNSet
		client client.Client
		expect func(g *WithT, action recon.Action[*v1alpha1.CNSet], err error)
	}{
		{
			name:  "create when resource not exist",
			cnset: tpl,
			client: &fake.Client{
				Client: fake.KubeClientBuilder().WithScheme(s).Build(),
			},
			expect: func(g *WithT, action recon.Action[*v1alpha1.CNSet], err error) {
				g.Expect(err).To(BeNil())
				g.Expect(action.String()).To(ContainSubstring("Create"))
			},
		},
		{
			name:  "update when resource not update to date",
			cnset: tpl,
			client: &fake.Client{
				Client: fake.KubeClientBuilder().WithScheme(s).WithObjects(
					&kruisev1.StatefulSet{
						ObjectMeta: metav1.ObjectMeta{
							Name:      n,
							Namespace: "default",
						},
						Spec: kruisev1.StatefulSetSpec{
							Replicas: pointer.Int32(1),
							Template: corev1.PodTemplateSpec{
								ObjectMeta: metav1.ObjectMeta{
									Labels: labels,
								},
								Spec: corev1.PodSpec{
									Containers: []corev1.Container{
										{
											Name:  "main",
											Image: "test:latest",
										},
									},
									Volumes: []corev1.Volume{},
								},
							},
							ServiceName: svc,
						},
					},
					&corev1.Service{
						ObjectMeta: metav1.ObjectMeta{
							Name:      svc,
							Namespace: "default",
						},
					},
				).Build(),
			},
			expect: func(g *WithT, action recon.Action[*v1alpha1.CNSet], err error) {
				g.Expect(err).To(BeNil())
				g.Expect(action.String()).To(ContainSubstring("Update"))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewGomegaWithT(t)
			r := &Actor{}
			mockCtrl := gomock.NewController(t)
			eventEmitter := fake.NewMockEventEmitter(mockCtrl)
			ctx := fake.NewContext(tt.cnset, tt.client, eventEmitter)
			ctx.Dep = tt.cnset.DeepCopy()
			ctx.Dep.Deps.LogSet = &v1alpha1.LogSet{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "default",
					Name:      "test",
				},
				Spec: v1alpha1.LogSetSpec{
					LogSetBasic: v1alpha1.LogSetBasic{
						SharedStorage: v1alpha1.SharedStorageProvider{
							S3: &v1alpha1.S3Provider{
								Path: "bucket/dir",
							},
						},
					},
				},
				Status: v1alpha1.LogSetStatus{
					Discovery: &v1alpha1.LogSetDiscovery{
						Port:    6001,
						Address: "test",
					},
				},
			}
			action, err := r.Observe(ctx)
			tt.expect(g, action, err)
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
