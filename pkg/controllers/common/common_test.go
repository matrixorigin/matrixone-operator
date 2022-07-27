package common

import (
	"testing"

	. "github.com/onsi/gomega"
	"golang.org/x/exp/utf8string"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestAddConfigMapDigest(t *testing.T) {
	// need fuzz?
	cmList := []*corev1.ConfigMap{
		newCM(""),
		newCM(" "),
		newCM("hello world"),
		newCM("你好世界"),
		newCM("こんにちは世界"),
	}
	g := NewGomegaWithT(t)
	for _, cm := range cmList {
		g.Expect(addConfigMapDigest(cm)).To(Succeed())
		g.Expect(utf8string.NewString(cm.Name).IsASCII()).To(BeTrue())
	}
}

func newCM(data string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "foo"},
		Data: map[string]string{
			"config": data,
		},
	}
}
