// Copyright 2025 Matrix Origin
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

	"github.com/google/go-cmp/cmp"
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

func TestMergeDeep(t *testing.T) {
	tests := []struct {
		name     string
		base     map[string]interface{}
		override map[string]interface{}
		want     map[string]interface{}
	}{
		{
			name: "fileservice merge by name preserves user fields",
			base: map[string]interface{}{
				"fileservice": []map[string]interface{}{
					{
						"name":    "S3",
						"backend": "S3",
						"s3": map[string]interface{}{
							"parallel-mode": "1",
						},
					},
				},
			},
			override: map[string]interface{}{
				"data-dir": "/data/dir",
				"fileservice": []map[string]interface{}{
					{"name": "LOCAL", "backend": "DISK", "data-dir": "/data/dir"},
					{
						"name":    "S3",
						"backend": "S3",
						"s3": map[string]interface{}{
							"endpoint":   "s3.us-west-2.amazonaws.com",
							"bucket":     "my-bucket",
							"key-prefix": "prefix/data",
						},
						"cache": map[string]string{"memory-capacity": "1B"},
					},
					{
						"name":    "ETL",
						"backend": "S3",
						"s3": map[string]interface{}{
							"endpoint":   "s3.us-west-2.amazonaws.com",
							"bucket":     "my-bucket",
							"key-prefix": "prefix/etl",
						},
						"cache": map[string]string{"memory-capacity": "1B"},
					},
				},
			},
			want: map[string]interface{}{
				"data-dir": "/data/dir",
				"fileservice": []map[string]interface{}{
					{"name": "LOCAL", "backend": "DISK", "data-dir": "/data/dir"},
					{
						"name":    "S3",
						"backend": "S3",
						"s3": map[string]interface{}{
							"endpoint":      "s3.us-west-2.amazonaws.com",
							"bucket":        "my-bucket",
							"key-prefix":    "prefix/data",
							"parallel-mode": "1",
						},
						"cache": map[string]string{"memory-capacity": "1B"},
					},
					{
						"name":    "ETL",
						"backend": "S3",
						"s3": map[string]interface{}{
							"endpoint":   "s3.us-west-2.amazonaws.com",
							"bucket":     "my-bucket",
							"key-prefix": "prefix/etl",
						},
						"cache": map[string]string{"memory-capacity": "1B"},
					},
				},
			},
		},
		{
			name: "no user fileservice config",
			base: map[string]interface{}{
				"service-type": "CN",
			},
			override: map[string]interface{}{
				"data-dir": "/data/dir",
				"fileservice": []map[string]interface{}{
					{"name": "LOCAL", "backend": "DISK"},
				},
			},
			want: map[string]interface{}{
				"service-type": "CN",
				"data-dir":     "/data/dir",
				"fileservice": []map[string]interface{}{
					{"name": "LOCAL", "backend": "DISK"},
				},
			},
		},
		{
			name: "user adds extra fileservice entry",
			base: map[string]interface{}{
				"fileservice": []map[string]interface{}{
					{"name": "CUSTOM", "backend": "DISK", "data-dir": "/custom"},
				},
			},
			override: map[string]interface{}{
				"fileservice": []map[string]interface{}{
					{"name": "LOCAL", "backend": "DISK", "data-dir": "/data"},
				},
			},
			want: map[string]interface{}{
				"fileservice": []map[string]interface{}{
					{"name": "LOCAL", "backend": "DISK", "data-dir": "/data"},
					{"name": "CUSTOM", "backend": "DISK", "data-dir": "/custom"},
				},
			},
		},
		{
			name: "deep merge nested maps",
			base: map[string]interface{}{
				"section": map[string]interface{}{
					"user-key": "user-val",
					"nested": map[string]interface{}{
						"a": "1",
					},
				},
			},
			override: map[string]interface{}{
				"section": map[string]interface{}{
					"op-key": "op-val",
					"nested": map[string]interface{}{
						"b": "2",
					},
				},
			},
			want: map[string]interface{}{
				"section": map[string]interface{}{
					"user-key": "user-val",
					"op-key":   "op-val",
					"nested": map[string]interface{}{
						"a": "1",
						"b": "2",
					},
				},
			},
		},
		{
			name: "override wins on conflict",
			base: map[string]interface{}{
				"key": "user-value",
				"section": map[string]interface{}{
					"endpoint": "user-endpoint",
					"extra":    "kept",
				},
			},
			override: map[string]interface{}{
				"key": "operator-value",
				"section": map[string]interface{}{
					"endpoint": "operator-endpoint",
				},
			},
			want: map[string]interface{}{
				"key": "operator-value",
				"section": map[string]interface{}{
					"endpoint": "operator-endpoint",
					"extra":    "kept",
				},
			},
		},
		{
			name: "fileservice with []interface{} base from json unmarshal",
			base: map[string]interface{}{
				"fileservice": []interface{}{
					map[string]interface{}{
						"name":    "S3",
						"backend": "S3",
						"s3": map[string]interface{}{
							"parallel-mode": "1",
						},
					},
				},
			},
			override: map[string]interface{}{
				"fileservice": []map[string]interface{}{
					{
						"name":    "S3",
						"backend": "S3",
						"s3":     map[string]interface{}{"endpoint": "s3.amazonaws.com"},
					},
				},
			},
			want: map[string]interface{}{
				"fileservice": []map[string]interface{}{
					{
						"name":    "S3",
						"backend": "S3",
						"s3": map[string]interface{}{
							"endpoint":      "s3.amazonaws.com",
							"parallel-mode": "1",
						},
					},
				},
			},
		},
		{
			name:     "nil base",
			base:     nil,
			override: map[string]interface{}{"key": "val"},
			want:     map[string]interface{}{"key": "val"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewTomlConfig(tt.base)
			c.MergeDeep(tt.override)
			if diff := cmp.Diff(tt.want, c.MP); diff != "" {
				t.Errorf("MergeDeep() mismatch (-want +got):\n%s", diff)
			}
		})
	}
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
