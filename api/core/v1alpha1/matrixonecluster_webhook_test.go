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
package v1alpha1

import (
	"context"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
)

var _ = Describe("MatrixOneCluster Webhook", func() {

	It("should accept MatrixOneCluster of old versions", func() {
		By("v0.6.x")
		// DO NOT mutate the following spec.
		// This spec is valid in mo-operator v0.6.0 and should always be accepted by
		// the webhook for backward compatibility.
		v06 := &MatrixOneCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "mo-" + randomString(5),
				Namespace: "default",
			},
			Spec: MatrixOneClusterSpec{
				LogService: LogSetBasic{
					PodSet: PodSet{
						Replicas: 3,
					},
					Volume: Volume{
						Size: resource.MustParse("10Gi"),
					},
					SharedStorage: SharedStorageProvider{
						S3: &S3Provider{Path: "test/data"},
					},
				},
				DN: DNSetBasic{
					PodSet: PodSet{
						Replicas: 2,
					},
				},
				TP: CNSetBasic{
					PodSet: PodSet{
						Replicas: 2,
					},
				},
				Version: "test",
			},
		}
		Expect(k8sClient.Create(context.TODO(), v06.DeepCopy())).To(Succeed())
		Expect(k8sClient.Create(context.TODO(), func() *MatrixOneCluster {
			singleReplica := v06.DeepCopy()
			singleReplica.Spec.LogService.Replicas = 1
			singleReplica.Spec.DN.Replicas = 1
			singleReplica.Spec.TP.Replicas = 1
			singleReplica.Name = "mo-" + randomString(5)
			return singleReplica
		}())).To(Succeed())
	})

	It("should reject invalid MatrixOneCluster", func() {
		tpl := &MatrixOneCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "mo-" + randomString(5),
				Namespace: "default",
			},
			Spec: MatrixOneClusterSpec{
				LogService: LogSetBasic{
					PodSet: PodSet{
						Replicas: 3,
					},
					Volume: Volume{
						Size: resource.MustParse("10Gi"),
					},
					SharedStorage: SharedStorageProvider{
						S3: &S3Provider{Path: "test/data"},
					},
				},
				DN: DNSetBasic{
					PodSet: PodSet{
						Replicas: 2,
					},
				},
				TP: CNSetBasic{
					PodSet: PodSet{
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
		invalidReplicas.Spec.LogService.InitialConfig.HAKeeperReplicas = pointer.Int(3)
		Expect(k8sClient.Create(context.TODO(), emptySharedStorage)).ToNot(Succeed())
	})

	It("should validate and mutate MatrixOneCluster", func() {
		cluster := &MatrixOneCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "mo-" + randomString(5),
				Namespace: "default",
			},
			Spec: MatrixOneClusterSpec{
				LogService: LogSetBasic{
					PodSet: PodSet{
						Replicas: 3,
					},
					Volume: Volume{
						Size: resource.MustParse("10Gi"),
					},
					SharedStorage: SharedStorageProvider{
						S3: &S3Provider{
							Path: "test/data",
						},
					},
				},
				DN: DNSetBasic{
					PodSet: PodSet{
						Replicas: 2,
					},
				},
				TP: CNSetBasic{
					PodSet: PodSet{
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
		Expect(*cluster.Spec.LogService.InitialConfig.HAKeeperReplicas).To(Equal(3), "default haKeeperReplicas should follow the replicas of logservice")

		By("accept valid update")
		cluster.Spec.LogService.Replicas = 5
		cluster.Spec.AP = &CNSetBasic{
			PodSet: PodSet{
				Replicas: 2,
			},
		}
		Expect(k8sClient.Update(context.TODO(), cluster)).To(Succeed())

		By("reject invalid update")
		invalidReplica := cluster.DeepCopy()
		invalidReplica.Spec.LogService.Replicas = 2
		Expect(k8sClient.Update(context.TODO(), invalidReplica)).ToNot(Succeed(), "logservice replicas cannot be lower than HAKeeperReplicas")

		mutateInitialConfig := cluster.DeepCopy()
		mutateInitialConfig.Spec.LogService.InitialConfig.HAKeeperReplicas = pointer.Int(*mutateInitialConfig.Spec.LogService.InitialConfig.HAKeeperReplicas - 1)
		Expect(k8sClient.Update(context.TODO(), invalidReplica)).ToNot(Succeed(), "initialConfig should be immutable")
	})
})
