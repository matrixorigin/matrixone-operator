package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type MatrixoneSpec struct {
	Foo string `json:"foo,omitempty"`
}
type MatrixoneStatus struct {
}

type Matrixone struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MatrixoneSpec   `json:"spec,omitempty"`
	Status MatrixoneStatus `json:"status,omitempty"`
}

type MatrixoneList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Matrixone `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Matrixone{}, &MatrixoneList{})
}
