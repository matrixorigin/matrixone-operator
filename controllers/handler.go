package controllers

import (
	"encoding/json"
	"fmt"

	matrixonev1alpha1 "github.com/matrixorigin/matrixone-operator/api/v1"
	v1alpha1 "github.com/matrixorigin/matrixone-operator/api/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var logger = logf.Log.WithName("matrixone_operator_handler")

func deployMatrixoneCluster(sdk client.Client, moc *matrixonev1alpha1.MatrixoneCluster, emitEvents EventEmitter) error {
	klog.Info("deployMatrixoneCluster")

	if err := verifyMatrixoneSpec(moc); err != nil {
		e := fmt.Errorf("invalid MatrixoneSpec[%s:%s] due to [%s]", moc.Kind, moc.Name, err.Error())
		emitEvents.EmitEventGeneric(moc, "MatrixoneOperatorInvalidSpec", "", e)
		return nil
	}

	return nil
}

func verifyMatrixoneSpec(moc *matrixonev1alpha1.MatrixoneCluster) error {
	errorMsg := ""

	if moc.Spec.Image == "" {
		errorMsg = fmt.Sprintf("%sImage missing from Matrixone Cluster Spec\n", errorMsg)
	}

	if moc.Spec.Replicas < 1 {
		errorMsg = fmt.Sprintf("%sCluster missing size\n", errorMsg)
	}

	if errorMsg == "" {
		return nil
	} else {
		return fmt.Errorf(errorMsg)
	}

}

func makeLabelsForMatrixone(name string) map[string]string {
	return map[string]string{"app": "matrixone"}
}

func namespacedName(name, namespace string) *types.NamespacedName {
	return &types.NamespacedName{Name: name, Namespace: namespace}
}

func stringifyForLogging(obj object, moc *v1alpha1.MatrixoneCluster) string {
	if bytes, err := json.Marshal(obj); err != nil {
		logger.Error(err, err.Error(), fmt.Sprintf("Failed to serialize [%s:%s]", obj.GetObjectKind().GroupVersionKind().Kind, obj.GetName()), "name", moc.Name, "namespace", moc.Namespace)
		return fmt.Sprintf("%v", obj)
	} else {
		return string(bytes)
	}
}

func makeStatefulSet() {}
