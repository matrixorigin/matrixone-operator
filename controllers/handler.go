package controllers

import (
	matrixonev1alpha1 "github.com/matrixorigin/matrixone-operator/api/v1alpha1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var logger = logf.Log.WithName("matrixone_operator_handler")

func deployMatrixoneCluster(sdk client.Client, m *matrixonev1alpha1.MatrixoneCluster, emitEvents EventEmitter) error {
	return nil
}

func namespacedName(name, namespace string) *types.NamespacedName {
	return &types.NamespacedName{Name: name, Namespace: namespace}
}
