// Copyright 2024 Matrix Origin
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
package common

import (
	"github.com/matrixorigin/matrixone-operator/pkg/utils"
	"testing"

	. "github.com/onsi/gomega"
)

type Version1 struct {
	A *string `json:"a,omitempty"`
	B *string `json:"b,omitempty"`
}

type Version2 struct {
	A *string `json:"a,omitempty"`
	B *string `json:"b,omitempty"`
	C *string `json:"c,omitempty"`
}

func TestHashControllerRevision(t *testing.T) {
	g := NewGomegaWithT(t)
	hash1, err := HashControllerRevision(&Version1{
		A: utils.PtrTo("a"),
		B: utils.PtrTo("b"),
	})
	g.Expect(err).To(BeNil())
	hash2, err := HashControllerRevision(&Version2{
		A: utils.PtrTo("a"),
		B: utils.PtrTo("b"),
	})
	g.Expect(err).To(BeNil())
	g.Expect(hash1).To(Equal(hash2))
}
