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

package common

import (
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/matrixorigin/controller-runtime/pkg/util"
	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
)

const (
	// BackupFileServiceName names the fileservice instance (defined by mo-operator) from which the hakeeper backup data can be read
	BackupFileServiceName = "BACKUP"

	AWSAccessKeyID     = "AWS_ACCESS_KEY_ID"
	AWSSecretAccessKey = "AWS_SECRET_ACCESS_KEY"
	AWSRegion          = "AWS_REGION"

	S3CertificateVolume = "s3-ssl"
	S3CertificatePath   = "/etc/s3-ssl"
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

	defaultAWSRegion = "us-west-2"

	byteSuffix = "B"
)

// SetStorageProviderConfig set inject configuration of storage provider to Pods
func SetStorageProviderConfig(sp v1alpha1.SharedStorageProvider, podSpec *corev1.PodSpec) {
	// S3 storage provider config
	if s3p := sp.S3; s3p != nil {
		// mount optional certificate secret volume
		if s3p.CertificateRef != nil {
			podSpec.Volumes = util.UpsertByKey(podSpec.Volumes, corev1.Volume{
				Name: S3CertificateVolume,
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: s3p.CertificateRef.Name,
					},
				},
			}, func(v corev1.Volume) string {
				return v.Name
			})
		}
		for i := range podSpec.Containers {
			if s3p.SecretRef != nil {
				for _, key := range []string{AWSAccessKeyID, AWSSecretAccessKey} {
					podSpec.Containers[i].Env = util.UpsertByKey(podSpec.Containers[i].Env, corev1.EnvVar{Name: key, ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: *s3p.SecretRef,
							Key:                  key,
						},
					}}, util.EnvVarKey)
				}
			}
			if s3p.CertificateRef != nil {
				podSpec.Containers[i].VolumeMounts = util.UpsertByKey(podSpec.Containers[i].VolumeMounts, corev1.VolumeMount{
					Name:      S3CertificateVolume,
					MountPath: S3CertificatePath,
					ReadOnly:  true,
				}, func(v corev1.VolumeMount) string {
					return v.Name
				})
			}
			region := s3p.Region
			if region == "" {
				region = defaultAWSRegion
			}
			podSpec.Containers[i].Env = util.UpsertByKey(podSpec.Containers[i].Env, corev1.EnvVar{Name: AWSRegion, Value: region}, util.EnvVarKey)
		}
	}
}

// FileServiceConfig generate the fileservice config for an MO component
func FileServiceConfig(localPath string, sp v1alpha1.SharedStorageProvider, cache *v1alpha1.SharedStorageCache) map[string]interface{} {
	localFS := map[string]interface{}{
		"name":     localFileServiceName,
		"backend":  fsBackendTypeDisk,
		"data-dir": localPath,
	}
	// MO Operator currently unifies the storage DB data and ETL data to a single shared storage
	// for user. We may provide options to configure the shared storages of DB and ETL separately if
	// we found it necessary in the future.
	s3FS := sharedFileServiceConfig(sp, cache, s3FileServiceName, "data")
	etlFS := sharedFileServiceConfig(sp, nil, etlFileServiceName, "etl")
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

// LogServiceFSConfig generate the fileservice config for log-service
func LogServiceFSConfig(localPath string, sp v1alpha1.SharedStorageProvider) map[string]interface{} {
	backupFS := sharedFileServiceConfig(sp, nil, BackupFileServiceName, "")
	return map[string]interface{}{
		"data-dir": localPath,
		"fileservice": []map[string]interface{}{
			backupFS,
		},
	}
}

func sharedFileServiceConfig(sp v1alpha1.SharedStorageProvider, cache *v1alpha1.SharedStorageCache, name, subDir string) map[string]interface{} {
	m := map[string]interface{}{
		"name": name,
	}
	// S3 file service config
	if s3 := sp.S3; s3 != nil {
		switch s3.GetProviderType() {
		case v1alpha1.S3ProviderTypeMinIO:
			m["backend"] = fsBackendTypeMinio
		case v1alpha1.S3ProviderTypeAWS:
			m["backend"] = fsBackendTypeS3
		}
		s3Config := map[string]interface{}{}

		// init default values
		// TODO: let AWS SDK discover its own endpoint by default
		s3Config["endpoint"] = "s3.us-west-2.amazonaws.com"

		if s3.Endpoint != "" {
			s3Config["endpoint"] = s3.Endpoint
		}
		if s3.Region != "" {
			s3Config["region"] = s3.Region
		}

		paths := strings.SplitN(strings.Trim(s3.Path, "/"), "/", 2)
		s3Config["bucket"] = paths[0]
		keyPrefix := subDir
		if len(paths) > 1 {
			keyPrefix = fmt.Sprintf("%s/%s", strings.Trim(paths[1], "/"), subDir)
		}
		s3Config["key-prefix"] = keyPrefix

		if s3.CertificateRef != nil {
			var certFiles []string
			for _, f := range s3.CertificateRef.Files {
				certFiles = append(certFiles, fmt.Sprintf("%s/%s", S3CertificatePath, f))
			}
			s3Config["cert-files"] = certFiles
		}
		m["s3"] = s3Config
	}
	// filesystem file service config
	if fs := sp.FileSystem; fs != nil {
		if name == etlFileServiceName {
			m["backend"] = fsBackendTypeDiskETL
		} else {
			m["backend"] = fsBackendTypeDisk
		}
		m["data-dir"] = fs.Path
	}
	cacheConfig := map[string]string{}
	if cache != nil {
		if cache.MemoryCacheSize != nil {
			cacheConfig["memory-capacity"] = asSizeBytes(*cache.MemoryCacheSize)
		}
		if cache.DiskCacheSize != nil {
			cacheConfig["disk-capacity"] = asSizeBytes(*cache.DiskCacheSize)
			cacheConfig["disk-path"] = fmt.Sprintf("%s/%s", DataPath, S3CacheDir)
		}
	} else {
		// disable cache
		cacheConfig["memory-capacity"] = "1B"
	}
	if len(cacheConfig) > 0 {
		m["cache"] = cacheConfig
	}
	return m
}

// asSizeBytes convert a Quantity to a size byte string
func asSizeBytes(q resource.Quantity) string {
	// workaround https://github.com/matrixorigin/matrixone/issues/9507,
	// will still keep this after the issue get fixed for better compatibility
	return q.String() + byteSuffix
}
