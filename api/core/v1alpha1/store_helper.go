// Copyright 2025 Matrix Origin
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

const (
	// StoreDrainingStartAnno is the annotation key that used to record the store draining start time
	StoreDrainingStartAnno = "matrixorigin.io/store-draining-start"

	// StoreConnectionAnno expose the connection count of the store
	StoreConnectionAnno = "matrixorigin.io/connections"

	// StoreScoreAnno expose the score of the store
	StoreScoreAnno = "matrixorigin.io/score"

	// StoreCordonAnno cordons a CN store
	StoreCordonAnno = "matrixorigin.io/store-cordon"
)
