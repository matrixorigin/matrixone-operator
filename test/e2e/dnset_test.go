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

package e2e

import (
	"fmt"
	"strings"
	"time"

	recon "github.com/matrixorigin/controller-runtime/pkg/reconciler"
	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	createDNSetTimeout = 5 * time.Minute
)

var _ = Describe("MatrixOneCluster test", func() {
	It("Should reconcile the cluster properly", func() {
		By("Create dnset")
		l := &v1alpha1.LogSet{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: env.Namespace,
				Name:      "dn",
			},
			Spec: v1alpha1.LogSetSpec{
				PodSet: v1alpha1.PodSet{
					Replicas: 3,
					MainContainer: v1alpha1.MainContainer{
						Image: fmt.Sprintf("%s:%s", moImageRepo, moVersion),
					},
				},
				Volume: v1alpha1.Volume{
					Size: resource.MustParse("100Mi"),
				},
				SharedStorage: v1alpha1.SharedStorageProvider{
					FileSystem: &v1alpha1.FileSystemProvider{
						Path: "/test",
					},
				},
				StoreFailureTimeout: &metav1.Duration{Duration: 2 * time.Minute},
			},
		}
		d := &v1alpha1.DNSet{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: env.Namespace,
				Name:      "dn-" + rand.String(6),
			},
			Spec: v1alpha1.DNSetSpec{
				PodSet: v1alpha1.PodSet{
					Replicas: 1,
					MainContainer: v1alpha1.MainContainer{
						Image: fmt.Sprintf("%s:%s", moImageRepo, moVersion),
					},
				},
				CacheVolume: &v1alpha1.Volume{
					Size: resource.MustParse("100Mi"),
				},
			},
			Deps: v1alpha1.DNSetDeps{
				LogSetRef: v1alpha1.LogSetRef{
					LogSet: &v1alpha1.LogSet{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "dn",
							Namespace: env.Namespace,
						},
					},
				},
			},
		}
		Expect(kubeCli.Create(ctx, l)).To(Succeed())
		Expect(kubeCli.Create(ctx, d)).To(Succeed())

		Eventually(func() error {
			if err := kubeCli.Get(ctx, client.ObjectKeyFromObject(d), d); err != nil {
				logger.Errorw("error get dnset status", "dnset", d.Name, "error", err)
				return err
			}
			if !recon.IsReady(&d.Status.ConditionalStatus) {
				logger.Infow("wait dnset ready", "dnset", d.Name)
				return errWait
			}
			return nil
		}, createDNSetTimeout, pollInterval).Should(Succeed())

		By("Teardown dnset")
		Expect(kubeCli.Delete(ctx, d)).To(Succeed())
		Expect(kubeCli.Delete(ctx, l)).To(Succeed())
		Eventually(func() error {
			err := kubeCli.Get(ctx, client.ObjectKeyFromObject(d), d)
			if err == nil {
				logger.Infow("wait dnset teardown", "dnset", d.Name)
				return errWait
			}
			if !apierrors.IsNotFound(err) {
				logger.Errorw("unexpected error when get dnset", "dnset", d, "error", err)
				return err
			}
			podList := &corev1.PodList{}
			err = kubeCli.List(ctx, podList, client.InNamespace(d.Namespace))
			if err != nil {
				logger.Errorw("error list pods", "error", err)
				return err
			}
			for _, pod := range podList.Items {
				if strings.HasPrefix(pod.Name, d.Name) {
					logger.Infow("Pod that belongs to the dnset is not cleaned", "pod", pod.Name)
					return errWait
				}
			}
			return nil
		}, teardownClusterTimeout, pollInterval).Should(Succeed())
	})
})
