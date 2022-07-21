package dnset

import (
	kruisev1alpha1 "github.com/openkruise/kruise/apis/apps/v1alpha1"
)

var (
	StatefulSet     = &kruisev1alpha1.StatefulSet{}
	StatefulSetList = &kruisev1alpha1.CloneSetList{}
	CloneSet        = &kruisev1alpha1.CloneSet{}
	CloneSetList    = &kruisev1alpha1.CloneSetList{}
	KruiseAddToScheme = kruisev1alpha1.AddToScheme
)
