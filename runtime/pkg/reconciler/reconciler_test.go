// Copyright 2022 Matrix Origin
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
