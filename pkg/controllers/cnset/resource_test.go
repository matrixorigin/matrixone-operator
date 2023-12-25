// Copyright 2023 Matrix Origin
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

package cnset

import (
	"github.com/google/go-cmp/cmp"
	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"testing"
)

func Test_buildCNSetConfigMap(t *testing.T) {
	type args struct {
		cn *v1alpha1.CNSet
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
				cn: &v1alpha1.CNSet{
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
service-type = "CN"

[cn]
port-base = 6002
role = ""

[cn.lockservice]
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
			name: "store-drain-enabled",
			args: args{
				cn: &v1alpha1.CNSet{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "test",
						Name:      "test",
					},
					Spec: v1alpha1.CNSetSpec{
						ScalingConfig: v1alpha1.ScalingConfig{
							StoreDrainEnabled: pointer.Bool(true),
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
service-type = "CN"

[cn]
init-work-state = "Draining"
port-base = 6002
role = ""

[cn.lockservice]
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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewGomegaWithT(t)
			got, err := buildCNSetConfigMap(tt.args.cn, tt.args.ls)
			if (err != nil) != tt.wantErr {
				t.Errorf("buildDNSetConfigMap() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			g.Expect(got.Data["config.toml"]).NotTo(BeNil())
			g.Expect(cmp.Diff(tt.wantConfig, got.Data["config.toml"])).To(BeEmpty())
		})
	}
}
