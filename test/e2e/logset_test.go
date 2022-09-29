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
	"fmt"
	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	"github.com/matrixorigin/matrixone-operator/pkg/controllers/common"
	recon "github.com/matrixorigin/matrixone-operator/runtime/pkg/reconciler"
	e2eutil "github.com/matrixorigin/matrixone-operator/test/e2e/util"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"time"
)

const (
	createLogSetTimeout = 5 * time.Minute
	failoverTimeout     = 10 * time.Minute
)

const pod0WillCrash = `
echo "chaos injected, container will fail if it has pod ordinal 0"
ORDINAL=${POD_NAME##*-}
if [ "${ORDINAL}" -eq "0" ]; then
	echo "I am the victim, exit" && exit 1
fi
/bin/sh /etc/logservice/start.sh
`

var _ = Describe("MatrixOneCluster test", func() {
	It("Should reconcile the cluster properly", func() {
		By("Create logset")
		l := &v1alpha1.LogSet{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: env.Namespace,
				Name:      "log",
			},
			Spec: v1alpha1.LogSetSpec{
				LogSetBasic: v1alpha1.LogSetBasic{
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
						S3: &v1alpha1.S3Provider{
							Path: "mo-e2e/logset",
						},
					},
					StoreFailureTimeout: &metav1.Duration{Duration: 2 * time.Minute},
				},
			},
		}
		Expect(kubeCli.Create(ctx, l)).To(Succeed())

		Eventually(func() error {
			if err := kubeCli.Get(ctx, client.ObjectKeyFromObject(l), l); err != nil {
				logger.Errorw("error get logset status", "logset", l.Name, "error", err)
				return err
			}
			if !recon.IsReady(&l.Status.ConditionalStatus) {
				logger.Infow("wait logset ready", "logset", l.Name)
				return errWait
			}
			return nil
		}, createLogSetTimeout, pollInterval).Should(Succeed())

		By("Logset failover")
		Expect(kubeCli.Get(ctx, client.ObjectKeyFromObject(l), l)).To(Succeed())
		l.Spec.Overlay = &v1alpha1.Overlay{
			MainContainerOverlay: v1alpha1.MainContainerOverlay{
				Command: []string{
					"/bin/sh",
					"-c",
					pod0WillCrash,
				},
			},
		}
		Expect(kubeCli.Update(ctx, l)).To(Succeed())
		Eventually(func() error {
			if err := kubeCli.Get(ctx, types.NamespacedName{Namespace: l.Namespace, Name: "log-log-3"}, &corev1.Pod{}); err != nil {
				logger.Info("wait failover create new pod log-log-3")
				return errWait
			}
			return nil
		}, failoverTimeout, pollInterval).Should(Succeed())

		By("Logset scale")
		Expect(e2eutil.Patch(ctx, kubeCli, l, func() error {
			l.Spec.Replicas = 4
			return nil
		})).To(Succeed())
		Eventually(func() error {
			podList := &corev1.PodList{}
			if err := kubeCli.List(ctx, podList, client.MatchingLabels(map[string]string{common.InstanceLabelKey: l.Name})); err != nil {
				logger.Errorw("error list pods", "logset", l.Name, "error", err)
				return err
			}
			if len(podList.Items) >= 4 {
				return nil
			}
			logger.Infow("wait enough pods running", "log pods count", len(podList.Items), "expect", l.Spec.Replicas)
			return errWait
		}, createClusterTimeout, pollInterval).Should(Succeed())

		By("Teardown logset")
		Expect(kubeCli.Delete(ctx, l)).To(Succeed())
		Eventually(func() error {
			err := kubeCli.Get(ctx, client.ObjectKeyFromObject(l), l)
			if err == nil {
				logger.Infow("wait logset teardown", "logset", l.Name)
				return errWait
			}
			if !apierrors.IsNotFound(err) {
				logger.Errorw("unexpected error when get logset", "logset", l, "error", err)
				return err
			}
			return nil
		}, teardownClusterTimeout, pollInterval).Should(Succeed())
	})
})
