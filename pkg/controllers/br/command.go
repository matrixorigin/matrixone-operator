// Copyright 2023 Matrix Origin
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

package br

import (
	"fmt"
	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	"strings"
)

const (
	MOUserEnvKey     = "MO_USER"
	MOPasswordEnvKey = "MO_PASSWORD"

	RestoreAccessEnvKey = "RESTORE_ACCESS_KEY_ID"
	RestoreSecretEnvKey = "RESTORE_SECRET_ACCESS_KEY"

	MetaDelimiter = "META_DELIMITER"
	RawMetaEnv    = "RAW_META"
)

type BackupCommand struct {
	Host string
	Port int
	S3   S3
}

type S3 struct {
	Endpoint      string
	Bucket        string
	Path          string
	Type          string
	ReadEnvSecret bool
}

func (b *BackupCommand) String() string {
	sb := strings.Builder{}
	sb.WriteString("/mo_br backup")
	sb.WriteString(fmt.Sprintf(" --host=%s", b.Host))
	sb.WriteString(fmt.Sprintf(" --port=%d", b.Port))
	sb.WriteString(" --user=$MO_USER")
	sb.WriteString(" --password=$MO_PASSWORD")
	sb.WriteString(" --backup_dir=s3")
	sb.WriteString(fmt.Sprintf(" --endpoint=%s", b.S3.Endpoint))
	sb.WriteString(fmt.Sprintf(" --bucket=%s", b.S3.Bucket))
	if b.S3.Path != "" {
		sb.WriteString(fmt.Sprintf(" --filepath=%s", b.S3.Path))
	}
	if b.S3.Type == string(v1alpha1.S3ProviderTypeMinIO) {
		sb.WriteString(" --is_minio")
	}
	if b.S3.ReadEnvSecret {
		sb.WriteString(" --access_key_id=$AWS_ACCESS_KEY_ID")
		sb.WriteString(" --secret_access_key=$AWS_SECRET_ACCESS_KEY")
	}
	sb.WriteString(fmt.Sprintf(" && echo %s && cat /mo_br.meta", MetaDelimiter))
	return sb.String()
}

type RestoreCommand struct {
	BackupID string
	Target   S3
	RawMeta  string

	ReadSourceEnvSecret bool
}

func (c *RestoreCommand) String() string {
	sb := strings.Builder{}
	sb.WriteString(fmt.Sprintf("echo $%s > /mo_br.meta", RawMetaEnv))
	sb.WriteString(` && sha256sum mo_br.meta | awk '{printf "%s",$1}' > mo_br.meta.sha256`)
	sb.WriteString(" && /mo_br restore")
	sb.WriteString(fmt.Sprintf(" %s", c.BackupID))
	sb.WriteString(" --restore_dir s3")
	if c.ReadSourceEnvSecret {
		sb.WriteString(" --backup_access_key_id=$AWS_ACCESS_KEY_ID")
		sb.WriteString(" --backup_secret_access_key=$AWS_SECRET_ACCESS_KEY")
	}
	sb.WriteString(fmt.Sprintf(" --restore_endpoint=%s", c.Target.Endpoint))
	sb.WriteString(fmt.Sprintf(" --restore_bucket=%s", c.Target.Bucket))
	if c.Target.Path != "" {
		sb.WriteString(fmt.Sprintf(" --restore_filepath=%s", c.Target.Path))
	}
	if c.Target.ReadEnvSecret {
		sb.WriteString(fmt.Sprintf(" --restore_access_key_id=$%s", RestoreAccessEnvKey))
		sb.WriteString(fmt.Sprintf(" --restore_secret_access_key=$%s", RestoreSecretEnvKey))
	}
	if c.Target.Type == string(v1alpha1.S3ProviderTypeMinIO) {
		sb.WriteString(" --restore_is_minio")
	}
	return sb.String()
}
