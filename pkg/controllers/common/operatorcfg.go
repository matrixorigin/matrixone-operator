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

package common

import (
	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	"k8s.io/apimachinery/pkg/util/yaml"
	"os"
	"path"
	"strings"
)

// OperatorConfig includes configurations for this operator process
type OperatorConfig struct {
	DefaultArgs  *v1alpha1.DefaultArgs `json:"defaultArgs,omitempty" yaml:"defaultArgs,omitempty"`
	FeatureGates map[string]bool       `json:"featureGates,omitempty" yaml:"featureGates,omitempty"`
}

// LoadOperatorConfig read all operator configurations from configmap mount path, and load it into OperatorConfig struct
func LoadOperatorConfig(cfgPath string, config *OperatorConfig) error {
	entries, err := os.ReadDir(cfgPath)
	if err != nil {
		return err
	}
	data := make(map[string]string)
	for _, e := range entries {
		if e.IsDir() || e.Name() == "..data" {
			continue
		}
		content, err := os.ReadFile(path.Join(cfgPath, e.Name()))
		if err != nil {
			return err
		}
		data[e.Name()] = string(content)
	}

	rawYaml := ""
	for field, value := range data {
		rawYaml += field + ": \n"
		rawYaml += insertSpaces(value)
	}
	return yaml.Unmarshal([]byte(rawYaml), config)
}

func insertSpaces(value string) string {
	spaced := ""
	for _, line := range strings.Split(value, "\n") {
		spaced += "  "
		spaced += line
		spaced += "\n"
	}
	return spaced
}
