// Copyright 2024 Matrix Origin
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package v1alpha1

import (
	"context"
	"crypto/sha1"
	"fmt"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"strings"
)

const (
	BucketUniqLabel = "matrixorigin.io/bucket-unique-id"

	// AnnAnyInstanceRunning is a bucket annotation indicates whether any pod instances running in a mo cluster.
	// pod instances include logset/cn/dn, if any pod of logset/cn/dn is ever in running status, this annotation will set to "true".
	// and this annotation can only be set to true, will not delete at any time.
	AnnAnyInstanceRunning = "bucket.matrixorigin.io/any-instance-running"

	// BucketDataFinalizer blocks BucketClaim reclaim until data in bucket has been recycled
	BucketDataFinalizer = "matrixorigin.io/bucket-data-finalizer"
	// BucketCNFinalizerPrefix is finalizer of cn set
	BucketCNFinalizerPrefix = "matrixorigin.io/CN"
	// BucketDNFinalizerPrefix is finalizer of dn set
	BucketDNFinalizerPrefix = "matrixorigin.io/DN"
)

// ClaimedBucket return claimed bucket according to S3Provider configuration, caller must ensure that provider is not nil
// NOTE: ClaimedBucket search bucket in cluster scope
func ClaimedBucket(c client.Client, provider *S3Provider) (*BucketClaim, error) {
	uniqLabel := UniqueBucketLabel(provider)
	bcList := &BucketClaimList{}
	if err := c.List(context.TODO(), bcList, client.MatchingLabels{BucketUniqLabel: uniqLabel}); err != nil {
		return nil, err
	}

	switch len(bcList.Items) {
	case 0:
		return nil, nil
	case 1:
		return &bcList.Items[0], nil
	default:
		return nil, fmt.Errorf("list more than one buckets")
	}
}

// UniqueBucketLabel generate an unique id for S3 provider, this id becomes a label in bucketClaim
func UniqueBucketLabel(s3Provider *S3Provider) string {
	var providerType string
	if s3Provider.Type != nil {
		providerType = string(*s3Provider.Type)
	}
	uniqId := fmt.Sprintf("%s-%s-%s", providerType, s3Provider.Endpoint, s3Provider.Path)
	uniqId = fmt.Sprintf("%x", sha1.Sum([]byte(uniqId)))
	return uniqId
}

func BucketBindToMark(logsetMeta metav1.ObjectMeta) string {
	return fmt.Sprintf("%s/%s", logsetMeta.Namespace, logsetMeta.Name)
}

func AddBucketFinalizer(ctx context.Context, c client.Client, lsMeta metav1.ObjectMeta, finalizer string) error {
	if lsMeta.Namespace == "" || lsMeta.Name == "" {
		return fmt.Errorf("bad logset meta %v", lsMeta)
	}
	ls := &LogSet{}
	err := c.Get(ctx, client.ObjectKey{Namespace: lsMeta.Namespace, Name: lsMeta.Name}, ls)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}
	s3 := ls.Spec.SharedStorage.S3
	if s3 == nil {
		return nil
	}
	bucket, err := ClaimedBucket(c, s3)
	if err != nil {
		return err
	}
	if bucket == nil {
		return nil
	}
	updated := controllerutil.AddFinalizer(bucket, finalizer)
	if updated {
		return c.Update(ctx, bucket)
	}
	return nil
}

func RemoveBucketFinalizer(ctx context.Context, c client.Client, lsMeta metav1.ObjectMeta, finalizer string) error {
	if lsMeta.Namespace == "" || lsMeta.Name == "" {
		return fmt.Errorf("bad losget meta %v", lsMeta)
	}

	ls := &LogSet{}
	err := c.Get(ctx, client.ObjectKey{Namespace: lsMeta.Namespace, Name: lsMeta.Name}, ls)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}
	s3 := ls.Spec.SharedStorage.S3
	if s3 == nil {
		return nil
	}
	bucket, err := ClaimedBucket(c, s3)
	if err != nil {
		return err
	}
	if bucket == nil {
		return nil
	}
	updated := controllerutil.RemoveFinalizer(bucket, finalizer)
	if updated {
		return c.Update(ctx, bucket)
	}
	return nil
}

func ContainFinalizerPrefix(finalizers []string, prefix string) bool {
	for _, f := range finalizers {
		if strings.HasPrefix(f, prefix) {
			return true
		}
	}
	return false
}

func SyncBucketEverRunningAnn(ctx context.Context, c client.Client, lsMeta metav1.ObjectMeta) error {
	if lsMeta.Namespace == "" || lsMeta.Name == "" {
		return fmt.Errorf("bad losget meta %v", lsMeta)
	}
	ls := &LogSet{}
	err := c.Get(ctx, client.ObjectKey{Namespace: lsMeta.Namespace, Name: lsMeta.Name}, ls)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}
	s3 := ls.Spec.SharedStorage.S3
	if s3 == nil {
		return nil
	}
	bucket, err := ClaimedBucket(c, s3)
	if err != nil || bucket == nil {
		// skip set annotation if bucket not exist
		return err
	}
	return SetBucketEverRunningAnn(ctx, c, bucket)
}

func SetBucketEverRunningAnn(ctx context.Context, c client.Client, bucket *BucketClaim) error {
	// skip if already exist
	if bucket.Annotations[AnnAnyInstanceRunning] != "" {
		return nil
	}
	if bucket.Annotations == nil {
		bucket.Annotations = make(map[string]string)
	}
	bucket.Annotations[AnnAnyInstanceRunning] = "true"
	return c.Update(ctx, bucket)
}
