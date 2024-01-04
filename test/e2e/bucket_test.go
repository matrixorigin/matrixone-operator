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
	"context"
	"fmt"
	recon "github.com/matrixorigin/controller-runtime/pkg/reconciler"
	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	"github.com/matrixorigin/matrixone-operator/test/e2e/e2eminio"
	e2eutil "github.com/matrixorigin/matrixone-operator/test/e2e/util"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strings"
	"time"
)

const (
	waitBucketStatusTimeout = time.Minute * 5
)

var _ = Describe("Matrix BucketClaim test", func() {

	It("Should bucket been released use retain policy", func() {
		By("create logset cluster with minio provider")
		minioSecret := e2eutil.MinioSecret(env.Namespace)
		Expect(kubeCli.Create(ctx, minioSecret)).To(Succeed())

		minioProvider := e2eutil.MinioShareStorage(minioSecret.Name)
		policyRetain := v1alpha1.PVCRetentionPolicyRetain
		minioProvider.S3.S3RetentionPolicy = &policyRetain
		ls := e2eutil.NewLogSetTpl(env.Namespace, fmt.Sprintf("%s:%s", moImageRepo, moVersion))
		ls.Spec.SharedStorage = minioProvider
		Expect(kubeCli.Create(ctx, ls)).To(Succeed())

		var bucket *v1alpha1.BucketClaim
		var err error
		Eventually(func() error {
			bucket, err = v1alpha1.ClaimedBucket(kubeCli, minioProvider.S3)
			if err != nil || bucket == nil {
				return fmt.Errorf("wait bucket creating for logset %v, %v", client.ObjectKeyFromObject(ls), err)
			}
			expectedStatus := v1alpha1.BucketClaimStatus{
				BindTo: v1alpha1.BucketBindToMark(ls.ObjectMeta),
				State:  v1alpha1.StatusInUse,
			}
			if !reflect.DeepEqual(expectedStatus, bucket.Status) {
				return fmt.Errorf("bucket status is not inuse, current %v", bucket.Status)
			}
			return nil
		}, waitBucketStatusTimeout, time.Second*2).Should(Succeed())

		By("tear down logset cluster")
		Expect(kubeCli.Delete(ctx, ls)).To(Succeed())
		Eventually(func() error {
			return waitLogSetDeleted(ls)
		}, teardownClusterTimeout, pollInterval).Should(Succeed())

		By("bucket should in released state")
		Eventually(func() error {
			if err := kubeCli.Get(ctx, client.ObjectKeyFromObject(bucket), bucket); err != nil {
				return err
			}
			if bucket.DeletionTimestamp != nil {
				return fmt.Errorf("bucket should not be deleted in default retain policy")
			}
			expectedStatus := v1alpha1.BucketClaimStatus{
				BindTo: "",
				State:  v1alpha1.StatusReleased,
			}
			if !reflect.DeepEqual(expectedStatus, bucket.Status) {
				return fmt.Errorf("bucket status is not released, current %v", bucket.Status)
			}
			return nil
		}, waitBucketStatusTimeout, time.Second*2).Should(Succeed())

		By("reclaim bucket")
		Expect(reclaimBucket(bucket)).Should(BeNil())
		Eventually(func() error {
			err := kubeCli.Get(ctx, client.ObjectKeyFromObject(bucket), bucket)
			if apierrors.IsNotFound(err) {
				return nil
			}
			return fmt.Errorf("bucket should be deleted")
		}, waitBucketStatusTimeout, time.Second*2).Should(Succeed())
	})

	It("Should bucket been deleted use delete retain policy", func() {
		By("create logset cluster with minio provider")
		minioSecret := e2eutil.MinioSecret(env.Namespace)
		Expect(kubeCli.Create(ctx, minioSecret)).To(Succeed())

		minioProvider := e2eutil.MinioShareStorage(minioSecret.Name)
		policyDelete := v1alpha1.PVCRetentionPolicyDelete
		minioProvider.S3.S3RetentionPolicy = &policyDelete
		ls := e2eutil.NewLogSetTpl(env.Namespace, fmt.Sprintf("%s:%s", moImageRepo, moVersion))
		ls.Spec.SharedStorage = minioProvider
		Expect(kubeCli.Create(ctx, ls)).To(Succeed())

		var bucket *v1alpha1.BucketClaim
		var err error
		Eventually(func() error {
			bucket, err = v1alpha1.ClaimedBucket(kubeCli, minioProvider.S3)
			if err != nil || bucket == nil {
				return fmt.Errorf("wait bucket creating for logset %v, %v", client.ObjectKeyFromObject(ls), err)
			}
			expectedStatus := v1alpha1.BucketClaimStatus{
				BindTo: v1alpha1.BucketBindToMark(ls.ObjectMeta),
				State:  v1alpha1.StatusInUse,
			}
			if !reflect.DeepEqual(expectedStatus, bucket.Status) {
				return fmt.Errorf("bucket status is not inuse, current %v", bucket.Status)
			}
			return nil
		}, waitBucketStatusTimeout, time.Second*2).Should(Succeed())

		By("minio bucket path should exist")
		object, err := e2eminio.PutObject(minioProvider.S3.Path)
		Expect(err).Should(BeNil())
		exist, err := e2eminio.IsObjectExist(object)
		Expect(err).Should(BeNil())
		Expect(exist).Should(BeTrue())

		By("wait logset available")
		Eventually(func() error {
			if err := kubeCli.Get(context.TODO(), client.ObjectKeyFromObject(ls), ls); err != nil {
				return err
			}
			if len(ls.Status.AvailableStores) > 0 {
				return nil
			}
			return fmt.Errorf("wait logset pod in running state")
		}, createLogSetTimeout, pollInterval).Should(Succeed())

		By("tear down logset cluster")
		Expect(kubeCli.Delete(ctx, ls)).To(Succeed())
		Eventually(func() error {
			return waitLogSetDeleted(ls)
		}, teardownClusterTimeout, pollInterval).Should(Succeed())

		By("bucket should been deleted")
		Eventually(func() error {
			err := kubeCli.Get(ctx, client.ObjectKeyFromObject(bucket), bucket)
			if apierrors.IsNotFound(err) {
				return nil
			}
			return fmt.Errorf("bucket should be deleted")
		}, waitBucketStatusTimeout, time.Second*2).Should(Succeed())

		By("minio bucket path should not exist")
		exist, err = e2eminio.IsObjectExist(object)
		Expect(err).Should(BeNil())
		Expect(exist).Should(BeFalse())
	})

	It("Should not delete a bucket which is in use", func() {
		By("create logset cluster with minio provider")
		minioSecret := e2eutil.MinioSecret(env.Namespace)
		Expect(kubeCli.Create(ctx, minioSecret)).To(Succeed())

		minioProvider := e2eutil.MinioShareStorage(minioSecret.Name)
		deletePolicy := v1alpha1.PVCRetentionPolicyDelete
		minioProvider.S3.S3RetentionPolicy = &deletePolicy
		ls := e2eutil.NewLogSetTpl(env.Namespace, fmt.Sprintf("%s:%s", moImageRepo, moVersion))
		ls.Spec.SharedStorage = minioProvider
		Expect(kubeCli.Create(ctx, ls)).To(Succeed())

		var bucket *v1alpha1.BucketClaim
		var err error
		Eventually(func() error {
			bucket, err = v1alpha1.ClaimedBucket(kubeCli, minioProvider.S3)
			if err != nil || bucket == nil {
				return fmt.Errorf("wait bucket creating for logset %v, %v", client.ObjectKeyFromObject(ls), err)
			}
			expectedStatus := v1alpha1.BucketClaimStatus{
				BindTo: v1alpha1.BucketBindToMark(ls.ObjectMeta),
				State:  v1alpha1.StatusInUse,
			}
			if !reflect.DeepEqual(expectedStatus, bucket.Status) {
				return fmt.Errorf("bucket status is not inuse, current %v", bucket.Status)
			}
			return nil
		}, waitBucketStatusTimeout, time.Second*2).Should(Succeed())

		By("bucket should contain inuse condition")
		Expect(kubeCli.Delete(ctx, bucket)).To(Succeed())
		Eventually(func() error {
			bucket, err = v1alpha1.ClaimedBucket(kubeCli, minioProvider.S3)
			if err != nil || bucket == nil {
				return fmt.Errorf("wait bucket creating for logset %v, %v", client.ObjectKeyFromObject(ls), err)
			}
			if len(bucket.Status.Conditions) == 0 {
				return fmt.Errorf("condition should not be empty")
			}
			failCondition := bucket.Status.Conditions[0]
			if failCondition.Type == "recyclable" &&
				failCondition.Status == metav1.ConditionFalse &&
				failCondition.Reason == "InUse" {
				return nil
			}
			return fmt.Errorf("wait inuse condition, current: %v", bucket.Status.Conditions)
		}, waitBucketStatusTimeout, time.Second*2).Should(Succeed())

		By("tear down logset cluster")
		Expect(kubeCli.Delete(ctx, ls)).To(Succeed())
		Eventually(func() error {
			return waitLogSetDeleted(ls)
		}, teardownClusterTimeout, pollInterval).Should(Succeed())
	})

	It("Should reclaim job success when bucket not exist", func() {
		By("create logset cluster with minio provider")
		minioSecret := e2eutil.MinioSecret(env.Namespace)
		Expect(kubeCli.Create(ctx, minioSecret)).To(Succeed())

		minioProvider := e2eutil.MinioShareStorage(minioSecret.Name)
		policyRetain := v1alpha1.PVCRetentionPolicyRetain
		minioProvider.S3.S3RetentionPolicy = &policyRetain
		ls := e2eutil.NewLogSetTpl(env.Namespace, fmt.Sprintf("%s:%s", moImageRepo, moVersion))
		ls.Spec.SharedStorage = minioProvider
		Expect(kubeCli.Create(ctx, ls)).To(Succeed())

		var bucket *v1alpha1.BucketClaim
		var err error
		Eventually(func() error {
			bucket, err = v1alpha1.ClaimedBucket(kubeCli, minioProvider.S3)
			if err != nil || bucket == nil {
				return fmt.Errorf("wait bucket creating for logset %v, %v", client.ObjectKeyFromObject(ls), err)
			}
			expectedStatus := v1alpha1.BucketClaimStatus{
				BindTo: v1alpha1.BucketBindToMark(ls.ObjectMeta),
				State:  v1alpha1.StatusInUse,
			}
			if !reflect.DeepEqual(expectedStatus, bucket.Status) {
				return fmt.Errorf("bucket status is not inuse, current %v", bucket.Status)
			}
			return nil
		}, waitBucketStatusTimeout, time.Second*2).Should(Succeed())

		By("simulate bucket consumer to put object")
		_, err = e2eminio.PutObject(minioProvider.S3.Path)
		Expect(err).Should(BeNil())

		By("tear down logset cluster")
		Expect(kubeCli.Delete(ctx, ls)).To(Succeed())
		Eventually(func() error {
			return waitLogSetDeleted(ls)
		}, teardownClusterTimeout, pollInterval).Should(Succeed())

		By("bucket should in released state")
		Eventually(func() error {
			if err := kubeCli.Get(ctx, client.ObjectKeyFromObject(bucket), bucket); err != nil {
				return err
			}
			if bucket.DeletionTimestamp != nil {
				return fmt.Errorf("bucket should not be deleted in default retain policy")
			}
			expectedStatus := v1alpha1.BucketClaimStatus{
				BindTo: "",
				State:  v1alpha1.StatusReleased,
			}
			if !reflect.DeepEqual(expectedStatus, bucket.Status) {
				return fmt.Errorf("bucket status is not released, current %v", bucket.Status)
			}
			return nil
		}, waitBucketStatusTimeout, time.Second*2).Should(Succeed())

		By("update bucket to reference a non-exist bucket")
		bucket.Spec.S3.Path = "non-exist-bucket/path1"
		Expect(kubeCli.Update(ctx, bucket)).To(Succeed())

		By("delete bucket, trigger reclaim job")
		Expect(kubeCli.Delete(ctx, bucket)).To(Succeed())

		By("wait bucket been deleted, bucket will been deleted only when job exits successfully")
		Eventually(func() error {
			err := kubeCli.Get(ctx, client.ObjectKeyFromObject(bucket), bucket)
			if apierrors.IsNotFound(err) {
				return nil
			}
			return fmt.Errorf("bucket should be deleted")
		}, waitBucketStatusTimeout, time.Second*2).Should(Succeed())
	})

	It("Should reclaim job success when prefix not exist", func() {
		By("create logset cluster with minio provider")
		minioSecret := e2eutil.MinioSecret(env.Namespace)
		Expect(kubeCli.Create(ctx, minioSecret)).To(Succeed())

		minioProvider := e2eutil.MinioShareStorage(minioSecret.Name)
		policyRetain := v1alpha1.PVCRetentionPolicyRetain
		minioProvider.S3.S3RetentionPolicy = &policyRetain
		ls := e2eutil.NewLogSetTpl(env.Namespace, fmt.Sprintf("%s:%s", moImageRepo, moVersion))
		ls.Spec.SharedStorage = minioProvider
		Expect(kubeCli.Create(ctx, ls)).To(Succeed())

		var bucket *v1alpha1.BucketClaim
		var err error
		Eventually(func() error {
			bucket, err = v1alpha1.ClaimedBucket(kubeCli, minioProvider.S3)
			if err != nil || bucket == nil {
				return fmt.Errorf("wait bucket creating for logset %v, %v", client.ObjectKeyFromObject(ls), err)
			}
			expectedStatus := v1alpha1.BucketClaimStatus{
				BindTo: v1alpha1.BucketBindToMark(ls.ObjectMeta),
				State:  v1alpha1.StatusInUse,
			}
			if !reflect.DeepEqual(expectedStatus, bucket.Status) {
				return fmt.Errorf("bucket status is not inuse, current %v", bucket.Status)
			}
			return nil
		}, waitBucketStatusTimeout, time.Second*2).Should(Succeed())

		By("simulate bucket consumer to put object")
		_, err = e2eminio.PutObject(minioProvider.S3.Path)
		Expect(err).Should(BeNil())

		By("tear down logset cluster")
		Expect(kubeCli.Delete(ctx, ls)).To(Succeed())
		Eventually(func() error {
			return waitLogSetDeleted(ls)
		}, teardownClusterTimeout, pollInterval).Should(Succeed())

		By("bucket should in released state")
		Eventually(func() error {
			if err := kubeCli.Get(ctx, client.ObjectKeyFromObject(bucket), bucket); err != nil {
				return err
			}
			if bucket.DeletionTimestamp != nil {
				return fmt.Errorf("bucket should not be deleted in default retain policy")
			}
			expectedStatus := v1alpha1.BucketClaimStatus{
				BindTo: "",
				State:  v1alpha1.StatusReleased,
			}
			if !reflect.DeepEqual(expectedStatus, bucket.Status) {
				return fmt.Errorf("bucket status is not released, current %v", bucket.Status)
			}
			return nil
		}, waitBucketStatusTimeout, time.Second*2).Should(Succeed())

		By("update bucket to reference a non-exist path")
		bucket.Spec.S3.Path = "minio-bucket/a-non-exist-path"
		Expect(kubeCli.Update(ctx, bucket)).To(Succeed())

		By("delete bucket, trigger reclaim job")
		Expect(kubeCli.Delete(ctx, bucket)).To(Succeed())

		By("wait bucket been deleted, bucket will been deleted only when job exits successfully")
		Eventually(func() error {
			err := kubeCli.Get(ctx, client.ObjectKeyFromObject(bucket), bucket)
			if apierrors.IsNotFound(err) {
				return nil
			}
			return fmt.Errorf("bucket should be deleted")
		}, waitBucketStatusTimeout, time.Second*2).Should(Succeed())
	})

	It("Should failure when creating logset which want bind to an already bound bucket", func() {
		By("create logset cluster with minio provider")
		minioSecret := e2eutil.MinioSecret(env.Namespace)
		Expect(kubeCli.Create(ctx, minioSecret)).To(Succeed())

		minioProvider := e2eutil.MinioShareStorage(minioSecret.Name)
		ls := e2eutil.NewLogSetTpl(env.Namespace, fmt.Sprintf("%s:%s", moImageRepo, moVersion))
		ls.Spec.SharedStorage = minioProvider
		Expect(kubeCli.Create(ctx, ls)).To(Succeed())

		var bucket *v1alpha1.BucketClaim
		var err error
		Eventually(func() error {
			bucket, err = v1alpha1.ClaimedBucket(kubeCli, minioProvider.S3)
			if err != nil || bucket == nil {
				return fmt.Errorf("wait bucket creating for logset %v, %v", client.ObjectKeyFromObject(ls), err)
			}
			expectedStatus := v1alpha1.BucketClaimStatus{
				BindTo: v1alpha1.BucketBindToMark(ls.ObjectMeta),
				State:  v1alpha1.StatusInUse,
			}
			if !reflect.DeepEqual(expectedStatus, bucket.Status) {
				return fmt.Errorf("bucket status is not inuse, current %v", bucket.Status)
			}
			return nil
		}, waitBucketStatusTimeout, time.Second*2).Should(Succeed())

		By("create another logset cluster, claim same bucket as above")
		lsShouldFailed := e2eutil.NewLogSetTpl(env.Namespace, fmt.Sprintf("%s:%s", moImageRepo, moVersion))
		lsShouldFailed.Spec.SharedStorage = minioProvider
		err = kubeCli.Create(ctx, lsShouldFailed)
		Expect(err != nil).Should(BeTrue())
		Expect(strings.Contains(err.Error(), "is invalid")).Should(BeTrue())
		Expect(strings.Contains(err.Error(), "spec.sharedStorage.s3")).Should(BeTrue())
		Expect(strings.Contains(err.Error(), "already bind to")).Should(BeTrue())

		By("tear down logset cluster")
		Expect(kubeCli.Delete(ctx, ls)).To(Succeed())
		Eventually(func() error {
			return waitLogSetDeleted(ls)
		}, teardownClusterTimeout, pollInterval).Should(Succeed())
	})

	It("Should create mo cluster fail if its included logset want to bind an already bound bucket", func() {
		By("create mo cluster in new namespace")
		newNS := e2eutil.NewNamespaceTpl()
		Expect(kubeCli.Create(ctx, newNS)).To(Succeed())

		By("create one mo cluster with minio s3 provider")
		minioSecret := e2eutil.MinioSecret(newNS.Name)
		Expect(kubeCli.Create(ctx, minioSecret)).To(Succeed())

		minioProvider := e2eutil.MinioShareStorage(minioSecret.Name)
		mo := e2eutil.NewMoTpl(newNS.Name, moVersion, moImageRepo)
		mo.Spec.LogService.SharedStorage = minioProvider
		Expect(kubeCli.Create(ctx, mo)).To(Succeed())

		Eventually(func() error {
			if err := kubeCli.Get(ctx, client.ObjectKeyFromObject(mo), mo); err != nil {
				logger.Errorw("error get mo cluster status", "cluster", mo.Name, "error", err)
				return err
			}
			if !recon.IsReady(mo) {
				logger.Infow("wait mo cluster ready", "cluster", mo.Name)
				return errWait
			}
			return nil
		}, createClusterTimeout, pollInterval).Should(Succeed())

		By("create another mo cluster with same minio s3 provider")
		shouldFailMO := e2eutil.NewMoTpl(newNS.Name, moVersion, moImageRepo)
		shouldFailMO.Spec.LogService.SharedStorage = minioProvider
		err := kubeCli.Create(ctx, shouldFailMO)
		Expect(err != nil).Should(BeTrue())
		Expect(strings.Contains(err.Error(), "is invalid")).Should(BeTrue())
		Expect(strings.Contains(err.Error(), "spec.sharedStorage.s3")).Should(BeTrue())
		Expect(strings.Contains(err.Error(), "already bind to")).Should(BeTrue())

		By("Teardown cluster")
		Expect(kubeCli.Delete(ctx, mo)).To(Succeed())
		Eventually(func() error {
			return waitClusterDeleted(mo)
		}, teardownClusterTimeout, pollInterval).Should(Succeed(), "cluster should be teardown")

		Expect(kubeCli.Delete(ctx, newNS)).Should(Succeed())
	})

	It("Should failure when creating logset in another namespace which want bind to an already bound bucket", func() {
		By("create logset cluster with minio provider")
		minioSecret := e2eutil.MinioSecret(env.Namespace)
		Expect(kubeCli.Create(ctx, minioSecret)).To(Succeed())

		minioProvider := e2eutil.MinioShareStorage(minioSecret.Name)
		ls := e2eutil.NewLogSetTpl(env.Namespace, fmt.Sprintf("%s:%s", moImageRepo, moVersion))
		ls.Spec.SharedStorage = minioProvider
		Expect(kubeCli.Create(ctx, ls)).To(Succeed())

		var bucket *v1alpha1.BucketClaim
		var err error
		Eventually(func() error {
			bucket, err = v1alpha1.ClaimedBucket(kubeCli, minioProvider.S3)
			if err != nil || bucket == nil {
				return fmt.Errorf("wait bucket creating for logset %v, %v", client.ObjectKeyFromObject(ls), err)
			}
			expectedStatus := v1alpha1.BucketClaimStatus{
				BindTo: v1alpha1.BucketBindToMark(ls.ObjectMeta),
				State:  v1alpha1.StatusInUse,
			}
			if !reflect.DeepEqual(expectedStatus, bucket.Status) {
				return fmt.Errorf("bucket status is not inuse, current %v", bucket.Status)
			}
			return nil
		}, waitBucketStatusTimeout, time.Second*2).Should(Succeed())

		By("create a logset cluster in another namespace, claim same bucket as above")
		newNS := e2eutil.NewNamespaceTpl()
		Expect(kubeCli.Create(ctx, newNS)).Should(Succeed())

		lsShouldFailed := e2eutil.NewLogSetTpl(newNS.Name, fmt.Sprintf("%s:%s", moImageRepo, moVersion))
		lsShouldFailed.Spec.SharedStorage = minioProvider
		err = kubeCli.Create(ctx, lsShouldFailed)
		Expect(err != nil).Should(BeTrue())
		Expect(strings.Contains(err.Error(), "is invalid")).Should(BeTrue())
		Expect(strings.Contains(err.Error(), "spec.sharedStorage.s3")).Should(BeTrue())
		Expect(strings.Contains(err.Error(), "already bind to")).Should(BeTrue())

		By("delete above new namespace")
		Expect(kubeCli.Delete(ctx, newNS)).Should(Succeed())

		By("tear down logset cluster")
		Expect(kubeCli.Delete(ctx, ls)).To(Succeed())
		Eventually(func() error {
			return waitLogSetDeleted(ls)
		}, teardownClusterTimeout, pollInterval).Should(Succeed())
	})

	It("Should pvc retention and s3 retention policy been correctly set", func() {
		newNS := e2eutil.NewNamespaceTpl()
		Expect(kubeCli.Create(ctx, newNS)).To(Succeed())

		var logsetList []*v1alpha1.LogSet
		policyRetain := v1alpha1.PVCRetentionPolicyRetain
		policyDelete := v1alpha1.PVCRetentionPolicyDelete
		minioSecret := e2eutil.MinioSecret(newNS.Name)
		Expect(kubeCli.Create(ctx, minioSecret)).To(Succeed())

		By("both be Delete if neither pvc policy nor s3 policy set")
		minioProvider := e2eutil.MinioShareStorage(minioSecret.Name)
		minioProvider.S3.S3RetentionPolicy = nil
		lsNilNil := e2eutil.NewLogSetTpl(newNS.Name, fmt.Sprintf("%s:%s", moImageRepo, moVersion))
		lsNilNil.Spec.Replicas = 1
		lsNilNil.Spec.PVCRetentionPolicy = nil
		lsNilNil.Spec.SharedStorage = minioProvider
		Expect(kubeCli.Create(ctx, lsNilNil)).To(Succeed())
		Expect(kubeCli.Get(ctx, client.ObjectKeyFromObject(lsNilNil), lsNilNil)).To(Succeed())
		Expect(*lsNilNil.Spec.PVCRetentionPolicy).Should(Equal(policyDelete))
		Expect(*lsNilNil.Spec.SharedStorage.S3.S3RetentionPolicy).Should(Equal(policyDelete))
		logsetList = append(logsetList, lsNilNil)

		By("both be Retain if pvc is Retain and s3 policy not set")
		minioProvider = e2eutil.MinioShareStorage(minioSecret.Name)
		minioProvider.S3.S3RetentionPolicy = nil
		lsRetainNil := e2eutil.NewLogSetTpl(newNS.Name, fmt.Sprintf("%s:%s", moImageRepo, moVersion))
		lsRetainNil.Spec.Replicas = 1
		lsRetainNil.Spec.PVCRetentionPolicy = &policyRetain
		lsRetainNil.Spec.SharedStorage = minioProvider
		Expect(kubeCli.Create(ctx, lsRetainNil)).To(Succeed())
		Expect(kubeCli.Get(ctx, client.ObjectKeyFromObject(lsRetainNil), lsRetainNil)).To(Succeed())
		Expect(*lsRetainNil.Spec.PVCRetentionPolicy).Should(Equal(policyRetain))
		Expect(*lsRetainNil.Spec.SharedStorage.S3.S3RetentionPolicy).Should(Equal(policyRetain))
		logsetList = append(logsetList, lsRetainNil)

		By("both be Delete if pvc is Delete and s3 policy not set")
		minioProvider = e2eutil.MinioShareStorage(minioSecret.Name)
		minioProvider.S3.S3RetentionPolicy = nil
		lsDeleteNil := e2eutil.NewLogSetTpl(newNS.Name, fmt.Sprintf("%s:%s", moImageRepo, moVersion))
		lsDeleteNil.Spec.Replicas = 1
		lsDeleteNil.Spec.PVCRetentionPolicy = &policyDelete
		lsDeleteNil.Spec.SharedStorage = minioProvider
		Expect(kubeCli.Create(ctx, lsDeleteNil)).To(Succeed())
		Expect(kubeCli.Get(ctx, client.ObjectKeyFromObject(lsDeleteNil), lsDeleteNil)).To(Succeed())
		Expect(*lsDeleteNil.Spec.PVCRetentionPolicy).Should(Equal(policyDelete))
		Expect(*lsDeleteNil.Spec.SharedStorage.S3.S3RetentionPolicy).Should(Equal(policyDelete))
		logsetList = append(logsetList, lsDeleteNil)

		By("both be Retain if pvc not set and s3 policy is Retain")
		minioProvider = e2eutil.MinioShareStorage(minioSecret.Name)
		minioProvider.S3.S3RetentionPolicy = &policyRetain
		lsNilRetain := e2eutil.NewLogSetTpl(newNS.Name, fmt.Sprintf("%s:%s", moImageRepo, moVersion))
		lsNilRetain.Spec.Replicas = 1
		lsNilRetain.Spec.PVCRetentionPolicy = nil
		lsNilRetain.Spec.SharedStorage = minioProvider
		Expect(kubeCli.Create(ctx, lsNilRetain)).To(Succeed())
		Expect(kubeCli.Get(ctx, client.ObjectKeyFromObject(lsNilRetain), lsNilRetain)).To(Succeed())
		Expect(*lsNilRetain.Spec.PVCRetentionPolicy).Should(Equal(policyRetain))
		Expect(*lsNilRetain.Spec.SharedStorage.S3.S3RetentionPolicy).Should(Equal(policyRetain))
		logsetList = append(logsetList, lsNilRetain)

		By("keep unchanged is both set")
		minioProvider = e2eutil.MinioShareStorage(minioSecret.Name)
		minioProvider.S3.S3RetentionPolicy = &policyRetain
		lsDeleteRetain := e2eutil.NewLogSetTpl(newNS.Name, fmt.Sprintf("%s:%s", moImageRepo, moVersion))
		lsDeleteRetain.Spec.Replicas = 1
		lsDeleteRetain.Spec.PVCRetentionPolicy = &policyDelete
		lsDeleteRetain.Spec.SharedStorage = minioProvider
		Expect(kubeCli.Create(ctx, lsDeleteRetain)).To(Succeed())
		Expect(kubeCli.Get(ctx, client.ObjectKeyFromObject(lsDeleteRetain), lsDeleteRetain)).To(Succeed())
		Expect(*lsDeleteRetain.Spec.PVCRetentionPolicy).Should(Equal(policyDelete))
		Expect(*lsDeleteRetain.Spec.SharedStorage.S3.S3RetentionPolicy).Should(Equal(policyRetain))
		logsetList = append(logsetList, lsDeleteRetain)

		By("keep unchanged is both set")
		minioProvider = e2eutil.MinioShareStorage(minioSecret.Name)
		minioProvider.S3.S3RetentionPolicy = &policyDelete
		lsRetainDelete := e2eutil.NewLogSetTpl(newNS.Name, fmt.Sprintf("%s:%s", moImageRepo, moVersion))
		lsRetainDelete.Spec.Replicas = 1
		lsRetainDelete.Spec.PVCRetentionPolicy = &policyRetain
		lsRetainDelete.Spec.SharedStorage = minioProvider
		Expect(kubeCli.Create(ctx, lsRetainDelete)).To(Succeed())
		Expect(kubeCli.Get(ctx, client.ObjectKeyFromObject(lsRetainDelete), lsRetainDelete)).To(Succeed())
		Expect(*lsRetainDelete.Spec.PVCRetentionPolicy).Should(Equal(policyRetain))
		Expect(*lsRetainDelete.Spec.SharedStorage.S3.S3RetentionPolicy).Should(Equal(policyDelete))
		logsetList = append(logsetList, lsRetainDelete)

		By("tear down logset cluster")
		for _, ls := range logsetList {
			Expect(kubeCli.Delete(ctx, ls)).To(Succeed())
		}
		for _, ls := range logsetList {
			Eventually(func() error {
				return waitLogSetDeleted(ls)
			}, teardownClusterTimeout, pollInterval).Should(Succeed())
		}
		Expect(kubeCli.Delete(ctx, newNS)).Should(Succeed())
	})
})

