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

package logset

import (
	"fmt"
	recon "github.com/matrixorigin/controller-runtime/pkg/reconciler"
	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	"github.com/matrixorigin/matrixone-operator/api/features"
	kruisev1 "github.com/openkruise/kruise-api/apps/v1beta1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sort"
)

func (r *Actor) syncBucketClaim(ctx *recon.Context[*v1alpha1.LogSet], sts *kruisev1.StatefulSet) error {
	if !features.DefaultFeatureGate.Enabled(features.S3Reclaim) {
		return nil
	}

	ls := ctx.Obj
	// skip if no s3 share storage configuration
	if ls.Spec.SharedStorage.S3 == nil {
		return nil
	}

	bucket, err := v1alpha1.ClaimedBucket(ctx.Client, ls.Spec.SharedStorage.S3)
	if err != nil {
		return err
	}

	// create new bucket if no bucket found
	if bucket == nil {
		return r.createNewBucket(ctx, sts)
	}
	if bucket.Status.BindTo != "" &&
		bucket.Status.BindTo != v1alpha1.BucketBindToMark(ls.ObjectMeta) {
		return fmt.Errorf("claim bucket %v already bind to %s", client.ObjectKeyFromObject(bucket), bucket.Status.BindTo)
	}

	targetBucket := bucket.DeepCopy()
	controllerutil.AddFinalizer(targetBucket, v1alpha1.BucketDataFinalizer)
	targetBucket.Spec.S3 = ls.Spec.SharedStorage.S3
	targetBucket.Status.BindTo = v1alpha1.BucketBindToMark(ls.ObjectMeta)
	targetBucket.Status.State = v1alpha1.StatusInUse
	sort.Strings(targetBucket.Finalizers)
	sort.Strings(bucket.Finalizers)
	if !equality.Semantic.DeepEqual(targetBucket, bucket) {
		return ctx.Update(targetBucket)
	}
	return nil
}

// finalizeBucket marks bucket as deleted for BucketClaim with Delete policy, delete always happen after deletion of
// logset, and will not block deletion of logset.
// logset can be deleted only after bucket status is correctly set (with the constraints of its finalizer)
func (r *Actor) finalizeBucket(ctx *recon.Context[*v1alpha1.LogSet]) (success bool, err error) {
	if !features.DefaultFeatureGate.Enabled(features.S3Reclaim) {
		return true, nil
	}

	ls := ctx.Obj
	// skip if no s3 share storage configuration
	if ls.Spec.SharedStorage.S3 == nil {
		return true, nil
	}

	claimedBucket, err := v1alpha1.ClaimedBucket(ctx.Client, ls.Spec.SharedStorage.S3)
	if err != nil {
		return false, err
	}
	// skip if no bucket found
	if claimedBucket == nil {
		return true, nil
	}
	if claimedBucket.Status.BindTo != "" &&
		claimedBucket.Status.BindTo != v1alpha1.BucketBindToMark(ls.ObjectMeta) {
		return false, fmt.Errorf("claimed bucket %v already bind to %s", client.ObjectKeyFromObject(claimedBucket), claimedBucket.Status.BindTo)
	}

	if v1alpha1.ContainFinalizerPrefix(claimedBucket.Finalizers, v1alpha1.BucketDNFinalizerPrefix) ||
		v1alpha1.ContainFinalizerPrefix(claimedBucket.Finalizers, v1alpha1.BucketCNFinalizerPrefix) {
		return false, nil
	}

	s3RetentionPolicy := ls.Spec.GetS3RetentionPolicy()
	if s3RetentionPolicy == nil {
		// generally we will never enter this block
		return true, nil
	}
	switch *s3RetentionPolicy {
	case v1alpha1.PVCRetentionPolicyRetain:
		claimedBucket.Status.State = v1alpha1.StatusReleased
		claimedBucket.Status.BindTo = ""
		return true, ctx.Update(claimedBucket)
	case v1alpha1.PVCRetentionPolicyDelete:
		claimedBucket.Status.State = v1alpha1.StatusDeleting
		claimedBucket.Status.BindTo = ""
		if err = ctx.Update(claimedBucket); err != nil {
			return false, err
		}
		return true, ctx.Delete(claimedBucket)
	default:
		return false, fmt.Errorf("unknown s3 retention policy %s", *s3RetentionPolicy)
	}
}

func (r *Actor) createNewBucket(ctx *recon.Context[*v1alpha1.LogSet], sts *kruisev1.StatefulSet) error {
	ls := ctx.Obj
	newBucketName := fmt.Sprintf("bucket-%s", ls.ObjectMeta.UID)
	bucket := &v1alpha1.BucketClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:       newBucketName,
			Namespace:  ls.Namespace,
			Finalizers: []string{v1alpha1.BucketDataFinalizer},
			Labels:     map[string]string{v1alpha1.BucketUniqLabel: v1alpha1.UniqueBucketLabel(ls.Spec.SharedStorage.S3)},
		},
		Spec: v1alpha1.BucketClaimSpec{
			S3:             ls.Spec.SharedStorage.S3,
			LogSetTemplate: sts.Spec.Template,
		},
		Status: v1alpha1.BucketClaimStatus{
			BindTo: v1alpha1.BucketBindToMark(ls.ObjectMeta),
			State:  v1alpha1.StatusInUse,
		},
	}
	return ctx.Create(bucket)
}
