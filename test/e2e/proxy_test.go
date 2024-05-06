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

package e2e

import (
	"fmt"

	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Proxy test", func() {
	It("Should set proxy service args defaults", func() {
		ps := &v1alpha1.ProxySet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "proxy-" + rand.String(6),
				Namespace: "default",
			},
			Spec: v1alpha1.ProxySetSpec{
				PodSet: v1alpha1.PodSet{
					Replicas: 2,
					MainContainer: v1alpha1.MainContainer{
						Image: fmt.Sprintf("%s:%s", moImageRepo, moVersion),
					},
				},
			},
			Deps: v1alpha1.ProxySetDeps{
				LogSetRef: v1alpha1.LogSetRef{
					LogSet: &v1alpha1.LogSet{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "non-exist",
							Namespace: env.Namespace,
						},
					},
				},
			},
		}
		Expect(kubeCli.Create(ctx, ps)).To(Succeed())
		Expect(kubeCli.Get(ctx, client.ObjectKeyFromObject(ps), ps)).To(Succeed())
		Expect(ps.Spec.ServiceArgs).NotTo(BeNil(), "default service args should be set")
		Expect(kubeCli.Delete(ctx, ps)).To(Succeed())
	})
})
