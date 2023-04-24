// Copyright 2023 Matrix Origin
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
	"github.com/matrixorigin/controller-runtime/pkg/util"
	kruisev1 "github.com/openkruise/kruise-api/apps/v1beta1"
	"k8s.io/apimachinery/pkg/api/equality"
	"strings"
	"time"

	recon "github.com/matrixorigin/controller-runtime/pkg/reconciler"
	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	"github.com/matrixorigin/matrixone-operator/pkg/controllers/common"
	e2eutil "github.com/matrixorigin/matrixone-operator/test/e2e/util"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/rand"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
				Name:      "log-" + rand.String(6),
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
			if err := kubeCli.Get(ctx, types.NamespacedName{Namespace: l.Namespace, Name: fmt.Sprintf("%s-log-3", l.Name)}, &corev1.Pod{}); err != nil {
				logger.Info("wait failover create new pod log-3")
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
			podList := &corev1.PodList{}
			err = kubeCli.List(ctx, podList, client.InNamespace(l.Namespace))
			if err != nil {
				logger.Errorw("error list pods", "error", err)
				return err
			}
			for _, pod := range podList.Items {
				if strings.HasPrefix(pod.Name, l.Name) {
					logger.Infow("Pod that belongs to the logset is not cleaned", "pod", pod.Name)
					return errWait
				}
			}
			return nil
		}, teardownClusterTimeout, pollInterval).Should(Succeed())
	})

	It("Should start logset service successfully when logset replicas is 1", func() {
		By("Create logset")
		l := &v1alpha1.LogSet{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: env.Namespace,
				Name:      "log-" + rand.String(6),
			},
			Spec: v1alpha1.LogSetSpec{
				PodSet: v1alpha1.PodSet{
					Replicas: 1,
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
			podList := &corev1.PodList{}
			err = kubeCli.List(ctx, podList, client.InNamespace(l.Namespace))
			if err != nil {
				logger.Errorw("error list pods", "error", err)
				return err
			}
			for _, pod := range podList.Items {
				if strings.HasPrefix(pod.Name, l.Name) {
					logger.Infow("Pod that belongs to the logset is not cleaned", "pod", pod.Name)
					return errWait
				}
			}
			return nil
		}, teardownClusterTimeout, pollInterval).Should(Succeed())
	})

	It("Should supply default fields after dry-run request", func() {
		// step 1: create a secret
		secret := e2eutil.NewSecretTpl(env.Namespace)
		Expect(kubeCli.Create(ctx, secret)).To(Succeed())

		// step 2: set secret volume as an overlay of log service, create log service
		// NOTE: we only set secret name in this volume, other fields (like defaultMode) are not set
		ls := e2eutil.NewLogSetTpl(env.Namespace, fmt.Sprintf("%s:%s", moImageRepo, moVersion))
		ls.Spec.Overlay = &v1alpha1.Overlay{}
		ls.Spec.Overlay.Volumes = []corev1.Volume{
			e2eutil.SecretVolume(secret.Name),
		}
		Expect(kubeCli.Create(ctx, ls)).To(Succeed())

		// step 3: fetch statefulset created by logset controller
		originSts := &kruisev1.StatefulSet{}
		Eventually(func() error {
			return kubeCli.Get(ctx, client.ObjectKey{Namespace: env.Namespace, Name: ls.Name + "-log"}, originSts)
		}, createLogSetTimeout, pollInterval).Should(Succeed())

		// statefulset volume should have default value, eg. DefaultMode: 0644
		stsVolumes := originSts.Spec.Template.Spec.Volumes
		secretVolume := util.FindFirst(stsVolumes, func(v corev1.Volume) bool {
			return v.Name == secret.Name
		})
		Expect(secretVolume != nil).Should(BeTrue())
		Expect(secretVolume.Secret != nil).Should(BeTrue())
		Expect(secretVolume.Secret.DefaultMode != nil).Should(BeTrue())

		// step 4: overlay the volume of deep copied statefulset, then dry run
		// NOTE: updated secret volume does not have any default values
		Eventually(func() error {
			if err := kubeCli.Get(ctx, client.ObjectKeyFromObject(originSts), originSts); err != nil {
				logger.Errorw("error get sts", "error", err)
				return err
			}
			stsCopy := originSts.DeepCopy()
			// overlay volume
			copiedVolume := stsCopy.Spec.Template.Spec.Volumes
			stsCopy.Spec.Template.Spec.Volumes = util.UpsertListByKey(copiedVolume, []corev1.Volume{e2eutil.SecretVolume(secret.Name)}, func(v corev1.Volume) string {
				return v.Name
			})
			if err := kubeCli.Update(ctx, stsCopy, client.DryRunAll); err != nil {
				logger.Errorw("error dry run update", "error", err)
				return err
			}

			// we set managedField to nil here because "managedFields" is not equal
			// kubeCli update operation will lead to volume field manager change, from "controller" to "kubeCli"
			// however in real running this is not an issue, but deep equal of whole statefulset is not recommended anyway
			stsCopy.ObjectMeta.ManagedFields = nil
			originSts.ObjectMeta.ManagedFields = nil
			if !equality.Semantic.DeepEqual(stsCopy, originSts) {
				return fmt.Errorf("unexpeted not equal after dry run update")
			}
			return nil
		}, time.Minute*5, time.Second*2).Should(Succeed())

		// tear down logset
		Expect(kubeCli.Delete(ctx, ls)).To(Succeed())
		Eventually(func() error {
			lsNS, lsName := ls.Namespace, ls.Name
			if err := kubeCli.Get(ctx, client.ObjectKey{Namespace: lsNS, Name: lsName}, ls); err == nil {
				logger.Infow("wait logset teardown", "logset", lsNS)
				return errWait
			} else if !apierrors.IsNotFound(err) {
				logger.Errorw("unexpected error when get logset", "logset", ls, "error", err)
				return err
			}
			podList := &corev1.PodList{}
			if err := kubeCli.List(ctx, podList, client.InNamespace(lsNS)); err != nil {
				logger.Errorw("error list pods", "error", err)
				return err
			}
			for _, pod := range podList.Items {
				if strings.HasPrefix(pod.Name, lsName) {
					logger.Infow("Pod that belongs to the logset is not cleaned", "pod", pod.Name)
					return errWait
				}
			}
			return nil
		}, teardownClusterTimeout, pollInterval).Should(Succeed())
	})
})
