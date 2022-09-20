// Copyright 2022 Matrix Origin
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
package reconciler

import (
	"fmt"
	"time"
)

type ReSync struct {
	Message      string
	RequeueAfter time.Duration
}

func (e *ReSync) Error() string {
	return fmt.Sprintf("reconcile error: %s, requeue after %s", e.Message, e.RequeueAfter)
}

func ErrReSync(msg string, requeueAfter ...time.Duration) *ReSync {
	e := &ReSync{
		Message: msg,
	}
	if len(requeueAfter) > 0 {
		e.RequeueAfter = requeueAfter[0]
	}
	return e
}
