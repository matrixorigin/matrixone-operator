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

package common

import (
	"fmt"
	"github.com/matrixorigin/controller-runtime/pkg/util"
	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"strings"
)

const (
	// s3FileServiceName names the fileservice instance used as shared data storage
	s3FileServiceName = "S3"
	// localFileServiceName names the fileservice instance used as local data or cache storage
	localFileServiceName = "LOCAL"
	// etlFilServiceName names the fileservice instance used as ETL data storage (current for monitoring data)
	etlFileServiceName = "ETL"

	fsBackendTypeDisk    = "DISK"
	fsBackendTypeDiskETL = "DISK-ETL"
	fsBackendTypeS3      = "S3"
	fsBackendTypeMinio   = "MINIO"

	awsAccessKeyID     = "AWS_ACCESS_KEY_ID"
	awsSecretAccessKey = "AWS_SECRET_ACCESS_KEY"
	awsRegion          = "AWS_REGION"
	defaultAWSRegion   = "us-west-2"
)

// SetStorageProviderConfig set inject configuration of storage provider to Pods
func SetStorageProviderConfig(sp v1alpha1.SharedStorageProvider, podSpec *corev1.PodSpec) {
	for i := range podSpec.Containers {
		if s3p := sp.S3; s3p != nil {
			if s3p.SecretRef != nil {
				for _, key := range []string{awsAccessKeyID, awsSecretAccessKey} {
					podSpec.Containers[i].Env = util.UpsertByKey(podSpec.Containers[i].Env, corev1.EnvVar{Name: key, ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: *s3p.SecretRef,
							Key:                  key,
						},
					}}, util.EnvVarKey)
				}
			}
			region := s3p.Region
			if region == "" {
				region = defaultAWSRegion
			}
			podSpec.Containers[i].Env = util.UpsertByKey(podSpec.Containers[i].Env, corev1.EnvVar{Name: awsRegion, Value: region}, util.EnvVarKey)
		}
	}
}

// FileServiceConfig generate the fileservice config for an MO component
func FileServiceConfig(localPath string, sp v1alpha1.SharedStorageProvider, v *v1alpha1.Volume, cache *v1alpha1.SharedStorageCache) map[string]interface{} {
	localFS := map[string]interface{}{
		"name":     localFileServiceName,
		"backend":  fsBackendTypeDisk,
		"data-dir": localPath,
	}
	if v != nil && v.MemoryCacheSize != nil {
		localFS["cache"] = map[string]string{
			"memory-capacity": v.MemoryCacheSize.String(),
		}
	}
	// MO Operator currently unifies the storage DB data and ETL data to a single shared storage
	// for user. We may provide options to configure the shared storages of DB and ETL separately if
	// we found it necessary in the future.
	s3FS := sharedFileServiceConfig(sp, cache, s3FileServiceName, "data")
	etlFS := sharedFileServiceConfig(sp, cache, etlFileServiceName, "etl")
	return map[string]interface{}{
		// some data are not accessed by fileservice and will be read/written at `data-dir` directly
		"data-dir": localPath,
		"fileservice": []map[string]interface{}{
			localFS,
			s3FS,
			etlFS,
		},
	}
}

func sharedFileServiceConfig(sp v1alpha1.SharedStorageProvider, cache *v1alpha1.SharedStorageCache, name, subDir string) map[string]interface{} {
	m := map[string]interface{}{
		"name": name,
	}
	if s3 := sp.S3; s3 != nil {
		switch s3.GetProviderType() {
		case v1alpha1.S3ProviderTypeMinIO:
			m["backend"] = fsBackendTypeMinio
		case v1alpha1.S3ProviderTypeAWS:
			m["backend"] = fsBackendTypeS3
		}
		s3Config := map[string]interface{}{}
		if s3.Endpoint != "" {
			s3Config["endpoint"] = s3.Endpoint
		} else {
			// TODO: let AWS SDK discover its own endpoint by default
			s3Config["endpoint"] = "s3.us-west-2.amazonaws.com"
		}
		paths := strings.SplitN(strings.Trim(s3.Path, "/"), "/", 2)
		s3Config["bucket"] = paths[0]
		keyPrefix := subDir
		if len(paths) > 1 {
			keyPrefix = fmt.Sprintf("%s/%s", strings.Trim(paths[1], "/"), subDir)
		}
		s3Config["key-prefix"] = keyPrefix

		m["s3"] = s3Config
	}
	if fs := sp.FileSystem; fs != nil {
		if name == etlFileServiceName {
			m["backend"] = fsBackendTypeDiskETL
		} else {
			m["backend"] = fsBackendTypeDisk
		}
		m["data-dir"] = fs.Path
	}
	if cache != nil {
		c := map[string]string{}
		if cache.MemoryCacheSize != nil {
			c["memory-capacity"] = cache.MemoryCacheSize.String()
		}
		if cache.DiskCacheSize != nil {
			c["disk-capacity"] = cache.DiskCacheSize.String()
			switch name {
			case s3FileServiceName:
				c["disk-path"] = fmt.Sprintf("%s/%s", DataPath, S3CacheDir)
			case etlFileServiceName:
				c["disk-path"] = fmt.Sprintf("%s/%s", DataPath, ETLCacheDir)
			}
		}
		if len(c) > 0 {
			m["cache"] = c
		}
	}

	return m
}
