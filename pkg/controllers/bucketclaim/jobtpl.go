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

package bucketclaim

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/matrixorigin/controller-runtime/pkg/util"
	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	"github.com/matrixorigin/matrixone-operator/pkg/utils"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	defaultImage = "amazon/aws-cli"

	// entrypoint of job pod
	entrypoint  = "start.sh"
	cmMountPath = "/etc/aws-cli-config"
)

/*
command example:
aws --endpoint-url https://play.minio.io:9000 --region us-east-1 s3api head-bucket test0407
aws --endpoint-url https://play.minio.io:9000 --region us-east-1 s3 rm s3://test0407/ddf --recursive

First check if bucket exists, if bucket exists then remove with '--recursive' option, remove will success even if path not exist
*/
var delCmd = template.Must(template.New("del-bucket-script").Parse(`
#!/bin/sh

set -ex

region={{ .Region }}
endpoint={{ .EndPoint }}

if [ -n "${region}" ]; then
  export AWS_REGION="${region}"
fi

if [ -n "${endpoint}" ]; then
  aws --endpoint-url "${endpoint}" s3api head-bucket --bucket {{ .Bucket }} || bucketNotExist=true
  if [ $bucketNotExist ]; then
    echo "bucket not exist"
    exit 0
  fi
  aws --endpoint-url "${endpoint}" s3 rm s3://{{ .Path }} --recursive
else
  aws s3api head-bucket --bucket {{ .Bucket }} || bucketNotExist=true
  if [ $bucketNotExist ]; then
    echo "bucket not exist"
    exit 0
  fi
  aws s3 rm s3://{{ .Path }} --recursive
fi
`))

type s3Param struct {
	Path     string
	EndPoint string
	Region   string
	Bucket   string
}

func parseEntrypoint(s3 *v1alpha1.S3Provider) (string, error) {
	bucketPath := strings.Split(s3.Path, "/")
	if len(bucketPath) < 0 {
		return "", fmt.Errorf("unexpected bucket path: %s", s3.Path)
	}
	buf := new(bytes.Buffer)
	err := delCmd.Execute(buf, &s3Param{
		Path:     s3.Path,
		EndPoint: s3.Endpoint,
		Region:   s3.Region,
		Bucket:   bucketPath[0],
	})
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}

func (bca *Actor) NewCmTpl(bucket *v1alpha1.BucketClaim) (*corev1.ConfigMap, error) {
	entry, err := parseEntrypoint(bucket.Spec.S3)
	if err != nil {
		return nil, err
	}

	cm := &corev1.ConfigMap{
		ObjectMeta: v1.ObjectMeta{
			Name:      cmName(bucket),
			Namespace: bucket.Namespace,
		},
		Data: map[string]string{
			entrypoint: entry,
		},
	}
	return cm, nil
}

func (bca *Actor) NewJobTpl(bucket *v1alpha1.BucketClaim, cm *corev1.ConfigMap) *batchv1.Job {
	podTpl := bucket.Spec.LogSetTemplate
	mainContainer := util.FindFirst(podTpl.Spec.Containers, func(c corev1.Container) bool {
		return c.Name == v1alpha1.ContainerMain
	})
	mainContainer.Image = bca.image
	mainContainer.Command = []string{"/bin/sh", filepath.Join(cmMountPath, entrypoint)}
	mainContainer.Args = []string{}
	podTpl.Spec.RestartPolicy = corev1.RestartPolicyOnFailure
	podTpl.Spec.ImagePullSecrets = bca.imagePullSecrets

	// remove all labels inherit from logset pod
	podTpl.Labels = nil

	// NOTE: make bucket deletion pod as best effort
	mainContainer.Resources = corev1.ResourceRequirements{}

	// ignore volumes from logset, which may dependent other resources(like configmap) and these resource may not exist
	mainContainer.VolumeMounts = []corev1.VolumeMount{{
		Name:      "aws-cli-config",
		MountPath: cmMountPath,
	}}
	podTpl.Spec.Containers = []corev1.Container{*mainContainer}

	mod := int32(0644)
	podTpl.Spec.Volumes = []corev1.Volume{{
		Name: "aws-cli-config",
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{Name: cm.Name},
				DefaultMode:          &mod,
			},
		},
	}}

	job := &batchv1.Job{
		ObjectMeta: v1.ObjectMeta{
			Name:      jobName(bucket),
			Namespace: bucket.Namespace,
		},
		Spec: batchv1.JobSpec{
			Parallelism: utils.PtrTo(int32(1)),
			Completions: utils.PtrTo(int32(1)),
			Template:    podTpl,
		},
	}
	return job
}

func jobName(bucket *v1alpha1.BucketClaim) string {
	return fmt.Sprintf("job-%s", bucket.Name)
}

func cmName(bucket *v1alpha1.BucketClaim) string {
	return fmt.Sprintf("cm-%s", bucket.Name)
}
