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

package dnset

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_buildDNSetConfigMap(t *testing.T) {
	type args struct {
		dn *v1alpha1.DNSet
		ls *v1alpha1.LogSet
	}
	tests := []struct {
		name       string
		args       args
		wantConfig string
		wantErr    bool
	}{
		{
			name: "default",
			args: args{
				dn: &v1alpha1.DNSet{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "test",
						Name:      "test",
					},
				},
				ls: &v1alpha1.LogSet{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "test",
						Name:      "test",
					},
					Spec: v1alpha1.LogSetSpec{SharedStorage: v1alpha1.SharedStorageProvider{
						FileSystem: &v1alpha1.FileSystemProvider{
							Path: "/test",
						},
					}},
					Status: v1alpha1.LogSetStatus{
						Discovery: &v1alpha1.LogSetDiscovery{
							Port:    6001,
							Address: "test",
						},
					},
				},
			},
			wantConfig: `data-dir = "/var/lib/matrixone/data"
service-type = "DN"

[dn]
listen-address = "0.0.0.0:41010"
port-base = 41010

[dn.LogtailServer]
listen-address = "0.0.0.0:32003"

[dn.lockservice]
listen-address = "0.0.0.0:6003"

[[fileservice]]
backend = "DISK"
data-dir = "/var/lib/matrixone/data"
name = "LOCAL"

[[fileservice]]
backend = "DISK"
data-dir = "/test"
name = "S3"

[[fileservice]]
backend = "DISK-ETL"
data-dir = "/test"
name = "ETL"

[fileservice.cache]
memory-capacity = "1B"

[hakeeper-client]
service-addresses = []
`,
		},
		{
			name: "overrideEngineType",
			args: args{
				dn: &v1alpha1.DNSet{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "test",
						Name:      "test",
					},
					Spec: v1alpha1.DNSetSpec{PodSet: v1alpha1.PodSet{
						Config: &v1alpha1.TomlConfig{MP: map[string]interface{}{
							"cn": map[string]interface{}{
								"Engine": map[string]interface{}{
									"type": "distributed-tae",
								},
							},
						}},
					}},
				},
				ls: &v1alpha1.LogSet{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "test",
						Name:      "test",
					},
					Spec: v1alpha1.LogSetSpec{SharedStorage: v1alpha1.SharedStorageProvider{
						FileSystem: &v1alpha1.FileSystemProvider{
							Path: "/test",
						},
					}},
					Status: v1alpha1.LogSetStatus{
						Discovery: &v1alpha1.LogSetDiscovery{
							Port:    6001,
							Address: "test",
						},
					},
				},
			},
			wantConfig: `data-dir = "/var/lib/matrixone/data"
service-type = "DN"

[cn]

[cn.Engine]
type = "distributed-tae"

[dn]
listen-address = "0.0.0.0:41010"
port-base = 41010

[dn.LogtailServer]
listen-address = "0.0.0.0:32003"

[dn.lockservice]
listen-address = "0.0.0.0:6003"

[[fileservice]]
backend = "DISK"
data-dir = "/var/lib/matrixone/data"
name = "LOCAL"

[[fileservice]]
backend = "DISK"
data-dir = "/test"
name = "S3"

[[fileservice]]
backend = "DISK-ETL"
data-dir = "/test"
name = "ETL"

[fileservice.cache]
memory-capacity = "1B"

[hakeeper-client]
service-addresses = []
`,
		},
		{
			name: "2.0",
			args: args{
				dn: &v1alpha1.DNSet{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "test",
						Name:      "test",
					},
					Spec: v1alpha1.DNSetSpec{
						PodSet: v1alpha1.PodSet{
							MainContainer: v1alpha1.MainContainer{
								Image: "test:v2.0.0",
							},
						},
					},
				},
				ls: &v1alpha1.LogSet{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "test",
						Name:      "test",
					},
					Spec: v1alpha1.LogSetSpec{SharedStorage: v1alpha1.SharedStorageProvider{
						FileSystem: &v1alpha1.FileSystemProvider{
							Path: "/test",
						},
					}},
					Status: v1alpha1.LogSetStatus{
						Discovery: &v1alpha1.LogSetDiscovery{
							Port:    6001,
							Address: "test",
						},
					},
				},
			},
			wantConfig: `data-dir = "/var/lib/matrixone/data"
service-type = "DN"

[dn]
listen-address = "0.0.0.0:41010"
port-base = 41010

[dn.LogtailServer]
listen-address = "0.0.0.0:32003"

[dn.lockservice]
listen-address = "0.0.0.0:6003"

[[fileservice]]
backend = "DISK"
data-dir = "/var/lib/matrixone/data"
name = "LOCAL"

[[fileservice]]
backend = "DISK"
data-dir = "/test"
name = "S3"

[[fileservice]]
backend = "DISK-ETL"
data-dir = "/test"
name = "ETL"

[fileservice.cache]
memory-capacity = "1B"

[hakeeper-client]
discovery-address = "test:6001"
`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewGomegaWithT(t)
			got, configSuffix, err := buildDNSetConfigMap(tt.args.dn, tt.args.ls)
			if (err != nil) != tt.wantErr {
				t.Errorf("buildDNSetConfigMap() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			configKey := "config.toml-" + configSuffix
			g.Expect(got.Data[configKey]).NotTo(BeNil())
			g.Expect(cmp.Diff(tt.wantConfig, got.Data[configKey])).To(BeEmpty())
		})
	}
}
