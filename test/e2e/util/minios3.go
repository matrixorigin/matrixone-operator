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

package util

import (
	"context"
	"fmt"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strings"
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

func PortForwardMinio(c client.Client, config *rest.Config) (*PortForwardHandler, error) {
	pod, err := minioPod(c)
	if err != nil {
		return nil, err
	}
	return PortForward(config, pod.Namespace, pod.Name, MinioPort, MinioPort)
}

func StopMinioForward(handler *PortForwardHandler) {
	if handler == nil {
		return
	}
	handler.Stop()
}

func IsMinioPrefixExist(path string) (bool, error) {
	paths := strings.Split(path, "/")
	if len(paths) < 2 {
		return false, fmt.Errorf("unexpected bucket path")
	}
	c, err := newMinioClient(MinioEndpoint, minioID, minioKey)
	if err != nil {
		return false, err
	}

	_, err = c.StatObject(context.TODO(), paths[0], paths[1], minio.StatObjectOptions{})
	if err != nil {
		errResp := minio.ToErrorResponse(err)
		if errResp.Code == "NoSuchKey" || errResp.Code == "NoSuchBucket" {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
