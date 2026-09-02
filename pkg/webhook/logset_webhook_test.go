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

package webhook

import (
	"context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"

	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
)

var _ = Describe("LogSet Webhook", func() {

	It("should accept LogSet of old versions", func() {
		// DO NOT mutate the following spec.
		// This spec is valid in mo-operator v0.6.0 and should always be accepted by
		// the webhook for backward compatibility.
		v06 := &v1alpha1.LogSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "ls-" + randomString(5),
				Namespace: "default",
			},
			Spec: v1alpha1.LogSetSpec{
				PodSet: v1alpha1.PodSet{
					Replicas: 3,
					MainContainer: v1alpha1.MainContainer{
						Image: "test:v1.2.3",
					},
				},
				Volume: v1alpha1.Volume{
					Size: resource.MustParse("10Gi"),
				},
				SharedStorage: v1alpha1.SharedStorageProvider{
					S3: &v1alpha1.S3Provider{Path: "test/data"},
				},
			},
		}
		Expect(k8sClient.Create(context.TODO(), v06)).To(Succeed())
	})

	It("should set defaults", func() {
		tpl := &v1alpha1.LogSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "ls-" + randomString(5),
				Namespace: "default",
			},
			Spec: v1alpha1.LogSetSpec{
				PodSet: v1alpha1.PodSet{
					Replicas: 3,
					MainContainer: v1alpha1.MainContainer{
						Image: "test:v1.2.3",
					},
				},
				Volume: v1alpha1.Volume{
					Size: resource.MustParse("10Gi"),
				},
				SharedStorage: v1alpha1.SharedStorageProvider{
					S3: &v1alpha1.S3Provider{Path: "test/data"},
				},
			},
		}
		testDefaultPVCRetainPolicy := tpl.DeepCopy()
		Expect(k8sClient.Create(context.TODO(), testDefaultPVCRetainPolicy)).To(Succeed())
		Expect(testDefaultPVCRetainPolicy.Spec.PVCRetentionPolicy).NotTo(BeNil())
		Expect(*testDefaultPVCRetainPolicy.Spec.PVCRetentionPolicy).To(Equal(v1alpha1.PVCRetentionPolicyDelete))
	})

	It("should reject sharedStorage.s3 updating", func() {
		ls := &v1alpha1.LogSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "ls-" + randomString(5),
				Namespace: "default",
			},
			Spec: v1alpha1.LogSetSpec{
				PodSet: v1alpha1.PodSet{
					Replicas: 3,
					MainContainer: v1alpha1.MainContainer{
						Image: "test:v1.2.3",
					},
				},
				Volume: v1alpha1.Volume{
					Size: resource.MustParse("10Gi"),
				},
				SharedStorage: v1alpha1.SharedStorageProvider{
					S3: &v1alpha1.S3Provider{
						Path: "test/data-old",
					},
				},
			},
		}

		Expect(k8sClient.Create(context.TODO(), ls)).To(Succeed())

		By("reject sharedStorage.s3 updating")
		modified := ls.DeepCopy()
		modified.Spec.SharedStorage.S3.Path = "test/data-new"
		Expect(k8sClient.Update(context.TODO(), modified)).NotTo(Succeed())
	})

	It("should allow scale to zero", func() {
		ls := &v1alpha1.LogSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "ls-" + randomString(5),
				Namespace: "default",
			},
			Spec: v1alpha1.LogSetSpec{
				InitialConfig: v1alpha1.InitialConfig{
					LogShardReplicas: pointer.Int(3),
				},
				PodSet: v1alpha1.PodSet{
					Replicas: 3,
					MainContainer: v1alpha1.MainContainer{
						Image: "test:v1.2.3",
					},
				},
				Volume: v1alpha1.Volume{
					Size: resource.MustParse("10Gi"),
				},
				SharedStorage: v1alpha1.SharedStorageProvider{
					S3: &v1alpha1.S3Provider{Path: "test/data"},
				},
			},
		}
		Expect(k8sClient.Create(context.TODO(), ls)).To(Succeed())
		ls.Spec.Replicas = 0
		Expect(k8sClient.Update(context.TODO(), ls)).To(Succeed())
	})
})
