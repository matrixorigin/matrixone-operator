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
package v1alpha1

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/mohae/deepcopy"
	"github.com/pelletier/go-toml"
	"github.com/pkg/errors"
)

// TomlConfig is an auxiliary struct that serialize a nested struct to raw string
// in toml format on serialization and vise-versa
// +kubebuilder:validation:Type=string
type TomlConfig struct {
	MP map[string]interface{} `json:"-"`
}

func (c *TomlConfig) ToString() (string, error) {
	s, err := c.MarshalTOML()
	if err != nil {
		return "", err
	}
	return string(s), nil
}

// Get a nested key, if no key is provided, the root nested map will be returned,
// returns nil if the path is not found.
func (c *TomlConfig) Get(path ...string) (value *Value) {
	if c.MP == nil {
		return nil
	}
	v := get(c.MP, path...)
	if v == nil {
		return nil
	}

	return &Value{inner: v}
}

// Del delete a key by path, ignore not found.
// key1 + keyN forms the full key path, which enforce
// the keyPath has at least depth 1
func (c *TomlConfig) Del(key1 string, keyN ...string) {
	if c.MP == nil {
		return
	}
	path := keyPath(key1, keyN...)
	// get the parent of the key to be deleted
	parent := get(c.MP, path[0:len(path)-1]...)
	if parent == nil {
		return
	}
	parentM, ok := parent.(map[string]interface{})
	if !ok {
		// parent is itself a simple kv, which means the full keyPath must not exist
		return
	}
	delete(parentM, path[len(path)-1])
}

// Set a key by path, override existing.
// the keyPath has at least depth 1
func (c *TomlConfig) Set(path []string, value interface{}) {
	if c.MP == nil {
		c.MP = map[string]interface{}{}
	}
	set(c.MP, value, path...)
}

func keyPath(key1 string, keyN ...string) []string {
	var keys []string
	keys = append(keys, key1)
	return append(keys, keyN...)
}

func get(m map[string]interface{}, path ...string) interface{} {
	if len(path) == 0 {
		return m
	}
	if len(path) == 1 {
		return m[path[0]]
	}
	next, ok := m[path[0]]
	if !ok {
		return nil
	}
	nextM, ok := next.(map[string]interface{})
	if !ok {
		return nil
	}
	return get(nextM, path[1:]...)
}

func set(m map[string]interface{}, value interface{}, path ...string) {
	if len(path) == 0 {
		return
	}
	if len(path) == 1 {
		m[path[0]] = value
		return
	}
	next, ok := m[path[0]]
	if ok {
		nextM, ok := next.(map[string]interface{})
		if ok {
			set(nextM, value, path[1:]...)
			return
		}
	}
	// if the next key is not found or the next key is not a nested map,
	// override it with nested map
	m[path[0]] = map[string]interface{}{}
	set(m[path[0]].(map[string]interface{}), value, path[1:]...)
	return
}

// +kubebuilder:object:root=false
// +kubebuilder:object:generate=false
type Value struct {
	inner interface{}
}

func (v *Value) Interface() interface{} {
	if v == nil {
		return nil
	}
	return v.inner
}

func (v *Value) MustString() string {
	value, err := v.AsString()
	if err != nil {
		panic(err)
	}
	return value
}

func (v *Value) AsString() (string, error) {
	s, ok := (v.inner).(string)
	if !ok {
		return "", errors.Errorf("type is %v not string", reflect.TypeOf(v.inner))
	}
	return s, nil
}

func (v *Value) MustInt() int64 {
	value, err := v.AsInt()
	if err != nil {
		panic(err)
	}
	return value
}

func (v *Value) AsInt() (int64, error) {
	switch value := v.inner.(type) {
	case int:
		return int64(value), nil
	case int8:
		return int64(value), nil
	case int16:
		return int64(value), nil
	case int32:
		return int64(value), nil
	case int64:
		return value, nil
	case uint:
		return int64(value), nil
	case uint8:
		return int64(value), nil
	case uint16:
		return int64(value), nil
	case uint32:
		return int64(value), nil
	case uint64:
		return int64(value), nil

	default:
		return 0, errors.Errorf("type is %v not integer", reflect.TypeOf(v.inner))
	}
}

func (v *Value) MustStringSlice() []string {
	value, err := v.AsStringSlice()
	if err != nil {
		panic(err)
	}
	return value
}

func (v *Value) AsStringSlice() ([]string, error) {
	switch s := v.inner.(type) {
	case []string:
		return s, nil
	case []interface{}:
		var slice []string
		for _, item := range s {
			str, ok := item.(string)
			if !ok {
				return nil, errors.Errorf("can not be string slice: %v", v.inner)
			}
			slice = append(slice, str)
		}
		return slice, nil
	default:
		return nil, errors.Errorf("invalid type: %v", reflect.TypeOf(v.inner))
	}
}

func (v *Value) MustToml() *TomlConfig {
	m, err := v.AsToml()
	if err != nil {
		panic(err)
	}
	return m
}

func (v *Value) AsToml() (*TomlConfig, error) {
	m, ok := (v.inner).(map[string]interface{})
	if !ok {
		return nil, errors.Errorf("type is %v not map", reflect.TypeOf(v.inner))
	}
	return NewTomlConfig(m), nil
}

func (c *TomlConfig) MarshalTOML() ([]byte, error) {
	if c == nil {
		return nil, nil
	}

	buff := new(bytes.Buffer)
	encoder := toml.NewEncoder(buff)
	encoder.Indentation("")
	encoder.Order(toml.OrderAlphabetical)
	err := encoder.Encode(c.MP)
	if err != nil {
		return nil, err
	}

	return buff.Bytes(), nil
}

func (c *TomlConfig) UnmarshalTOML(data []byte) error {
	return toml.Unmarshal(data, &c.MP)
}

func (c *TomlConfig) MarshalJSON() ([]byte, error) {
	toml, err := c.MarshalTOML()
	if err != nil {
		return nil, err
	}

	return json.Marshal(string(toml))
}

func (c *TomlConfig) UnmarshalJSON(data []byte) error {
	var value interface{}
	err := json.Unmarshal(data, &value)
	if err != nil {
		return err
	}

	switch s := value.(type) {
	case string:
		err = toml.Unmarshal([]byte(s), &c.MP)
		if err != nil {
			return err
		}
		return nil
	case map[string]interface{}:
		// If v is a *map[string]interface{}, numbers are converted to int64 or float64
		// using s directly all numbers are float type of go
		// Keep the behavior Unmarshal *map[string]interface{} directly we unmarshal again here.
		err = json.Unmarshal(data, &c.MP)
		if err != nil {
			return err
		}
		return nil
	default:
		return fmt.Errorf("unknown type: %v", reflect.TypeOf(value))
	}
}

// deepcopy-gen cannot
func (c *TomlConfig) DeepCopyJsonObject() *TomlConfig {
	if c == nil {
		return nil
	}
	if c.MP == nil {
		return NewTomlConfig(nil)
	}
	MP := deepcopy.Copy(c.MP).(map[string]interface{})
	return NewTomlConfig(MP)
}

func (c *TomlConfig) DeepCopy() *TomlConfig {
	return c.DeepCopyJsonObject()
}

func (c *TomlConfig) DeepCopyInto(out *TomlConfig) {
	*out = *c
	out.MP = c.DeepCopyJsonObject().MP
}

func NewTomlConfig(o map[string]interface{}) *TomlConfig {
	return &TomlConfig{o}
}

var _ json.Marshaler = &TomlConfig{}
var _ json.Unmarshaler = &TomlConfig{}
