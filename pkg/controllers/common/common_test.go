// Copyright 2022 Matrix Origin
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
