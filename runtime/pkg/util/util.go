package util

import (
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func Ignore(isErr func(error) bool, err error) error {
	if isErr(err) {
		return nil
	}
	return err
}

func WasDeleted(obj client.Object) bool {
	return obj.GetDeletionTimestamp() != nil
}