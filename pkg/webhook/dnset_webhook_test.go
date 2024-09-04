// Copyright 2024 Matrix Origin
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

package webhook

import (
	"context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
)

var _ = Describe("DNSet Webhook", func() {

	It("should accept DNSet of old versions", func() {
		// DO NOT mutate the following spec.
		// This spec is valid in mo-operator v0.6.0 and should always be accepted by
		// the webhook for backward compatibility.
		v06 := &v1alpha1.DNSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "dn-" + randomString(5),
				Namespace: "default",
			},
			Spec: v1alpha1.DNSetSpec{
				PodSet: v1alpha1.PodSet{
					Replicas: 2,
					MainContainer: v1alpha1.MainContainer{
						Image: "test:v1.2.3",
					},
				},
			},
			Deps: v1alpha1.DNSetDeps{
				LogSetRef: v1alpha1.LogSetRef{
					LogSet: &v1alpha1.LogSet{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "test",
							Namespace: "default",
						},
					},
				},
			},
		}
		Expect(k8sClient.Create(context.TODO(), v06)).To(Succeed())
	})

	It("should set default cache size", func() {
		dn := &v1alpha1.DNSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "dn-" + randomString(5),
				Namespace: "default",
			},
			Spec: v1alpha1.DNSetSpec{
				PodSet: v1alpha1.PodSet{
					Replicas: 2,
					MainContainer: v1alpha1.MainContainer{
						Image: "test:v1.2.3",
					},
				},
				CacheVolume: &v1alpha1.Volume{
					Size: resource.MustParse("20Gi"),
				},
			},
			Deps: v1alpha1.DNSetDeps{
				LogSetRef: v1alpha1.LogSetRef{
					ExternalLogSet: &v1alpha1.ExternalLogSet{},
				},
			},
		}
		Expect(k8sClient.Create(context.TODO(), dn)).To(Succeed())
		expected := resource.MustParse("18Gi")
		Expect(dn.Spec.SharedStorageCache.DiskCacheSize.Value()).To(Equal(expected.Value()))
	})

	It("should reject duplicate [tn] and [dn] config", func() {
		dn := &v1alpha1.DNSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "dn-" + randomString(5),
				Namespace: "default",
			},
			Spec: v1alpha1.DNSetSpec{
				PodSet: v1alpha1.PodSet{
					Replicas: 2,
					MainContainer: v1alpha1.MainContainer{
						Image: "test:v1.2.3",
					},
					Config: v1alpha1.NewTomlConfig(map[string]interface{}{
						"tn": map[string]interface{}{
							"port-base": 1000,
						},
						"dn": map[string]interface{}{
							"port-base": 2000,
						},
					}),
				},
			},
			Deps: v1alpha1.DNSetDeps{
				LogSetRef: v1alpha1.LogSetRef{
					ExternalLogSet: &v1alpha1.ExternalLogSet{},
				},
			},
		}
		Expect(k8sClient.Create(context.TODO(), dn)).NotTo(Succeed())
	})
})
