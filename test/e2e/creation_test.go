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
package e2e

import (
	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	recon "github.com/matrixorigin/matrixone-operator/runtime/pkg/reconciler"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"time"
)

const (
	createClusterTimeout   = 5 * time.Minute
	teardownClusterTimeout = 5 * time.Minute
	pollInterval           = 15 * time.Second
)

var _ = Describe("Cluster creation test", func() {
	It("Should create cluster successfully", func() {
		By("Create cluster")
		mo := &v1alpha1.MatrixOneCluster{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: env.Namespace,
				Name:      "test",
			},
			Spec: v1alpha1.MatrixOneClusterSpec{
				TP: v1alpha1.CNSetBasic{
					PodSet: v1alpha1.PodSet{
						Replicas: 2,
					},
					CacheVolume: &v1alpha1.Volume{
						Size: resource.MustParse("100Mi"),
					},
				},
				DN: v1alpha1.DNSetBasic{
					PodSet: v1alpha1.PodSet{
						Replicas: 1,
					},
					CacheVolume: &v1alpha1.Volume{
						Size: resource.MustParse("100Mi"),
					},
				},
				LogService: v1alpha1.LogSetBasic{
					PodSet: v1alpha1.PodSet{
						Replicas: 3,
					},
					Volume: v1alpha1.Volume{
						Size: resource.MustParse("100Mi"),
					},
					SharedStorage: v1alpha1.SharedStorageProvider{
						S3: &v1alpha1.S3Provider{
							Path: "test/bucket",
						},
					},
					InitialConfig: v1alpha1.InitialConfig{},
				},
				Version:         moVersion,
				ImageRepository: moImageRepo,
			},
		}
		Expect(kubeCli.Create(ctx, mo)).To(Succeed())

		Eventually(func() error {
			err := kubeCli.Get(ctx, client.ObjectKeyFromObject(mo), mo)
			if err != nil {
				logger.Errorw("error get mo cluster status", "cluster", mo.Name, "error", err)
				return err
			}
			if mo.Status.TP == nil || !recon.IsReady(mo.Status.TP) {
				logger.Infow("wait mo cluster ready", "cluster", mo.Name)
				return errWait
			}
			return nil
		}, createClusterTimeout, pollInterval).Should(Succeed())

		By("Teardown cluster")
		Expect(kubeCli.Delete(ctx, mo)).To(Succeed())
		Eventually(func() error {
			err := kubeCli.Get(ctx, client.ObjectKeyFromObject(mo), mo)
			if err == nil {
				logger.Infow("wait mo cluster teardown", "cluster", mo.Name)
				return errWait
			}
			if !apierrors.IsNotFound(err) {
				logger.Errorw("unexpected error when get mo cluster", "cluster", mo.Name, "error", err)
				return err
			}
			return nil
		}, teardownClusterTimeout, pollInterval).Should(Succeed())
	})
})
