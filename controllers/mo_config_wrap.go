package controllers

import (
	"bytes"

	"github.com/BurntSushi/toml"
)

func MarshalTOML(config MatrixoneConfig) (string, error) {

	buff := new(bytes.Buffer)
	encoder := toml.NewEncoder(buff)
	err := encoder.Encode(config)
	if err != nil {
		return "", err
	}

	return string(buff.Bytes()), nil
}
