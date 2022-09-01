package webui

import "github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"

func webUIName(wi *v1alpha1.WebUI) string {
	return wi.Name + objSuffix
}
