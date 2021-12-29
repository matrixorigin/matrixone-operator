package controllers

import (
	matrixonev1alpha1 "github.com/matrixorigin/matrixone-operator/api/v1"

	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var logger = logf.Log.WithName("matrixone_operator_handler")

func deployMatrixoneCluster(moc *matrixonev1alpha1.MatrixoneCluster) error {
	logger.Info("deployMatrixoneCluster")

	return nil
}
