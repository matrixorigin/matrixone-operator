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

package bucketclaim

import (
	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestParseEndpoint(t *testing.T) {
	s3 := v1alpha1.S3Provider{
		Path:     "minio-mo/test",
		Endpoint: "http://minio.mostorage:9000",
		Region:   "us-east-1",
	}
	expect := `
#!/bin/sh

set -ex

region=us-east-1
endpoint=http://minio.mostorage:9000

if [ -n "${region}" ]; then
  export AWS_REGION="${region}"
fi

if [ -n "${endpoint}" ]; then
  aws --endpoint-url "${endpoint}" s3api head-bucket --bucket minio-mo || bucketNotExist=true
  if [ $bucketNotExist ]; then
    echo "bucket not exist"
    exit 0
  fi
  aws --endpoint-url "${endpoint}" s3 rm s3://minio-mo/test --recursive
else
  aws s3api head-bucket --bucket minio-mo || bucketNotExist=true
  if [ $bucketNotExist ]; then
    echo "bucket not exist"
    exit 0
  fi
  aws s3 rm s3://minio-mo/test --recursive
fi
`
	endpoint, err := parseEntrypoint(&s3)
	assert.Nil(t, err)
	assert.Equal(t, expect, endpoint)
}
