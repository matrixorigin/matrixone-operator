// Copyright 2024 Matrix Origin
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

package v1alpha1

import (
	"github.com/blang/semver/v4"
	"testing"

	. "github.com/onsi/gomega"
)

func TestHasMOFeature(t *testing.T) {
	g := NewGomegaWithT(t)
	g.Expect(HasMOFeature(mustParse("1.1.2"), MOFeaturePipelineInfo)).To(BeTrue())
	g.Expect(HasMOFeature(mustParse("1.1.3"), MOFeaturePipelineInfo)).To(BeTrue())
	g.Expect(HasMOFeature(mustParse("1.2.0"), MOFeaturePipelineInfo)).To(BeTrue())
	g.Expect(HasMOFeature(mustParse("2.0.0"), MOFeaturePipelineInfo)).To(BeFalse())
	g.Expect(HasMOFeature(mustParse("1.1.1"), MOFeaturePipelineInfo)).To(BeFalse())
	g.Expect(HasMOFeature(mustParse("1.1.1"), MOFeaturePipelineInfo)).To(BeFalse())
	g.Expect(HasMOFeature(mustParse("1.1.2"), MOFeaturePipelineInfo)).To(BeTrue())
	g.Expect(HasMOFeature(mustParse("v1.1.1"), MOFeaturePipelineInfo)).To(BeFalse())
	g.Expect(HasMOFeature(mustParse("v1.1.2-rc1"), MOFeaturePipelineInfo)).To(BeTrue())
	g.Expect(HasMOFeature(mustParse("v1.2.0-alpha.1"), MOFeatureLockMigration)).To(BeTrue())
	featureVersions["dummy"] = []semver.Version{mustParse("1.2.3")}
	g.Expect(HasMOFeature(mustParse("v1.2.3"), "dummy")).To(BeTrue())
	g.Expect(HasMOFeature(mustParse("v1.3.0"), "dummy")).To(BeFalse())
}

func mustParse(s string) semver.Version {
	v, err := semver.ParseTolerant(s)
	if err != nil {
		panic(err)
	}
	return v
}
