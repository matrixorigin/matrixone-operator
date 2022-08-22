// Copyright 2022 Matrix Origin
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

package dnset

import (
	"fmt"
	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	"github.com/matrixorigin/matrixone-operator/pkg/controllers/common"
)

const (
	nameSuffix = "-dn"
)

func getListenAddress() string {
	return fmt.Sprintf("%s:%d", common.ListenAddress, common.DNServicePort)
}

func configMapName(dn *v1alpha1.DNSet) string {
	return resourceName(dn) + "-config"
}

func stsName(dn *v1alpha1.DNSet) string {
	return resourceName(dn)
}

func headlessSvcName(dn *v1alpha1.DNSet) string {
	return resourceName(dn) + "-headless"
}

func resourceName(dn *v1alpha1.DNSet) string {
	return dn.Name + nameSuffix
}
