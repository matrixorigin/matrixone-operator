// Copyright 2026 Matrix Origin
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package bucketclaim

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
)

func TestCleanupTargetValidation(t *testing.T) {
	for _, tt := range []struct {
		name string
		s3   *v1alpha1.S3Provider
	}{
		{"empty", &v1alpha1.S3Provider{}},
		{"missing bucket", &v1alpha1.S3Provider{Path: "/prefix"}},
		{"only slashes", &v1alpha1.S3Provider{Path: "///"}},
		{"URL instead of bucket path", &v1alpha1.S3Provider{Path: "s3://bucket/prefix"}},
		{"dot bucket", &v1alpha1.S3Provider{Path: "."}},
		{"dotdot bucket", &v1alpha1.S3Provider{Path: ".."}},
		{"whitespace bucket", &v1alpha1.S3Provider{Path: " bucket/prefix"}},
		{"NUL key", &v1alpha1.S3Provider{Path: "bucket/key\x00suffix"}},
		{"NUL endpoint", &v1alpha1.S3Provider{Path: "bucket/key", Endpoint: "https://example.test\x00"}},
		{"NUL region", &v1alpha1.S3Provider{Path: "bucket/key", Region: "us-east-1\x00"}},
		{"nil", nil},
	} {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("must reject input without panic: %v", r)
				}
			}()
			if script, err := parseEntrypoint(tt.s3); err == nil || script != "" {
				t.Errorf("invalid target generated script: error=%v script=%q", err, script)
			}
		})
	}
}

func TestCleanupArgumentsRemainLiteral(t *testing.T) {
	for _, tt := range []struct{ name, path, endpoint, region string }{
		{"AWS bucket root", "valid-bucket", "", "us-east-1"},
		{"MinIO legacy bucket", "Legacy_bucket/key", "http://minio:9000", "us-east-1"},
		{"OSS endpoint", "test-bucket/prefix/", "https://oss-cn-hangzhou.aliyuncs.com", "cn-hangzhou"},
		{"literal key bytes", "test-bucket/space % quoted'\" $(printf COMPROMISED);\n//../你好", "https://storage.example.test", "us-east-1"},
		{"literal endpoint and region", "test-bucket/key", "https://example.test/quote'\"?x=$(printf COMPROMISED); value", "region'\" $(printf COMPROMISED); value"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			argsFile, regionFile := filepath.Join(dir, "argv"), filepath.Join(dir, "region")
			aws := `#!/bin/sh
printf '%s\000' "$@" >> "$MAP2_ARGV"
printf '%s' "$AWS_REGION" > "$MAP2_REGION"
`
			if err := os.WriteFile(filepath.Join(dir, "aws"), []byte(aws), 0700); err != nil {
				t.Fatal(err)
			}
			script, err := parseEntrypoint(&v1alpha1.S3Provider{Path: tt.path, Endpoint: tt.endpoint, Region: tt.region})
			if err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command("/bin/sh")
			cmd.Stdin = strings.NewReader(script)
			cmd.Env = []string{"PATH=" + dir, "MAP2_ARGV=" + argsFile, "MAP2_REGION=" + regionFile}
			if output, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("script failed: %v: %s", err, output)
			}
			var want []string
			if tt.endpoint != "" {
				want = append(want, "--endpoint-url", tt.endpoint)
			}
			want = append(want, "s3api", "head-bucket", "--bucket", strings.SplitN(tt.path, "/", 2)[0])
			if tt.endpoint != "" {
				want = append(want, "--endpoint-url", tt.endpoint)
			}
			want = append(want, "s3", "rm", "s3://"+tt.path, "--recursive")
			got, err := os.ReadFile(argsFile)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, []byte(strings.Join(want, "\x00")+"\x00")) {
				t.Errorf("argv bytes=%q, want %q", got, want)
			}
			got, err = os.ReadFile(regionFile)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != tt.region {
				t.Errorf("region=%q want %q", got, tt.region)
			}
		})
	}
}

func TestCleanupScriptFailsClosed(t *testing.T) {
	for _, endpoint := range []string{"", "https://storage.example.test"} {
		for _, tt := range []struct {
			name         string
			head, remove int
		}{
			{"success", 0, 0},
			{"head denied or absent", 254, 0},
			{"head network error", 255, 0},
			{"head unknown failure", 1, 0},
			{"delete failure", 0, 1},
		} {
			t.Run(endpoint+"/"+tt.name, func(t *testing.T) {
				dir := t.TempDir()
				calls := filepath.Join(dir, "calls")
				// Execute the real generated script. The executable logs the arguments
				// and returns controlled errors, without reading credentials or networking.
				aws := `#!/bin/sh
printf '%s\n' "$*" >> "$MAP2_CALLS"
if [ "$1" = "--endpoint-url" ]; then shift 2; fi
case "$1 $2" in
  "s3api head-bucket") exit "$MAP2_HEAD_EXIT" ;;
  "s3 rm") exit "$MAP2_REMOVE_EXIT" ;;
  *) exit 99 ;;
esac
`
				if err := os.WriteFile(filepath.Join(dir, "aws"), []byte(aws), 0700); err != nil {
					t.Fatal(err)
				}
				script, err := parseEntrypoint(&v1alpha1.S3Provider{Path: "test-bucket/tenant-prefix", Region: "us-east-1", Endpoint: endpoint})
				if err != nil {
					t.Fatal(err)
				}
				cmd := exec.Command("/bin/sh")
				cmd.Stdin = strings.NewReader(script)
				cmd.Env = []string{"PATH=" + dir, "MAP2_CALLS=" + calls, "MAP2_HEAD_EXIT=" + strconv.Itoa(tt.head), "MAP2_REMOVE_EXIT=" + strconv.Itoa(tt.remove)}
				output, err := cmd.CombinedOutput()
				wantFailure := tt.head != 0 || tt.remove != 0
				if (err != nil) != wantFailure {
					t.Errorf("error=%v, want failure=%v; output: %s", err, wantFailure, output)
				}
				got, err := os.ReadFile(calls)
				if err != nil {
					t.Fatal(err)
				}
				prefix := ""
				if endpoint != "" {
					prefix = "--endpoint-url " + endpoint + " "
				}
				want := prefix + "s3api head-bucket --bucket test-bucket\n"
				if tt.head == 0 {
					want += prefix + "s3 rm s3://test-bucket/tenant-prefix --recursive\n"
				}
				if string(got) != want {
					t.Errorf("calls=%q, want %q", got, want)
				}
			})
		}
	}
}
