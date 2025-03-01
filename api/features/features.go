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

package features

import (
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/component-base/featuregate"
)

var (
	// DefaultMutableFeatureGate is a mutable version of DefaultFeatureGate.
	// Only top-level commands/options setup and the k8s.io/component-base/featuregate/testing package should make use of this.
	// Tests that need to modify feature gates for the duration of their test should use:
	//   defer featuregatetesting.SetFeatureGateDuringTest(t, utilfeature.DefaultFeatureGate, features.<FeatureName>, <value>)()
	DefaultMutableFeatureGate featuregate.MutableFeatureGate = featuregate.NewFeatureGate()

	// DefaultFeatureGate is a shared global FeatureGate.
	// Top-level commands/options setup that needs to modify this feature gate should use DefaultMutableFeatureGate.
	DefaultFeatureGate featuregate.FeatureGate = DefaultMutableFeatureGate
)

const (
	S3Reclaim    featuregate.Feature = "s3Reclaim"
	ProxySupport featuregate.Feature = "proxySupport"

	BRSupport featuregate.Feature = "backupRestore"

	CNLabel featuregate.Feature = "cnLabel"

	CNPool featuregate.Feature = "cnPool"
)

var defaultFeatureGates = map[featuregate.Feature]featuregate.FeatureSpec{
	S3Reclaim:    {Default: false, PreRelease: featuregate.Alpha},
	ProxySupport: {Default: false, PreRelease: featuregate.Alpha},
	CNLabel:      {Default: false, PreRelease: featuregate.Alpha},
	BRSupport:    {Default: false, PreRelease: featuregate.Alpha},
	CNPool:       {Default: false, PreRelease: featuregate.Alpha},
}

func init() {
	runtime.Must(DefaultMutableFeatureGate.Add(defaultFeatureGates))
}
