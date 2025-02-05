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

package e2eminio

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/matrixorigin/matrixone-operator/test/e2e/util"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	minioNs       = "default"
	minioSelector = map[string]string{"app": "minio"}
)

const (
	MinioPort     = 9000
	MinioEndpoint = "127.0.0.1:9000"
	minioID       = "minio"
	minioKey      = "minio123"
)

func newMinioClient(endpoint, accessID, accessKey string) (*minio.Client, error) {
	return minio.New(endpoint, &minio.Options{
		Creds: credentials.NewStaticV4(accessID, accessKey, ""),
	})
}

func minioPod(c client.Client) (*corev1.Pod, error) {
	podList := &corev1.PodList{}
	err := c.List(context.TODO(), podList, client.InNamespace(minioNs), client.MatchingLabels(minioSelector))
	if err != nil {
		return nil, err
	}
	if len(podList.Items) == 0 {
		return nil, fmt.Errorf("no minio instance found")
	}
	return &podList.Items[0], nil
}

func PortForward(c client.Client, config *rest.Config) (*util.PortForwardHandler, error) {
	pod, err := minioPod(c)
	if err != nil {
		return nil, err
	}
	return util.PortForward(config, pod.Namespace, pod.Name, MinioPort, MinioPort)
}

func StopForward(handler *util.PortForwardHandler) {
	if handler == nil {
		return
	}
	handler.Stop()
}

func IsObjectExist(path string) (bool, error) {
	bucket, prefix := splitBucketPath(path)
	c, err := newMinioClient(MinioEndpoint, minioID, minioKey)
	if err != nil {
		return false, err
	}

	_, err = c.StatObject(context.TODO(), bucket, prefix, minio.StatObjectOptions{})
	if err != nil {
		errResp := minio.ToErrorResponse(err)
		if errResp.Code == "NoSuchKey" || errResp.Code == "NoSuchBucket" {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func PutObject(path string) (string, error) {
	c, err := newMinioClient(MinioEndpoint, minioID, minioKey)
	if err != nil {
		return "", err
	}

	bucket, prefix := splitBucketPath(path)
	if err = createBucketIfNotExist(c, bucket); err != nil {
		return "", err
	}

	fileName := "e2e-test-file-" + rand.String(6)
	reader := bytes.NewReader([]byte("hello world"))
	_, err = c.PutObject(context.TODO(), bucket, filepath.Join(prefix, fileName), reader, reader.Size(), minio.PutObjectOptions{})
	return filepath.Join(path, fileName), err
}

func createBucketIfNotExist(c *minio.Client, bucket string) error {
	exist, err := c.BucketExists(context.TODO(), bucket)
	if err != nil {
		return err
	}
	if !exist {
		err = c.MakeBucket(context.TODO(), bucket, minio.MakeBucketOptions{})
		if err != nil {
			errResp := minio.ToErrorResponse(err)
			if errResp.Code == "BucketAlreadyOwnedByYou" {
				return nil
			}
			return err
		}
	}
	return nil
}

func splitBucketPath(path string) (bucket, prefix string) {
	i := strings.Index(path, "/")
	switch {
	case i < 0:
		bucket = path
	case i == len(path):
		bucket = path[0:i]
	default:
		bucket = path[0:i]
		prefix = path[i+1:]
	}
	return
}
