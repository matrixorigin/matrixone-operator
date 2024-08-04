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

package bucketclaim

import (
	corev1 "k8s.io/api/core/v1"
)

type Option func(actor *Actor)

func WithImage(image string) Option {
	return func(actor *Actor) {
		actor.image = image
	}
}

func WithImagePullSecrets(secrets []corev1.LocalObjectReference) Option {
	return func(actor *Actor) {
		if actor.imagePullSecrets == nil {
			actor.imagePullSecrets = make([]corev1.LocalObjectReference, 0, len(secrets))
		}
		actor.imagePullSecrets = append(actor.imagePullSecrets, secrets...)
	}
}
