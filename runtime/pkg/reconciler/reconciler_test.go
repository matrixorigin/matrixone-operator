package reconciler

import (
	"testing"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestSetupObjectFactory(t *testing.T) {
	g := NewGomegaWithT(t)
	r := &Reconciler[*corev1.Service]{}
	s := runtime.NewScheme()
	_ = corev1.AddToScheme(s)
	g.Expect(r.setupObjectFactory(s, &corev1.Service{})).To(Succeed())
	g.Expect(r.newT()).ToNot(BeNil())
}
