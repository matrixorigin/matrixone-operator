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
	// ls := makeLabelsForMatrixone()

	return nil
}

func namespacedName(name, namespace string) *types.NamespacedName {
	return &types.NamespacedName{Name: name, Namespace: namespace}
}

func makeLabelsForMatrixone() map[string]string {
	return map[string]string{"app": "matrixone"}
}

func stringifyForLogging(obj object, moc *v1alpha1.MatrixoneCluster) string {
	if bytes, err := json.Marshal(obj); err != nil {
		logger.Error(err, err.Error(), fmt.Sprintf("Failed to serialize [%s:%s]", obj.GetObjectKind().GroupVersionKind().Kind, obj.GetName()), "name", moc.Name, "namespace", moc.Namespace)
		return fmt.Sprintf("%v", obj)
	} else {
		return string(bytes)
	}
}
