// Copyright 2025 Matrix Origin
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

package common

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func TestFileServiceConfig(t *testing.T) {
	type args struct {
		localPath string
		sp        v1alpha1.SharedStorageProvider
		c         *v1alpha1.SharedStorageCache
	}
	quantity1GiB := resource.MustParse("1Gi")
	tests := []struct {
		name string
		args args
		want map[string]interface{}
	}{{
		name: "s3",
		args: args{
			localPath: "/test",
			sp: v1alpha1.SharedStorageProvider{
				S3: &v1alpha1.S3Provider{
					Path: "bucket/prefix",
					SecretRef: &corev1.LocalObjectReference{
						Name: "aws",
					},
				},
			},
			c: nil,
		},
		want: map[string]interface{}{
			"data-dir": "/test",
			"fileservice": []map[string]interface{}{{
				"name":     "LOCAL",
				"data-dir": "/test",
				"backend":  "DISK",
			}, {
				"name":    "S3",
				"backend": "S3",
				"cache": map[string]string{
					"memory-capacity": "1B",
				},
				"s3": map[string]interface{}{
					"endpoint":   "s3.us-west-2.amazonaws.com",
					"key-prefix": "prefix/data",
					"bucket":     "bucket",
				},
			}, {
				"name":    "ETL",
				"backend": "S3",
				"cache": map[string]string{
					"memory-capacity": "1B",
				},
				"s3": map[string]interface{}{
					"endpoint":   "s3.us-west-2.amazonaws.com",
					"key-prefix": "prefix/etl",
					"bucket":     "bucket",
				},
			}},
		},
	}, {
		name: "s3 cache",
		args: args{
			localPath: "/test",
			sp: v1alpha1.SharedStorageProvider{
				S3: &v1alpha1.S3Provider{
					Path: "/bucket/prefix",
					SecretRef: &corev1.LocalObjectReference{
						Name: "aws",
					},
				},
			},
			c: &v1alpha1.SharedStorageCache{
				MemoryCacheSize: &quantity1GiB,
				DiskCacheSize:   &quantity1GiB,
			},
		},
		want: map[string]interface{}{
			"data-dir": "/test",
			"fileservice": []map[string]interface{}{{
				"name":     "LOCAL",
				"data-dir": "/test",
				"backend":  "DISK",
			}, {
				"name":    "S3",
				"backend": "S3",
				"s3": map[string]interface{}{
					"endpoint":   "s3.us-west-2.amazonaws.com",
					"key-prefix": "prefix/data",
					"bucket":     "bucket",
				},
				"cache": map[string]string{
					"memory-capacity": "1GiB",
					"disk-path":       "/var/lib/matrixone/disk-cache",
					"disk-capacity":   "1GiB",
				},
			}, {
				"name":    "ETL",
				"backend": "S3",
				"s3": map[string]interface{}{
					"endpoint":   "s3.us-west-2.amazonaws.com",
					"key-prefix": "prefix/etl",
					"bucket":     "bucket",
				},
				"cache": map[string]string{
					"memory-capacity": "1B",
				},
			}},
		},
	},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if diff := cmp.Diff(tt.want, FileServiceConfig(tt.args.localPath, tt.args.sp, tt.args.c)); diff != "" {
				t.Errorf("FileServiceConfig(), diff:\n %s", diff)
			}
		})
	}
}
