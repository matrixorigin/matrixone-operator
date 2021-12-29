package controllers

import (
	"context"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"fmt"

	matrixonev1alpha1 "github.com/matrixorigin/matrixone-operator/api/v1"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	storage "k8s.io/api/storage/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var logger = logf.Log.WithName("matrixone_operator_handler")

const (
	matrixoneOpResourceHash = "matrixoneOpResourceHash"
	finalizerName           = "deletepvc.finalizers.matrixone.matrixorigin.cn"
)

func deployMatrixoneCluster(sdk client.Client, moc *matrixonev1alpha1.MatrixoneCluster, emitEvents EventEmitter) error {
	logger.Info("deployMatrixoneCluster")

	ls := makeLabelsForMatrixone(moc.Name)

	statefulSetNames := make(map[string]bool)
	serviceNames := make(map[string]bool)
	// pvcNames := make(map[string]bool)
	uniqueStr := "matrixone-cluster"

	serviceName := ""
	services := moc.Spec.Services
	for _, svc := range services {
		if _, err := sdkCreateOrUpdateAsNeeded(sdk,
			func() (object, error) { return makeService(&svc, moc, ls, uniqueStr) },
			func() object { return makeServiceEmptyObj() }, alwaysTrueIsEqualsFn,
			func(prev, curr object) { (curr.(*v1.Service)).Spec.ClusterIP = (prev.(*v1.Service)).Spec.ClusterIP },
			moc, serviceNames, emitEvents); err != nil {
			return err
		}
		if serviceName == "" {
			serviceName = svc.ObjectMeta.Name
		}

	}

	//	scalePVCForSTS to be only called only if volumeExpansion is supported by the storage class.
	//  Ignore for the first iteration ie cluster creation, else get sts shall unnecessary log errors.

	if moc.Generation > 1 && moc.Spec.ScalePvcSts {
		if isVolumeExpansionEnabled(sdk, moc, emitEvents) {
			err := scalePVCForSts(sdk, uniqueStr, moc, emitEvents)
			if err != nil {
				return err
			}
		}
	}

	// Create/Update StatefulSet
	if stsCreateUpdateStatus, err := sdkCreateOrUpdateAsNeeded(sdk,
		func() (object, error) {
			return makeStatefulSet(moc, ls, serviceName, uniqueStr)
		},
		func() object { return makeStatefulSetEmptyObj() },
		statefulSetIsEquals, noopUpdaterFn, moc, statefulSetNames, emitEvents); err != nil {
		return err
	} else if moc.Spec.RollingDeploy {

		if stsCreateUpdateStatus == resourceUpdated {
			// we just updated, give sts controller some time to update status of replicas after update
			return nil
		}
	}

	// Default is set to true
	// execCheckCrashStatus(sdk, &nodeSpec, m, emitEvents)

	// Ignore isObjFullyDeployed() for the first iteration ie cluster creation
	// will force cluster creation in parallel, post first iteration rolling updates
	// will be sequential.
	// 	if m.Generation > 1 {
	// 		//Check StatefulSet rolling update status, if in-progress then stop here
	// 		done, err := isObjFullyDeployed(sdk, nodeSpec, nodeSpecUniqueStr, m, func() object { return makeStatefulSetEmptyObj() }, emitEvents)
	// 		if !done {
	// 			return err
	// 		}
	// 	}
	// }

	// Default is set to true
	// execCheckCrashStatus(sdk, &nodeSpec, m, emitEvents)

	/*
		Default Behavior: Finalizer shall be always executed resulting in deletion of pvc post deletion of Matrixone CR
		When the object (matrixone CR) has for deletion time stamp set, execute the finalizer
		Finalizer shall execute the following flow :
		1. Get sts List and PVC List
		2. Range and Delete sts first and then delete pvc. PVC must be deleted after sts termination has been executed
			else pvc finalizer shall block deletion since a pod/sts is referencing it.
		3. Once delete is executed we block program and return.
	*/

	if moc.Spec.DisablePVCDeletionFinalizer == false {
		md := moc.GetDeletionTimestamp() != nil
		if md {
			return executeFinalizers(sdk, moc, emitEvents)
		}
		/*
			If finalizer isn't present add it to object meta.
			In case cr is already deleted do not call this function
		*/
		cr := checkIfCRExists(sdk, moc, emitEvents)
		if cr {
			if !ContainsString(moc.ObjectMeta.Finalizers, finalizerName) {
				moc.SetFinalizers(append(moc.GetFinalizers(), finalizerName))
				_, err := writers.Update(context.Background(), sdk, moc, moc, emitEvents)
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func executeFinalizers(sdk client.Client, moc *matrixonev1alpha1.MatrixoneCluster, emitEvents EventEmitter) error {

	if ContainsString(moc.ObjectMeta.Finalizers, finalizerName) {
		pvcLabels := map[string]string{
			"matrixone_cr": moc.Name,
		}

		pvcList, err := readers.List(
			context.TODO(),
			sdk,
			moc,
			pvcLabels,
			emitEvents,
			func() objectList { return makePersistentVolumeClaimListEmptyObj() },
			func(listObj runtime.Object) []object {
				items := listObj.(*v1.PersistentVolumeClaimList).Items
				result := make([]object, len(items))
				for i := 0; i < len(items); i++ {
					result[i] = &items[i]
				}
				return result
			})
		if err != nil {
			return err
		}

		stsList, err := readers.List(
			context.TODO(),
			sdk,
			moc,
			makeLabelsForMatrixone(moc.Name),
			emitEvents,
			func() objectList { return makeStatefulSetListEmptyObj() },
			func(listObj runtime.Object) []object {
				items := listObj.(*appsv1.StatefulSetList).Items
				result := make([]object, len(items))
				for i := 0; i < len(items); i++ {
					result[i] = &items[i]
				}
				return result
			})
		if err != nil {
			return err
		}

		msg := fmt.Sprintf("Trigerring finalizer for CR [%s] in namespace [%s]", moc.Name, moc.Namespace)
		//	sendEvent(sdk, m, v1.EventTypeNormal, MatrixoneFinalizer, msg)
		logger.Info(msg)
		if err := deleteSTSAndPVC(sdk, moc, stsList, pvcList, emitEvents); err != nil {
			return err
		} else {
			msg := fmt.Sprintf("Finalizer success for CR [%s] in namespace [%s]", moc.Name, moc.Namespace)
			// sendEvent(sdk, m, v1.EventTypeNormal, MatrixoneFinalizerSuccess, msg)
			logger.Info(msg)
		}

		// remove our finalizer from the list and update it.
		moc.ObjectMeta.Finalizers = RemoveString(moc.ObjectMeta.Finalizers, finalizerName)

		_, err = writers.Update(context.TODO(), sdk, moc, moc, emitEvents)
		if err != nil {
			return err
		}

	}
	return nil

}

func namespacedName(name, namespace string) *types.NamespacedName {
	return &types.NamespacedName{Name: name, Namespace: namespace}
}

func makeLabelsForMatrixone(name string) map[string]string {
	return map[string]string{"app": "matrixone", "matrixone_cr": name}
}

func stringifyForLogging(obj object, moc *matrixonev1alpha1.MatrixoneCluster) string {
	if bytes, err := json.Marshal(obj); err != nil {
		logger.Error(err, err.Error(),
			fmt.Sprintf("Failed to serialize [%s:%s]", obj.GetObjectKind().GroupVersionKind().Kind, obj.GetName()), "name", moc.Name, "namespace", moc.Namespace)
		return fmt.Sprintf("%v", obj)
	} else {
		return string(bytes)
	}
}

func sdkCreateOrUpdateAsNeeded(
	sdk client.Client,
	objFn func() (object, error),
	emptyObjFn func() object,
	isEqualFn func(prev, curr object) bool,
	updaterFn func(prev, curr object),
	moc *matrixonev1alpha1.MatrixoneCluster,
	names map[string]bool,
	emitEvent EventEmitter) (MatrixoneNodeStatus, error) {
	if obj, err := objFn(); err != nil {
		return "", err
	} else {
		names[obj.GetName()] = true

		addOwnerRefToObject(obj, asOwner(moc))
		addHashToObject(obj)

		prevObj := emptyObjFn()
		if err := sdk.Get(context.TODO(), *namespacedName(obj.GetName(), obj.GetNamespace()), prevObj); err != nil {
			if apierrors.IsNotFound(err) {
				// resource does not exist, create it.
				logger.Info("Create Resources")
				create, err := writers.Create(context.TODO(), sdk, moc, obj, emitEvent)
				if err != nil {
					return "", err
				} else {
					return create, nil
				}
			} else {
				e := fmt.Errorf("Failed to get [%s:%s] due to [%s].", obj.GetObjectKind().GroupVersionKind().Kind, obj.GetName(), err.Error())
				logger.Error(e, e.Error(), "Prev object", stringifyForLogging(prevObj, moc), "name", moc.Name, "namespace", moc.Namespace)
				emitEvent.EmitEventGeneric(moc, string(matrixoneOjectGetFail), "", err)
				return "", e
			}
		} else {
			// TODO resource already exists, updated it if needed
		}
	}
	return "", nil
}

// addOwnerRefToObject appends the desired OwnerReference to the object
func addOwnerRefToObject(obj metav1.Object, ownerRef metav1.OwnerReference) {
	obj.SetOwnerReferences(append(obj.GetOwnerReferences(), ownerRef))
}

// asOwner returns an OwnerReference set as the matrixone CR
func asOwner(moc *matrixonev1alpha1.MatrixoneCluster) metav1.OwnerReference {
	trueVar := true
	return metav1.OwnerReference{
		APIVersion: moc.APIVersion,
		Kind:       moc.Kind,
		Name:       moc.Name,
		UID:        moc.UID,
		Controller: &trueVar,
	}
}

func addHashToObject(obj object) error {
	if sha, err := getObjectHash(obj); err != nil {
		return err
	} else {
		annotations := obj.GetAnnotations()
		if annotations == nil {
			annotations = make(map[string]string)
			obj.SetAnnotations(annotations)
		}
		annotations[matrixoneOpResourceHash] = sha
		return nil
	}
}

func getObjectHash(obj object) (string, error) {
	if bytes, err := json.Marshal(obj); err != nil {
		return "", err
	} else {
		sha1Bytes := sha1.Sum(bytes)
		return base64.StdEncoding.EncodeToString(sha1Bytes[:]), nil
	}
}

func checkIfCRExists(sdk client.Client, moc *matrixonev1alpha1.MatrixoneCluster, emitEvents EventEmitter) bool {
	_, err := readers.Get(context.TODO(), sdk, moc.Name, moc, func() object { return makeMatrixoneEmptyObj() }, emitEvents)
	if err != nil {
		return false
	} else {
		return true
	}
}

func makeMatrixoneEmptyObj() *matrixonev1alpha1.MatrixoneCluster {
	return &matrixonev1alpha1.MatrixoneCluster{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Matrixone",
			APIVersion: "v1alpha1",
		},
	}
}

func deleteSTSAndPVC(sdk client.Client, moc *matrixonev1alpha1.MatrixoneCluster, stsList, pvcList []object, emitEvents EventEmitter) error {

	for _, sts := range stsList {
		err := writers.Delete(context.TODO(), sdk, moc, sts, emitEvents, &client.DeleteAllOfOptions{})
		if err != nil {
			return err
		}
	}

	for i := range pvcList {
		err := writers.Delete(context.TODO(), sdk, moc, pvcList[i], emitEvents, &client.DeleteAllOfOptions{})
		if err != nil {
			return err
		}
	}

	return nil
}

func makeUniqueString(moc *matrixonev1alpha1.MatrixoneCluster, key string) string {
	return fmt.Sprintf("matrixone-%s-%s", moc.Name, key)
}

func alwaysTrueIsEqualsFn(prev, curr object) bool {
	return true
}

func isVolumeExpansionEnabled(sdk client.Client, moc *matrixonev1alpha1.MatrixoneCluster, emitEvent EventEmitter) bool {

	for _, nodeVCT := range moc.Spec.VolumeClaimTemplates {
		sc, err := readers.Get(context.TODO(), sdk, *nodeVCT.Spec.StorageClassName, moc, func() object { return makeStorageClassEmptyObj() }, emitEvent)
		if err != nil {
			return false
		}

		if sc.(*storage.StorageClass).AllowVolumeExpansion != boolFalse() {
			return true
		}
	}
	return false
}

func makeStorageClassEmptyObj() *storage.StorageClass {
	return &storage.StorageClass{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "storage.k8s.io/v1",
			Kind:       "StorageClass",
		},
	}
}

func statefulSetIsEquals(obj1, obj2 object) bool {

	return true
}

func noopUpdaterFn(prev, curr object) {
	// do nothing
}
