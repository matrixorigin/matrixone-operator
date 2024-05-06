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
