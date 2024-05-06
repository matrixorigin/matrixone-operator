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
