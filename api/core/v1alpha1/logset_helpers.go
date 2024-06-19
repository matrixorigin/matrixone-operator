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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (l *LogSet) AsDependency() LogSetRef {
	return LogSetRef{
		LogSet: &LogSet{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: l.Namespace,
				Name:      l.Name,
			},
		},
	}
}

// setDefaultRetentionPolicy always set PVCRetentionPolicy, and always set S3RetentionPolicy only if S3 is not nil
// setDefaultRetentionPolicy does not change origin policy and only set default value when policy is nil
func (l *LogSetSpec) setDefaultRetentionPolicy() {
	defaultDeletePolicy := PVCRetentionPolicyDelete

	if l.SharedStorage.S3 == nil {
		if l.PVCRetentionPolicy == nil {
			l.PVCRetentionPolicy = &defaultDeletePolicy
		}
		return
	}

	pvcPolicy := l.PVCRetentionPolicy
	s3Policy := l.SharedStorage.S3.S3RetentionPolicy

	switch {
	// if both set, does not set any values
	case pvcPolicy != nil && s3Policy != nil:
		return
	// if both not set, set to delete
	case pvcPolicy == nil && s3Policy == nil:
		l.PVCRetentionPolicy = &defaultDeletePolicy
		l.SharedStorage.S3.S3RetentionPolicy = &defaultDeletePolicy
	// if only set pvcPolicy, set it to s3Policy
	case pvcPolicy != nil && s3Policy == nil:
		l.SharedStorage.S3.S3RetentionPolicy = pvcPolicy
	// if only set s3Policy, set it to pvcPolicy
	case pvcPolicy == nil && s3Policy != nil:
		l.PVCRetentionPolicy = s3Policy
	}
}
