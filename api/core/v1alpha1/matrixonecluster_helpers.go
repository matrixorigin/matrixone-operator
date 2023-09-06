// Copyright 2023 Matrix Origin
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

import "fmt"

func (m *MatrixOneCluster) LogSetImage() string {
	image := m.Spec.LogService.Image
	if image == "" {
		image = m.DefaultImage()
	}
	return image
}

func (m *MatrixOneCluster) DnSetImage() string {
	image := m.GetTN().Image
	if image == "" {
		image = m.DefaultImage()
	}
	return image
}

func (m *MatrixOneCluster) ProxySetImage() string {
	if m.Spec.Proxy == nil {
		return ""
	}
	image := m.Spec.Proxy.Image
	if image == "" {
		image = m.DefaultImage()
	}
	return image
}

func (m *MatrixOneCluster) DefaultImage() string {
	return fmt.Sprintf("%s:%s", m.Spec.ImageRepository, m.Spec.Version)
}
