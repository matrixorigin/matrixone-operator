package controllers

import (
	"context"
	"fmt"

	matrixonev1alpha1 "github.com/matrixorigin/matrixone-operator/api/v1"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func getPersistentVolumeClaim(moc *matrixonev1alpha1.MatrixoneCluster) []v1.PersistentVolumeClaim {
	pvc := []v1.PersistentVolumeClaim{}

	for _, val := range moc.Spec.VolumeClaimTemplates {
		pvc = append(pvc, val)
	}
	return pvc

}

func makePersistentVolumeClaim(pvc *v1.PersistentVolumeClaim, moc *matrixonev1alpha1.MatrixoneCluster, ls map[string]string, uniqueStr string) (*v1.PersistentVolumeClaim, error) {

	pvc.TypeMeta = metav1.TypeMeta{
		APIVersion: "v1",
		Kind:       "PersistentVolumeClaim",
	}

	pvc.ObjectMeta.Namespace = moc.Namespace

	if pvc.ObjectMeta.Labels == nil {
		pvc.ObjectMeta.Labels = ls
	} else {
		for k, v := range ls {
			pvc.ObjectMeta.Labels[k] = v
		}
	}

	if pvc.ObjectMeta.Name == "" {
		pvc.ObjectMeta.Name = uniqueStr
	}

	return pvc, nil
}

func makePersistentVolumeClaimListEmptyObj() *v1.PersistentVolumeClaimList {
	return &v1.PersistentVolumeClaimList{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "PersistentVolumeClaim",
		},
	}
}