func waitClusterDeleted(mo *v1alpha1.MatrixOneCluster) error {
	err := kubeCli.Get(ctx, client.ObjectKeyFromObject(mo), mo)
	if err == nil {
		logger.Infow("wait mo cluster teardown", "cluster", mo.Name)
		return errWait
	}
	if !apierrors.IsNotFound(err) {
		logger.Errorw("unexpected error when get mo cluster", "cluster", mo.Name, "error", err)
		return err
	}
	pvcList := &corev1.PersistentVolumeClaimList{}
	err = kubeCli.List(ctx, pvcList, client.InNamespace(mo.Namespace))
	if err != nil {
		logger.Errorw("error list PVCs", "error", err)
		return errWait
	}
	for _, pvc := range pvcList.Items {
		if strings.HasPrefix(pvc.Name, fmt.Sprintf("data-%s", mo.Name)) {
			logger.Infow("pvc is not yet cleaned", "name", pvc.Name)
			return errWait
		}
	}
	return nil
}

func waitLogSetDeleted(ls *v1alpha1.LogSet) error {
	err := kubeCli.Get(ctx, client.ObjectKeyFromObject(ls), ls)
	if err == nil {
		logger.Infow("wait logset teardown", "logset", ls.Name)
		return errWait
	}
	if !apierrors.IsNotFound(err) {
		logger.Errorw("unexpected error when get logset", "logset", ls, "error", err)
		return err
	}
	podList := &corev1.PodList{}
	err = kubeCli.List(ctx, podList, client.InNamespace(ls.Namespace))
	if err != nil {
		logger.Errorw("error list pods", "error", err)
		return err
	}
	for _, pod := range podList.Items {
		if strings.HasPrefix(pod.Name, ls.Name) {
			logger.Infow("Pod that belongs to the logset is not cleaned", "pod", pod.Name)
			return errWait
		}
	}
	return nil
}

func reclaimBucket(bucket *v1alpha1.BucketClaim) error {
	bucket.Finalizers = nil
	if err := kubeCli.Update(ctx, bucket); err != nil {
		return err
	}
	return kubeCli.Delete(ctx, bucket)
}

// ReclaimReleasedBucket recycle buckets, since mo-operator may be deleted before bucketClaims when deleting env namespace, its claim work will not success
// e2e test about logset/cnset/dnset/mocluster will also create buckets with default retain policy, these buckets should be deleted proactive
func ReclaimReleasedBucket() {
	buckets := &v1alpha1.BucketClaimList{}
	err := kubeCli.List(ctx, buckets, client.InNamespace(env.Namespace))
	Expect(err).To(Succeed())
	for _, b := range buckets.Items {
		// failed test cases may cause a bucket in InUse status
		if b.Status.State == v1alpha1.StatusInUse {
			logger.Warnf("unexpect bucket status:%v", b.Status)
		}
		if err = reclaimBucket(&b); err != nil {
			logger.Infof("reclaim bucket err: %v", err)
		}
	}
}
