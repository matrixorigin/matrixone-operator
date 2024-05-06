// Copyright 2024 Matrix Origin
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

package cnpool

import (
	"context"
	"github.com/go-logr/logr"
	"github.com/golang/mock/gomock"
	"github.com/matrixorigin/controller-runtime/pkg/fake"
	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	"github.com/matrixorigin/matrixone-operator/pkg/utils"
	kruisev1alpha1 "github.com/openkruise/kruise-api/apps/v1alpha1"
	kruisev1 "github.com/openkruise/kruise-api/apps/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"math/rand"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"testing"
	"time"

	. "github.com/onsi/gomega"
)

func Test_sortPodByDeletionOrder(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name  string
		pods  []*corev1.Pod
		order []string
	}{{
		name: "basic",
		pods: []*corev1.Pod{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "idle-new",
					Labels: map[string]string{
						v1alpha1.CNPodPhaseLabel: v1alpha1.CNPodPhaseIdle,
					},
					CreationTimestamp: metav1.NewTime(now),
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "unknown",
					Labels: map[string]string{
						v1alpha1.CNPodPhaseLabel: v1alpha1.CNPodPhaseUnknown,
					},
					CreationTimestamp: metav1.NewTime(now),
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "idle-old",
					Labels: map[string]string{
						v1alpha1.CNPodPhaseLabel: v1alpha1.CNPodPhaseIdle,
					},
					CreationTimestamp: metav1.NewTime(now.Add(-time.Hour)),
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "terminating",
					Labels: map[string]string{
						v1alpha1.CNPodPhaseLabel: v1alpha1.CNPodPhaseTerminating,
					},
					CreationTimestamp: metav1.NewTime(now),
				},
			},
		},
		order: []string{
			"terminating",
			"unknown",
			"idle-new",
			"idle-old",
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rand.Shuffle(len(tt.pods), func(i, j int) {
				tt.pods[i], tt.pods[j] = tt.pods[j], tt.pods[i]
			})
			sortPodByDeletionOrder(tt.pods)
			g := NewGomegaWithT(t)
			var res []string
			for _, po := range tt.pods {
				res = append(res, po.Name)
			}
			g.Expect(res).To(Equal(tt.order))
		})
	}
}

func Test_syncLegacySet(t *testing.T) {
	s := newScheme()
	tests := []struct {
		name   string
		cnSet  *v1alpha1.CNSet
		client client.Client
		expect func(g *WithT, cli client.Client, replicas int32, err error)
	}{
		{
			name: "comprehensive",
			cnSet: &v1alpha1.CNSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "test",
				},
				Status: v1alpha1.CNSetStatus{
					LabelSelector: "foo=bar",
				},
			},
			client: fake.KubeClientBuilder().WithScheme(s).WithObjects(
				&corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "unused",
						Namespace: "test",
						Labels: map[string]string{
							"foo":                    "bar",
							v1alpha1.CNPodPhaseLabel: v1alpha1.CNPodPhaseIdle,
						},
					},
				},
				&corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "used",
						Namespace: "test",
						Labels: map[string]string{
							"foo":                      "bar",
							v1alpha1.CNPodPhaseLabel:   v1alpha1.CNPodPhaseBound,
							v1alpha1.PodClaimedByLabel: "test-claim",
						},
					},
				},
				&corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "deleted",
						Namespace: "test",
						Labels: map[string]string{
							"foo":                    "bar",
							v1alpha1.CNPodPhaseLabel: v1alpha1.CNPodPhaseIdle,
						},
						DeletionTimestamp: utils.PtrTo(metav1.Now()),
						Finalizers:        []string{"mock"},
					},
				},
				&v1alpha1.CNClaim{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "test",
						Name:      "test-claim",
					},
					Status: v1alpha1.CNClaimStatus{
						Phase: v1alpha1.CNClaimPhaseBound,
					},
				},
			).WithStatusSubresource(&v1alpha1.CNClaim{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test",
					Name:      "test-claim",
				},
				Status: v1alpha1.CNClaimStatus{
					Phase: v1alpha1.CNClaimPhaseBound,
				},
			}).Build(),
			expect: func(g *WithT, cli client.Client, replicas int32, err error) {
				g.Expect(err).To(BeNil())
				g.Expect(replicas).To(Equal(int32(1)))
				cnClaim := &v1alpha1.CNClaim{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "test",
						Name:      "test-claim",
					},
				}
				g.Expect(cli.Get(context.TODO(), client.ObjectKeyFromObject(cnClaim), cnClaim)).To(Succeed())
				g.Expect(cnClaim.Status.Phase).To(Equal(v1alpha1.CNClaimPhaseOutdated))
				cnSet := &v1alpha1.CNSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test",
						Namespace: "test",
					},
				}
				g.Expect(cli.Get(context.TODO(), client.ObjectKeyFromObject(cnSet), cnSet)).To(Succeed())
				g.Expect(cnSet.Spec.PodsToDelete).To(ConsistOf("unused"))
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewGomegaWithT(t)
			mockCtrl := gomock.NewController(t)
			eventEmitter := fake.NewMockEventEmitter(mockCtrl)
			ctx := fake.NewContext(&v1alpha1.CNPool{}, tt.client, eventEmitter)
			r := &Actor{
				Logger: logr.Discard(),
			}
			res, err := r.syncLegacySet(ctx, tt.cnSet)
			tt.expect(g, tt.client, res, err)
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
