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
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"

	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
)

var _ = Describe("MatrixOneCluster Webhook", func() {

	It("should accept MatrixOneCluster of old versions", func() {
		By("v0.6.x")
		// DO NOT mutate the following spec.
		// This spec is valid in mo-operator v0.6.0 and should always be accepted by
		// the webhook for backward compatibility.
		v06 := &v1alpha1.MatrixOneCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "mo-" + randomString(5),
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
				TN: &v1alpha1.DNSetSpec{
					PodSet: v1alpha1.PodSet{
						Replicas: 2,
					},
				},
				TP: &v1alpha1.CNSetSpec{
					PodSet: v1alpha1.PodSet{
						Replicas: 2,
					},
				},
				Version: "test",
			},
		}
		Expect(k8sClient.Create(context.TODO(), v06.DeepCopy())).To(Succeed())
		Expect(k8sClient.Create(context.TODO(), func() *v1alpha1.MatrixOneCluster {
			singleReplica := v06.DeepCopy()
			singleReplica.Spec.LogService.Replicas = 1
			singleReplica.Spec.TN.Replicas = 1
			singleReplica.Spec.TP.Replicas = 1
			singleReplica.Name = "mo-" + randomString(5)
			return singleReplica
		}())).To(Succeed())
	})

	It("should reject invalid MatrixOneCluster", func() {
		tpl := &v1alpha1.MatrixOneCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "mo-" + randomString(5),
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
				TN: &v1alpha1.DNSetSpec{
					PodSet: v1alpha1.PodSet{
						Replicas: 2,
					},
				},
				TP: &v1alpha1.CNSetSpec{
					PodSet: v1alpha1.PodSet{
						Replicas: 2,
					},
				},
				Version: "test",
			},
		}

		By("reject zero size volume")
		zeroVolume := tpl.DeepCopy()
		zeroVolume.Spec.LogService.Volume.Size = resource.MustParse("0")
		Expect(k8sClient.Create(context.TODO(), zeroVolume)).ToNot(Succeed())

		By("reject empty shared storage config")
		emptySharedStorage := tpl.DeepCopy()
		emptySharedStorage.Spec.LogService.SharedStorage.S3 = nil
		Expect(k8sClient.Create(context.TODO(), emptySharedStorage)).ToNot(Succeed())

		By("reject invalid replicas")
		invalidReplicas := tpl.DeepCopy()
		invalidReplicas.Spec.LogService.Replicas = 2
		invalidReplicas.Spec.LogService.InitialConfig.LogShardReplicas = pointer.Int(3)
		Expect(k8sClient.Create(context.TODO(), emptySharedStorage)).ToNot(Succeed())
	})

	It("should validate and mutate MatrixOneCluster", func() {
		cluster := &v1alpha1.MatrixOneCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "mo-" + randomString(5),
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
						S3: &v1alpha1.S3Provider{
							Path: "test/data",
						},
					},
				},
				TN: &v1alpha1.DNSetSpec{
					PodSet: v1alpha1.PodSet{
						Replicas: 2,
					},
				},
				TP: &v1alpha1.CNSetSpec{
					PodSet: v1alpha1.PodSet{
						Replicas: 2,
					},
				},
				Version: "test",
			},
		}
		Expect(k8sClient.Create(context.TODO(), cluster)).To(Succeed())

		By("defaults should be set on creation")
		Expect(cluster.Spec.TP.ServiceType).ToNot(BeEmpty(), "CN serviceType should have default")
		Expect(cluster.Spec.LogService.InitialConfig.LogShardReplicas).ToNot(BeNil(), "LogService initialConfig should have default")
		Expect(*cluster.Spec.LogService.InitialConfig.LogShardReplicas).To(Equal(3), "default logShardReplicas should follow the replicas of logservice")

		By("accept valid update")
		cluster.Spec.LogService.Replicas = 5
		cluster.Spec.AP = &v1alpha1.CNSetSpec{
			PodSet: v1alpha1.PodSet{
				Replicas: 2,
			},
		}
		Expect(k8sClient.Update(context.TODO(), cluster)).To(Succeed())

		By("reject invalid update")
		invalidReplica := cluster.DeepCopy()
		invalidReplica.Spec.LogService.Replicas = 2
		Expect(k8sClient.Update(context.TODO(), invalidReplica)).NotTo(Succeed(), "logservice replicas cannot be lower than HAKeeperReplicas")

		mutateInitialConfig := cluster.DeepCopy()
		mutateInitialConfig.Spec.LogService.InitialConfig.LogShardReplicas = pointer.Int(*mutateInitialConfig.Spec.LogService.InitialConfig.LogShardReplicas - 1)
		Expect(k8sClient.Update(context.TODO(), invalidReplica)).ToNot(Succeed(), "initialConfig should be immutable")
	})

	It("should validate and set defaults for CNGroups", func() {
		cluster := &v1alpha1.MatrixOneCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "mo-" + randomString(5),
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
						S3: &v1alpha1.S3Provider{
							Path: "test/data",
						},
					},
				},
				TN: &v1alpha1.DNSetSpec{
					PodSet: v1alpha1.PodSet{
						Replicas: 2,
					},
				},
				Version: "test",
				CNGroups: []v1alpha1.CNGroup{{
					Name: "test",
					CNSetSpec: v1alpha1.CNSetSpec{
						PodSet: v1alpha1.PodSet{
							Replicas: 3,
						},
					},
				}, {
					Name: "cache",
					CNSetSpec: v1alpha1.CNSetSpec{
						PodSet: v1alpha1.PodSet{
							Replicas: 3,
							MainContainer: v1alpha1.MainContainer{
								Resources: corev1.ResourceRequirements{
									Requests: map[corev1.ResourceName]resource.Quantity{
										corev1.ResourceMemory: resource.MustParse("10Gi"),
									},
								},
							},
						},
						ConfigThatChangeCNSpec: v1alpha1.ConfigThatChangeCNSpec{
							CacheVolume: &v1alpha1.Volume{
								Size: resource.MustParse("10Gi"),
							},
						},
					},
				}},
			},
		}
		setDefault := cluster.DeepCopy()
		Expect(k8sClient.Create(context.TODO(), setDefault)).To(Succeed())

		By("defaults should be set on creation")
		Expect(setDefault.Spec.CNGroups[0].ServiceType).ToNot(BeEmpty(), "CN serviceType should have default")
		Expect(setDefault.Spec.CNGroups[1].SharedStorageCache.DiskCacheSize).ToNot(BeNil(), "CN DiskCache should have default")
		Expect(setDefault.Spec.CNGroups[1].SharedStorageCache.MemoryCacheSize).ToNot(BeNil(), "CN MemoryCache should have default")

		for _, badName := range []string{"a b", "a/b", "a_b"} {
			b := cluster.DeepCopy()
			b.Spec.CNGroups[0].Name = badName
			Expect(k8sClient.Create(context.TODO(), setDefault)).NotTo(Succeed())
		}
	})

	It("should reject duplicate CNGroups", func() {
		cluster := &v1alpha1.MatrixOneCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "mo-" + randomString(5),
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
						S3: &v1alpha1.S3Provider{
							Path: "test/data",
						},
					},
				},
				TN: &v1alpha1.DNSetSpec{
					PodSet: v1alpha1.PodSet{
						Replicas: 1,
					},
				},
				Version: "test",
				TP: &v1alpha1.CNSetSpec{
					PodSet: v1alpha1.PodSet{
						Replicas: 3,
					},
				},
			},
		}
		dupTP := cluster.DeepCopy()
		dupTP.Spec.CNGroups = []v1alpha1.CNGroup{{
			Name: "tp",
			CNSetSpec: v1alpha1.CNSetSpec{
				PodSet: v1alpha1.PodSet{
					Replicas: 3,
				},
			},
		}}
		Expect(k8sClient.Create(context.TODO(), dupTP)).NotTo(Succeed())

		dupCNGroup := cluster.DeepCopy()
		dupCNGroup.Spec.CNGroups = []v1alpha1.CNGroup{{
			Name: "a",
			CNSetSpec: v1alpha1.CNSetSpec{
				PodSet: v1alpha1.PodSet{
					Replicas: 3,
				},
			},
		}, {
			Name: "a",
			CNSetSpec: v1alpha1.CNSetSpec{
				PodSet: v1alpha1.PodSet{
					Replicas: 3,
				},
			},
		}}
		Expect(k8sClient.Create(context.TODO(), dupCNGroup)).NotTo(Succeed())
	})

	It("should reject MatrixOneCluster with invalid name", func() {
		cluster := &v1alpha1.MatrixOneCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "mo-" + randomString(5),
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
						S3: &v1alpha1.S3Provider{
							Path: "test/data",
						},
					},
				},
				TN: &v1alpha1.DNSetSpec{
					PodSet: v1alpha1.PodSet{
						Replicas: 1,
					},
				},
				Version: "test",
				TP: &v1alpha1.CNSetSpec{
					PodSet: v1alpha1.PodSet{
						Replicas: 3,
					},
				},
			},
		}

		Expect(k8sClient.Create(context.TODO(), cluster.DeepCopy())).To(Succeed())

		By("reject name start with number")
		dpCluster1 := cluster.DeepCopy()
		dpCluster1.Name = "1" + dpCluster1.Name
		Expect(k8sClient.Create(context.TODO(), dpCluster1.DeepCopy())).NotTo(Succeed())

		By("reject name longer than " + fmt.Sprintf("%d", MatrixOneClusterNameMaxLength))
		dpCluster2 := cluster.DeepCopy()
		dpCluster2.Name = "mo-" + randomString(MatrixOneClusterNameMaxLength-2)
		Expect(k8sClient.Create(context.TODO(), dpCluster2.DeepCopy())).NotTo(Succeed())
	})
})
