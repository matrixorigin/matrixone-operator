// Copyright 2025 Matrix Origin
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

package cnclaim

import (
	"context"
	"math/rand"
	"testing"

	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	. "github.com/onsi/gomega"
)

func newFakeClient(objs ...client.Object) client.Client {
	scheme := runtime.NewScheme()
	_ = v1alpha1.SchemeBuilder.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)
	return fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()
}

// fakeKubeClient adapts client.Client to recon.KubeClient for testing.
type fakeKubeClient struct {
	client.Client
}

func (f *fakeKubeClient) Create(obj client.Object, opts ...client.CreateOption) error {
	return f.Client.Create(context.TODO(), obj, opts...)
}
func (f *fakeKubeClient) CreateOwned(obj client.Object, opts ...client.CreateOption) error {
	return f.Client.Create(context.TODO(), obj, opts...)
}
func (f *fakeKubeClient) Get(objKey client.ObjectKey, obj client.Object) error {
	return f.Client.Get(context.TODO(), objKey, obj)
}
func (f *fakeKubeClient) Update(obj client.Object, opts ...client.UpdateOption) error {
	return f.Client.Update(context.TODO(), obj, opts...)
}
func (f *fakeKubeClient) UpdateStatus(obj client.Object, opts ...client.SubResourceUpdateOption) error {
	return f.Client.Status().Update(context.TODO(), obj, opts...)
}
func (f *fakeKubeClient) Delete(obj client.Object, opts ...client.DeleteOption) error {
	return f.Client.Delete(context.TODO(), obj, opts...)
}
func (f *fakeKubeClient) List(objList client.ObjectList, opts ...client.ListOption) error {
	return f.Client.List(context.TODO(), objList, opts...)
}
func (f *fakeKubeClient) Patch(obj client.Object, mutateFn func() error, opts ...client.PatchOption) error {
	return mutateFn()
}
func (f *fakeKubeClient) Exist(objKey client.ObjectKey, kind client.Object) (bool, error) {
	err := f.Client.Get(context.TODO(), objKey, kind)
	if err != nil {
		return false, client.IgnoreNotFound(err)
	}
	return true, nil
}

func Test_podClaimedByOthers(t *testing.T) {
	tests := []struct {
		name         string
		claims       []client.Object
		podName      string
		excludeClaim string
		want         bool
	}{
		{
			name:         "no claims exist",
			claims:       nil,
			podName:      "pod-1",
			excludeClaim: "claim-a",
			want:         false,
		},
		{
			name: "pod only claimed by self",
			claims: []client.Object{
				&v1alpha1.CNClaim{
					ObjectMeta: metav1.ObjectMeta{Name: "claim-a", Namespace: "ns"},
					Spec:       v1alpha1.CNClaimSpec{ClaimPodRef: v1alpha1.ClaimPodRef{PodName: "pod-1"}},
				},
			},
			podName:      "pod-1",
			excludeClaim: "claim-a",
			want:         false,
		},
		{
			name: "pod claimed by another claim",
			claims: []client.Object{
				&v1alpha1.CNClaim{
					ObjectMeta: metav1.ObjectMeta{Name: "claim-a", Namespace: "ns"},
					Spec:       v1alpha1.CNClaimSpec{ClaimPodRef: v1alpha1.ClaimPodRef{PodName: "pod-1"}},
				},
				&v1alpha1.CNClaim{
					ObjectMeta: metav1.ObjectMeta{Name: "claim-b", Namespace: "ns"},
					Spec:       v1alpha1.CNClaimSpec{ClaimPodRef: v1alpha1.ClaimPodRef{PodName: "pod-1"}},
				},
			},
			podName:      "pod-1",
			excludeClaim: "claim-a",
			want:         true,
		},
		{
			name: "pod not claimed by anyone",
			claims: []client.Object{
				&v1alpha1.CNClaim{
					ObjectMeta: metav1.ObjectMeta{Name: "claim-a", Namespace: "ns"},
					Spec:       v1alpha1.CNClaimSpec{ClaimPodRef: v1alpha1.ClaimPodRef{PodName: "pod-2"}},
				},
			},
			podName:      "pod-1",
			excludeClaim: "claim-a",
			want:         false,
		},
		{
			name: "claim with empty podName is ignored",
			claims: []client.Object{
				&v1alpha1.CNClaim{
					ObjectMeta: metav1.ObjectMeta{Name: "claim-pending", Namespace: "ns"},
					Spec:       v1alpha1.CNClaimSpec{},
				},
			},
			podName:      "pod-1",
			excludeClaim: "claim-a",
			want:         false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewGomegaWithT(t)
			cli := newFakeClient(tt.claims...)
			got, err := podClaimedByOthers(&fakeKubeClient{cli}, "ns", tt.podName, tt.excludeClaim)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(got).To(Equal(tt.want))
		})
	}
}

func Test_sortCNByPriority(t *testing.T) {
	tests := []struct {
		name  string
		c     *v1alpha1.CNClaim
		pods  []corev1.Pod
		order []string
	}{{
		name: "basic",
		c: &v1alpha1.CNClaim{
			Spec: v1alpha1.CNClaimSpec{
				OwnerName: pointer.String("set1"),
			},
		},
		pods: []corev1.Pod{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "not-claimed-but-older",
					Labels: map[string]string{
						v1alpha1.CNPodPhaseLabel: v1alpha1.CNPodPhaseIdle,
					},
					CreationTimestamp: metav1.Unix(0, 0),
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "previously-claimed",
					Labels: map[string]string{
						v1alpha1.CNPodPhaseLabel:   v1alpha1.CNPodPhaseIdle,
						v1alpha1.PodLastOwnerLabel: "set1",
					},
					CreationTimestamp: metav1.Unix(10, 0),
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "previously-claimed-by-other-set",
					Labels: map[string]string{
						v1alpha1.CNPodPhaseLabel:   v1alpha1.CNPodPhaseIdle,
						v1alpha1.PodLastOwnerLabel: "set2",
					},
					CreationTimestamp: metav1.Unix(10, 0),
				},
			},
		},
		order: []string{
			"previously-claimed",
			"not-claimed-but-older",
			"previously-claimed-by-other-set",
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rand.Shuffle(len(tt.pods), func(i, j int) {
				tt.pods[i], tt.pods[j] = tt.pods[j], tt.pods[i]
			})
			sortCNByPriority(tt.c, tt.pods)
			g := NewGomegaWithT(t)
			var res []string
			for _, po := range tt.pods {
				res = append(res, po.Name)
			}
			g.Expect(res).To(Equal(tt.order))
		})
	}
}
