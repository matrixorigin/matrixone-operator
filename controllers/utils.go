package controllers

import "reflect"

func firstNonNilValue(v1, v2 interface{}) interface{} {
	if !reflect.ValueOf(v1).IsNil() {
		return v1
	} else {
		return v2
	}
}
