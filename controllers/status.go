package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/matrixorigin/matrixone-operator/api/v1alpha1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func newMatrixoneNodeTypeStatus(
	nodeConditionStatus v1.ConditionStatus,
	nodeCondition v1alpha1.MatrixoneNodeConditionType,
	nodeTierOrType string,
	err error) *v1alpha1.MatrixoneNodeTypeStatus {

	var reason string

	if nodeCondition == v1alpha1.MatrixoneClusterReady {
		nodeTierOrType = "All"
		reason = "All Matrixone Nodes are in Ready Condition"
	} else if nodeCondition == v1alpha1.MatrixoneNodeRollingUpdate {
		reason = "Matrixone Node [" + nodeTierOrType + "] is Rolling Update"
	} else if err != nil {
		reason = err.Error()
		nodeCondition = v1alpha1.MatrixoneNodeErrorState
	}

	return &v1alpha1.MatrixoneNodeTypeStatus{
		MatrixoneNode:                nodeTierOrType,
		MatrixoneNodeConditionStatus: nodeConditionStatus,
		MatrixoneNodeConditionType:   nodeCondition,
		Reason:                       reason,
	}

}

func matrixoneClusterStatusPatcher(sdk client.Client, updatedStatus v1alpha1.MatrixoneClusterStatus, m *v1alpha1.MatrixoneCluster, emitEvent EventEmitter) error {

	if !reflect.DeepEqual(updatedStatus, m.Status) {
		patchBytes, err := json.Marshal(map[string]v1alpha1.MatrixoneClusterStatus{"status": updatedStatus})
		if err != nil {
			return fmt.Errorf("failed to serialize status patch to bytes: %v", err)
		}
		_ = writers.Patch(context.TODO(), sdk, m, m, true, client.RawPatch(types.MergePatchType, patchBytes), emitEvent)
	}
	return nil
}

func matrixoneNodeConditionStatusPatch(
	updatedStatus v1alpha1.MatrixoneClusterStatus,
	sdk client.Client,
	nodeSpecUniqueStr string,
	m *v1alpha1.MatrixoneCluster,
	emitEvent EventEmitter,
	emptyObjFn func() object) (err error) {

	if !reflect.DeepEqual(updatedStatus.MatrixoneNodeStatus, m.Status.MatrixoneNodeStatus) {

		err = matrixoneClusterStatusPatcher(sdk, updatedStatus, m, emitEvent)
		if err != nil {
			return err
		}

		obj, err := readers.Get(context.TODO(), sdk, nodeSpecUniqueStr, m, emptyObjFn, emitEvent)
		if err != nil {
			return err
		}

		emitEvent.EmitEventRollingDeployWait(m, obj, nodeSpecUniqueStr)

		return nil

	}
	return nil
}
