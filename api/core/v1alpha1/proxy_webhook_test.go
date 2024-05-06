// Copyright 2024 Matrix Origin
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package v1alpha1

import (
	"context"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ProxySet Webhook", func() {

	It("should hook proxyset", func() {
		ps := &ProxySet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "proxy-" + randomString(5),
				Namespace: "default",
			},
			Spec: ProxySetSpec{
				PodSet: PodSet{
					Replicas: 2,
					MainContainer: MainContainer{
						Image: "test",
					},
				},
			},
			Deps: ProxySetDeps{
				LogSetRef: LogSetRef{
					ExternalLogSet: &ExternalLogSet{},
				},
			},
		}
		Expect(k8sClient.Create(context.TODO(), ps)).To(Succeed())
	})
})
