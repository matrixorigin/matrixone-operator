package utils

import (
	"os"
	"reflect"
	"strings"
)

func FirstNonEmptyStr(s1 string, s2 string) string {
	if len(s1) > 0 {
		return s1
	} else {
		return s2
	}
}

// Note that all the arguments passed to this function must have zero value of Nil.
func FirstNonNilValue(v1, v2 interface{}) interface{} {
	if !reflect.ValueOf(v1).IsNil() {
		return v1
	} else {
		return v2
	}
}

// lookup DENY_LIST, default is nil
func GetDenyListEnv(key string, defaultVal string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultVal
}

// pass slice of strings for namespaces
func GetEnvAsSlice(name string, defaultVal []string, sep string) []string {
	valStr := GetDenyListEnv(name, "")
	if valStr == "" {
		return defaultVal
	}
	// split on ","
	val := strings.Split(valStr, sep)
	return val
}

func ContainsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

func RemoveString(slice []string, s string) (result []string) {
	for _, item := range slice {
		if item == s {
			continue
		}
		result = append(result, item)
	}
	return
}

// returns pointer to bool
func BoolFalse() *bool {
	bool := false
	return &bool
}
