// Copyright 2022 Matrix Origin
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
	"context"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("LogSet Webhook", func() {

	It("should accept LogSet of old versions", func() {
		// DO NOT mutate the following spec.
		// This spec is valid in mo-operator v0.6.0 and should always be accepted by
		// the webhook for backward compatibility.
		v06 := &LogSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "ls-" + randomString(5),
				Namespace: "default",
			},
			Spec: LogSetSpec{
				LogSetBasic: LogSetBasic{
					PodSet: PodSet{
						Replicas: 3,
						MainContainer: MainContainer{
							Image: "test",
						},
					},
					Volume: Volume{
						Size: resource.MustParse("10Gi"),
					},
					SharedStorage: SharedStorageProvider{
						S3: &S3Provider{Path: "test/data"},
					},
				},
			},
		}
		Expect(k8sClient.Create(context.TODO(), v06)).To(Succeed())
	})
})