// scalePVCForSts shall expand the sts volumeclaimtemplates size as well as N no of pvc supported by the sts.
// NOTE: To be called only if generation > 1
func scalePVCForSts(sdk client.Client, uniqueStr string, moc *matrixonev1alpha1.MatrixoneCluster, emitEvent EventEmitter) error {

	getSTSList, err := readers.List(context.TODO(), sdk, moc, makeLabelsForMatrixone(moc.Name), emitEvent, func() objectList { return makeStatefulSetListEmptyObj() }, func(listObj runtime.Object) []object {
		items := listObj.(*appsv1.StatefulSetList).Items
		result := make([]object, len(items))
		for i := 0; i < len(items); i++ {
			result[i] = &items[i]
		}
		return result
	})
	if err != nil {
		return nil
	}

	// Dont proceed unless all statefulsets are up and running.
	//  This can cause the go routine to panic

	for _, sts := range getSTSList {
		if sts.(*appsv1.StatefulSet).Status.Replicas != sts.(*appsv1.StatefulSet).Status.ReadyReplicas {
			return nil
		}
	}

	// return nil, in case return err the program halts since sts would not be able
	// we would like the operator to create sts.
	sts, err := readers.Get(context.TODO(), sdk, uniqueStr, moc, func() object { return makeStatefulSetEmptyObj() }, emitEvent)
	if err != nil {
		return nil
	}

	pvcLabels := map[string]string{
		"app": "matrixone",
	}

	pvcList, err := readers.List(context.TODO(), sdk, moc, pvcLabels, emitEvent, func() objectList { return makePersistentVolumeClaimListEmptyObj() }, func(listObj runtime.Object) []object {
		items := listObj.(*v1.PersistentVolumeClaimList).Items
		result := make([]object, len(items))
		for i := 0; i < len(items); i++ {
			result[i] = &items[i]
		}
		return result
	})
	if err != nil {
		return nil
	}

	desVolumeClaimTemplateSize, currVolumeClaimTemplateSize, pvcSize := getVolumeClaimTemplateSizes(sts, moc, pvcList)

	// current number of PVC can't be less than desired number of pvc
	if len(pvcSize) < len(desVolumeClaimTemplateSize) {
		return nil
	}

	// iterate over array for matching each index in desVolumeClaimTemplateSize, currVolumeClaimTemplateSize and pvcSize
	for i := range desVolumeClaimTemplateSize {

		// Validate Request, shrinking of pvc not supported
		// desired size cant be less than current size
		// in that case re-create sts/pvc which is a user execute manual step

		desiredSize, _ := desVolumeClaimTemplateSize[i].AsInt64()
		currentSize, _ := currVolumeClaimTemplateSize[i].AsInt64()

		if desiredSize < currentSize {
			e := fmt.Errorf("Request for Shrinking of sts pvc size [sts:%s] in [namespace:%s] is not Supported", sts.(*appsv1.StatefulSet).Name, sts.(*appsv1.StatefulSet).Namespace)
			klog.Error(e, e.Error(), "name", moc.Name, "namespace", moc.Namespace)
			emitEvent.EmitEventGeneric(moc, "MatrixoneOperatorPvcReSizeFail", "", err)
			return e
		}

		// In case size dont match and dessize > currsize, delete the sts using casacde=false / propagation policy set to orphan
		// The operator on next reconcile shall create the sts with latest changes
		if desiredSize != currentSize {
			msg := fmt.Sprintf("Detected Change in VolumeClaimTemplate Sizes for Statefuleset [%s] in Namespace [%s], desVolumeClaimTemplateSize: [%s], currVolumeClaimTemplateSize: [%s]\n, deleteing STS [%s] with casacde=false]", sts.(*appsv1.StatefulSet).Name, sts.(*appsv1.StatefulSet).Namespace, desVolumeClaimTemplateSize[i].String(), currVolumeClaimTemplateSize[i].String(), sts.(*appsv1.StatefulSet).Name)
			klog.Info(msg)
			emitEvent.EmitEventGeneric(moc, "MatrixoneOperatorPvcReSizeDetected", msg, nil)

			if err := writers.Delete(context.TODO(), sdk, moc, sts, emitEvent, client.PropagationPolicy(metav1.DeletePropagationOrphan)); err != nil {
				return err
			} else {
				msg := fmt.Sprintf("[StatefuleSet:%s] successfully deleted with casacde=false", sts.(*appsv1.StatefulSet).Name)
				klog.Info(msg, "name", moc.Name, "namespace", moc.Namespace)
				emitEvent.EmitEventGeneric(moc, "MatrixoneOperatorStsOrphaned", msg, nil)
			}

		}

		// In case size dont match, patch the pvc with the desiredsize from matrixone CR
		for p := range pvcSize {
			pSize, _ := pvcSize[p].AsInt64()
			if desiredSize != pSize {
				// use deepcopy
				patch := client.MergeFrom(pvcList[p].(*v1.PersistentVolumeClaim).DeepCopy())
				pvcList[p].(*v1.PersistentVolumeClaim).Spec.Resources.Requests[v1.ResourceStorage] = desVolumeClaimTemplateSize[i]
				if err := writers.Patch(context.TODO(), sdk, moc, pvcList[p].(*v1.PersistentVolumeClaim), false, patch, emitEvent); err != nil {
					return err
				} else {
					msg := fmt.Sprintf("[PVC:%s] successfully Patched with [Size:%s]", pvcList[p].(*v1.PersistentVolumeClaim).Name, desVolumeClaimTemplateSize[i].String())
					klog.Info(msg, "name", moc.Name, "namespace", moc.Namespace)
				}
			}
		}

	}

	return nil
}

func getVolumeClaimTemplateSizes(sts object, moc *matrixonev1alpha1.MatrixoneCluster, pvc []object) (desVolumeClaimTemplateSize, currVolumeClaimTemplateSize, pvcSize []resource.Quantity) {

	for i := range moc.Spec.VolumeClaimTemplates {
		desVolumeClaimTemplateSize = append(desVolumeClaimTemplateSize, moc.Spec.VolumeClaimTemplates[i].Spec.Resources.Requests[v1.ResourceStorage])
	}

	for i := range sts.(*appsv1.StatefulSet).Spec.VolumeClaimTemplates {
		currVolumeClaimTemplateSize = append(currVolumeClaimTemplateSize, sts.(*appsv1.StatefulSet).Spec.VolumeClaimTemplates[i].Spec.Resources.Requests[v1.ResourceStorage])
	}

	for i := range pvc {
		pvcSize = append(pvcSize, pvc[i].(*v1.PersistentVolumeClaim).Spec.Resources.Requests[v1.ResourceStorage])
	}

	return desVolumeClaimTemplateSize, currVolumeClaimTemplateSize, pvcSize

}
