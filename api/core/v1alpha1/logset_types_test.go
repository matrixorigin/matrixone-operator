// Copyright 2025-2026 Matrix Origin
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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSetDefaultRetentionPolicy(t *testing.T) {
	del := PVCRetentionPolicyDelete
	retain := PVCRetentionPolicyRetain

	testCases := []struct {
		logset    LogSetSpec
		pvcPolicy *PVCRetentionPolicy
		s3Policy  *PVCRetentionPolicy
	}{
		// does not set any policies
		{
			logset:    LogSetSpec{},
			pvcPolicy: &del,
			s3Policy:  nil,
		},

		// set only one policy, four cases
		{
			logset:    LogSetSpec{PVCRetentionPolicy: &del},
			pvcPolicy: &del,
			s3Policy:  nil,
		},
		{
			logset:    LogSetSpec{SharedStorage: SharedStorageProvider{S3: &S3Provider{S3RetentionPolicy: &del}}},
			pvcPolicy: &del,
			s3Policy:  &del,
		},
		{
			logset:    LogSetSpec{PVCRetentionPolicy: &retain},
			pvcPolicy: &retain,
			s3Policy:  nil,
		},
		{
			logset:    LogSetSpec{SharedStorage: SharedStorageProvider{S3: &S3Provider{S3RetentionPolicy: &retain}}},
			pvcPolicy: &retain,
			s3Policy:  &retain,
		},

		// all policy set, four cases
		{
			logset: LogSetSpec{
				PVCRetentionPolicy: &retain,
				SharedStorage:      SharedStorageProvider{S3: &S3Provider{S3RetentionPolicy: &retain}},
			},
			pvcPolicy: &retain,
			s3Policy:  &retain,
		},
		{
			logset: LogSetSpec{
				PVCRetentionPolicy: &del,
				SharedStorage:      SharedStorageProvider{S3: &S3Provider{S3RetentionPolicy: &retain}},
			},
			pvcPolicy: &del,
			s3Policy:  &retain,
		},
		{
			logset: LogSetSpec{
				PVCRetentionPolicy: &retain,
				SharedStorage:      SharedStorageProvider{S3: &S3Provider{S3RetentionPolicy: &del}},
			},
			pvcPolicy: &retain,
			s3Policy:  &del,
		},
		{
			logset: LogSetSpec{
				PVCRetentionPolicy: &del,
				SharedStorage:      SharedStorageProvider{S3: &S3Provider{S3RetentionPolicy: &del}},
			},
			pvcPolicy: &del,
			s3Policy:  &del,
		},
	}

	for _, c := range testCases {
		(&c.logset).setDefaultRetentionPolicy()
		assert.NotNil(t, c.pvcPolicy)
		assert.Equal(t, *c.logset.PVCRetentionPolicy, *c.pvcPolicy)
		if c.logset.SharedStorage.S3 != nil {
			assert.NotNil(t, c.s3Policy)
			assert.Equal(t, *c.logset.SharedStorage.S3.S3RetentionPolicy, *c.s3Policy)
		} else {
			assert.Nil(t, c.s3Policy)
		}
	}
}

func TestGetRetentionPolicy(t *testing.T) {
	del := PVCRetentionPolicyDelete
	retain := PVCRetentionPolicyRetain
	s3Tpl := func(policy *PVCRetentionPolicy) SharedStorageProvider {
		return SharedStorageProvider{S3: &S3Provider{S3RetentionPolicy: policy}}
	}

	testCases := []struct {
		name      string
		logset    LogSetSpec
		pvcPolicy PVCRetentionPolicy
		s3Policy  *PVCRetentionPolicy
	}{
		{
			name:      "both nil with s3 returns delete",
			logset:    LogSetSpec{SharedStorage: s3Tpl(nil)},
			pvcPolicy: del,
			s3Policy:  &del,
		},
		{
			name:      "pvc only delete inherits to s3 getter",
			logset:    LogSetSpec{PVCRetentionPolicy: &del, SharedStorage: s3Tpl(nil)},
			pvcPolicy: del,
			s3Policy:  &del,
		},
		{
			name:      "s3 only retain inherits to pvc getter",
			logset:    LogSetSpec{SharedStorage: s3Tpl(&retain)},
			pvcPolicy: retain,
			s3Policy:  &retain,
		},
		{
			name: "both set delete and retain independently",
			logset: LogSetSpec{
				PVCRetentionPolicy: &del,
				SharedStorage:      s3Tpl(&retain),
			},
			pvcPolicy: del,
			s3Policy:  &retain,
		},
		{
			name: "both set retain and delete independently",
			logset: LogSetSpec{
				PVCRetentionPolicy: &retain,
				SharedStorage:      s3Tpl(&del),
			},
			pvcPolicy: retain,
			s3Policy:  &del,
		},
		{
			name: "both set retain",
			logset: LogSetSpec{
				PVCRetentionPolicy: &retain,
				SharedStorage:      s3Tpl(&retain),
			},
			pvcPolicy: retain,
			s3Policy:  &retain,
		},
		{
			name: "both set delete",
			logset: LogSetSpec{
				PVCRetentionPolicy: &del,
				SharedStorage:      s3Tpl(&del),
			},
			pvcPolicy: del,
			s3Policy:  &del,
		},
		{
			name:      "no s3 returns nil s3 policy",
			logset:    LogSetSpec{PVCRetentionPolicy: &retain},
			pvcPolicy: retain,
			s3Policy:  nil,
		},
		{
			name:      "no s3 and no pvc returns delete",
			logset:    LogSetSpec{},
			pvcPolicy: del,
			s3Policy:  nil,
		},
	}

	for _, c := range testCases {
		t.Run(c.name, func(t *testing.T) {
			spec := c.logset
			origPVC := spec.PVCRetentionPolicy
			var origS3 *PVCRetentionPolicy
			if spec.SharedStorage.S3 != nil {
				origS3 = spec.SharedStorage.S3.S3RetentionPolicy
			}

			assert.Equal(t, c.pvcPolicy, spec.GetPVCRetentionPolicy())

			s3Policy := spec.GetS3RetentionPolicy()
			if c.s3Policy == nil {
				assert.Nil(t, s3Policy)
			} else {
				assert.NotNil(t, s3Policy)
				assert.Equal(t, *c.s3Policy, *s3Policy)
			}

			// getters must not mutate spec
			assert.Equal(t, origPVC, spec.PVCRetentionPolicy)
			if spec.SharedStorage.S3 != nil {
				assert.Equal(t, origS3, spec.SharedStorage.S3.S3RetentionPolicy)
			}
		})
	}
}
