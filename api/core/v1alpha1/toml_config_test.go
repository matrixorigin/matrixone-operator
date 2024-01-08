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

package v1alpha1

import (
	"testing"

	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/util/json"
)

func TestDeepCopy(t *testing.T) {
	c := NewTomlConfig(map[string]interface{}{
		"id":   1,
		"name": "Alice",
		"pets": []string{"Dog", "Cat"},
		"profile": map[string]interface{}{
			"k1": "v1",
			"nested": map[string]interface{}{
				"k2": 2,
			},
		},
	})
	copied := c.DeepCopy()
	g := NewGomegaWithT(t)
	g.Expect(copied).To(Equal(c))
}

func TestOperation(t *testing.T) {
	g := NewGomegaWithT(t)
	c := NewTomlConfig(map[string]interface{}{
		"id":   1,
		"name": "Alice",
		"pets": []string{"Dog", "Cat"},
		"profile": map[string]interface{}{
			"k1": "v1",
			"nested": map[string]interface{}{
				"k2": 2,
			},
		},
	})

	g.Expect(c.Get("id").MustInt()).Should(Equal(int64(1)))
	g.Expect(c.Get("name").MustString()).Should(Equal("Alice"))
	g.Expect(c.Get("pets").MustStringSlice()).Should(Equal([]string{"Dog", "Cat"}))
	g.Expect(c.Get("profile", "k1").MustString()).Should(Equal("v1"))

	c.Del("pets")
	g.Expect(c.Get("pets")).To(BeNil())

	c.Set([]string{"profile", "nested", "k2"}, 3)
	g.Expect(c.Get("profile", "nested", "k2").MustInt()).Should(Equal(int64(3)))

	c.Set([]string{"k3", "k4"}, "v4")
	g.Expect(c.Get("k3", "k4").MustString()).Should(Equal("v4"))
	c.Set([]string{"k3", "k4"}, "v5")
	g.Expect(c.Get("k3", "k4").MustString()).Should(Equal("v5"))
	c.Set([]string{"k3"}, "v6")
	g.Expect(c.Get("k3", "k4")).Should(BeNil())

	c.Set([]string{"profile", "nested", "k1", "k3"}, "v4")
	g.Expect(c.Get("profile", "nested", "k2")).ShouldNot(BeNil(), "set nested fields must not override parent map")

	profile := c.Get("profile").MustToml()
	profile.Del("nested")
	g.Expect(c.Get("profile", "nested", "k2")).Should(BeNil())
	g.Expect(c.Get("profile", "nested")).Should(BeNil())
}

func TestMarshal(t *testing.T) {
	type S struct {
		Config *TomlConfig `json:"config,omitempty"`
	}

	g := NewGomegaWithT(t)

	s := &S{
		Config: NewTomlConfig(map[string]interface{}{}),
	}
	s.Config.Set([]string{"sk"}, "v")

	data, err := json.Marshal(s)
	g.Expect(err).Should(BeNil())

	sback := new(S)
	err = json.Unmarshal(data, sback)
	g.Expect(err).Should(BeNil())
	g.Expect(sback).Should(Equal(s))

	// test object type
	data, err = json.Marshal(map[string]interface{}{
		"config": s.Config.MP,
	})
	g.Expect(err).Should(BeNil())

	sback = new(S)
	err = json.Unmarshal(data, sback)
	g.Expect(err).Should(BeNil())
	g.Expect(sback).Should(Equal(s))
}

func TestOmitEmpty(t *testing.T) {
	type S struct {
		Config *TomlConfig `json:"config,omitempty"`
	}

	g := NewGomegaWithT(t)

	s := new(S)
	err := json.Unmarshal([]byte("{}"), s)
	g.Expect(err).Should(BeNil())
	g.Expect(s.Config).Should(BeNil())
	data, err := json.Marshal(s)
	g.Expect(err).Should(BeNil())
	s = new(S)
	err = json.Unmarshal(data, s)
	g.Expect(err).Should(BeNil())
	g.Expect(s.Config).Should(BeNil())

	// test Config should not be nil
	s = new(S)
	err = json.Unmarshal([]byte("{\"config\":\"a = 1\"}"), s)
	g.Expect(err).Should(BeNil())
	g.Expect(s.Config).ShouldNot(BeNil())
	data, err = json.Marshal(s)
	g.Expect(err).Should(BeNil())
	s = new(S)
	err = json.Unmarshal(data, s)
	g.Expect(err).Should(BeNil())
	g.Expect(s.Config).ShouldNot(BeNil())
}
