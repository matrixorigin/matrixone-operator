// Copyright 2021 Matrix Origin
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package matrixone

import (
	"context"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"

	"github.com/matrixorigin/matrixone-operator/pkg/apis/matrixone/v1alpha1"
	com "github.com/matrixorigin/matrixone-operator/pkg/controllers/components"
	"github.com/matrixorigin/matrixone-operator/pkg/controllers/utils"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	matrixoneOpResourceHash = "matrixoneOpResourceHash"
	finalizerName           = "deletepvc.finalizers.matrixone.matrixorigin.cn"
)

var logger = logf.Log.WithName("matrixone_operator_handler")

func deployMatrixoneCluster(sdk client.Client, moc *v1alpha1.MatrixoneCluster, emitEvents EventEmitter) error {
	serviceNames := make(map[string]bool)
	statefulSetNames := make(map[string]bool)

	ls := makeLabelsForMatrixone(moc.Name)

	// Create Service
	// see more: https://kubernetes.io/docs/concepts/services-networking/service/
	if _, err := sdkCreateOrUpdateAsNeeded(sdk,
		func() (object, error) { return com.MakeService(&corev1.Service{}, moc, ls, false) },
		func() object { return com.MakeServiceEmptyObj() }, alwaysTrueIsEqualsFn,
		func(prev, curr object) {
			(curr.(*corev1.Service)).Spec.ClusterIP = (prev.(*corev1.Service)).Spec.ClusterIP
		},
		moc, serviceNames, emitEvents); err != nil {
		return err
	}

	// Create Headless Service
	if _, err := sdkCreateOrUpdateAsNeeded(sdk,
		func() (object, error) { return com.MakeService(&corev1.Service{}, moc, ls, true) },
		func() object { return com.MakeServiceEmptyObj() }, alwaysTrueIsEqualsFn,
		func(prev, curr object) {
			(curr.(*corev1.Service)).Spec.ClusterIP = (prev.(*corev1.Service)).Spec.ClusterIP
		},
		moc, serviceNames, emitEvents); err != nil {
		return err
	}

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
			if !utils.ContainsString(moc.ObjectMeta.Finalizers, finalizerName) {
				moc.SetFinalizers(append(moc.GetFinalizers(), finalizerName))
				_, err := writers.Update(context.Background(), sdk, moc, moc, emitEvents)
				if err != nil {
					return err
				}
			}
		}
	}

	// Create/Update Statefulset
	if stsCreateUpdateStatus, err := sdkCreateOrUpdateAsNeeded(
		sdk,
		func() (object, error) { return com.MakeSts(moc, ls) },
		func() object { return com.MakeStatefulSetEmptyObj() },
		statefulSetIsEquals,
		updateFn,
		moc,
		statefulSetNames,
		emitEvents); err != nil {
		return err
	} else if moc.Spec.RollingDeploy {
		if stsCreateUpdateStatus == resourceUpdated {
			return nil
		}

		execCheckCrashStatus(sdk, moc, emitEvents)

		if moc.Generation > 1 {
			done, err := isObjFullyDeployed(sdk, moc, func() object { return com.MakeServiceEmptyObj() }, emitEvents)
			if !done {
				return err
			}
		}

	}

	if moc.Generation > 1 && moc.Spec.DeleteOrphanPvc {
		if err := deleteOrphanPVC(sdk, moc, emitEvents); err != nil {
			return err
		}
	}

	updatedStatus := v1alpha1.MatrixoneClusterStatus{}
	updatedStatus.StatefulSets = deleteUnusedResources(sdk, moc, statefulSetNames, ls,
		func() objectList { return com.MakeStatefulSetListEmptyObj() },
		func(listObj runtime.Object) []object {
			items := listObj.(*appsv1.StatefulSetList).Items
			result := make([]object, len(items))
			for i := 0; i < len(items); i++ {
				result[i] = &items[i]
			}
			return result
		}, emitEvents)
	sort.Strings(updatedStatus.StatefulSets)

	updatedStatus.Services = deleteUnusedResources(sdk, moc, serviceNames, ls,
		func() objectList { return com.MakeServiceListEmptyObj() },
		func(listObj runtime.Object) []object {
			items := listObj.(*v1.ServiceList).Items
			result := make([]object, len(items))
			for i := 0; i < len(items); i++ {
				result[i] = &items[i]
			}
			return result
		}, emitEvents)
	sort.Strings(updatedStatus.Services)

	podList, _ := readers.List(context.TODO(), sdk, moc, makeLabelsForMatrixone(moc.Name), emitEvents, func() objectList { return com.MakePodList() }, func(listObj runtime.Object) []object {
		items := listObj.(*v1.PodList).Items
		result := make([]object, len(items))
		for i := 0; i < len(items); i++ {
			result[i] = &items[i]
		}
		return result
	})

	updatedStatus.Pods = getPodNames(podList)
	sort.Strings(updatedStatus.Pods)

	err := matrixnoeClusterStatusPatcher(sdk, updatedStatus, moc, emitEvents)
	if err != nil {
		return err
	}

	return nil
}

func sdkCreateOrUpdateAsNeeded(
	sdk client.Client,
	objFn func() (object, error),
	emptyObjFn func() object,
	isEqualFn func(prev, curr object) bool,
	updaterFn func(prev, curr object),
	moc *v1alpha1.MatrixoneCluster,
	names map[string]bool,
	emitEvent EventEmitter,
) (MatrixoneClusterStatus, error) {
	if obj, err := objFn(); err != nil {
		return "", nil
	} else {
		names[obj.GetName()] = true

		addOwnerRefToObject(obj, asOwner(moc))
		addHashToObject(obj)
		prevObj := emptyObjFn()
		if err := sdk.Get(context.TODO(), *namespacedName(obj.GetName(), obj.GetNamespace()), prevObj); err != nil {
			if apierrors.IsNotFound(err) {
				// resource dose not exist, create it.
				create, err := writers.Create(context.TODO(), sdk, moc, obj, emitEvent)
				if err != nil {
					return "", err
				} else {
					return create, nil
				}
			} else {
				e := fmt.Errorf("Failed to get [%s:%s] due to [%s].", obj.GetObjectKind().GroupVersionKind().Kind, obj.GetName(), err.Error())
				logger.Error(e, e.Error(), "Prev object", stringifyForLogging(prevObj, moc), "name", moc.Name, "namespace", moc.Namespace)
				emitEvent.EmitEventGeneric(moc, string(matrixoneObjectGetFail), "", err)
				return "", err
			}
		} else {
			if obj.GetAnnotations()[matrixoneOpResourceHash] != prevObj.GetAnnotations()[matrixoneOpResourceHash] || !isEqualFn(prevObj, obj) {
				obj.SetResourceVersion(prevObj.GetResourceVersion())
				updaterFn(prevObj, obj)
				update, err := writers.Update(context.TODO(), sdk, moc, obj, emitEvent)
				if err != nil {
					return "", err
				} else {
					return update, err
				}
			} else {
				return "", nil
			}
		}
	}
}

func isObjFullyDeployed(
	sdk client.Client,
	moc *v1alpha1.MatrixoneCluster,
	emptyObjFn func() object,
	emitEvent EventEmitter) (bool, error) {

	// Get Object
	obj, err := readers.Get(context.TODO(), sdk, moc, emptyObjFn, emitEvent)
	if err != nil {
		return false, err
	}

	// In case obj is a statefulset or deployment, make sure the sts/deployment has successfully reconciled to desired state
	if detectType(obj) == "*corev1.StatefulSet" {
		if obj.(*appsv1.StatefulSet).Status.CurrentRevision != obj.(*appsv1.StatefulSet).Status.UpdateRevision {
			return false, nil
		} else if obj.(*appsv1.StatefulSet).Status.CurrentReplicas != obj.(*appsv1.StatefulSet).Status.ReadyReplicas {
			return false, nil
		} else {
			return obj.(*appsv1.StatefulSet).Status.CurrentRevision == obj.(*appsv1.StatefulSet).Status.UpdateRevision, nil
		}
	} else if detectType(obj) == "*corev1.Deployment" {
		for _, condition := range obj.(*appsv1.Deployment).Status.Conditions {
			// This detects a failure condition, operator should send a rolling deployment failed event
			if condition.Type == appsv1.DeploymentReplicaFailure {
				return false, errors.New(condition.Reason)
			} else if condition.Type == appsv1.DeploymentProgressing && condition.Status != corev1.ConditionTrue || obj.(*appsv1.Deployment).Status.ReadyReplicas != obj.(*appsv1.Deployment).Status.Replicas {
				return false, nil
			} else {
				return obj.(*appsv1.Deployment).Status.ReadyReplicas == obj.(*appsv1.Deployment).Status.Replicas, nil
			}
		}
	}
	return false, nil
}

// addOwnerRefToObject appends the desired OwnerReference to the object
func addOwnerRefToObject(obj metav1.Object, ownerRef metav1.OwnerReference) {
	obj.SetOwnerReferences(append(obj.GetOwnerReferences(), ownerRef))
}

// asOwner returns an OwnerReference set as the MatrixoneCluster CR
func asOwner(moc *v1alpha1.MatrixoneCluster) metav1.OwnerReference {
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

func namespacedName(name, namespace string) *types.NamespacedName {
	return &types.NamespacedName{Name: name, Namespace: namespace}
}

func alwaysTrueIsEqualsFn(prev, curr object) bool {
	return true
}

func stringifyForLogging(obj object, moc *v1alpha1.MatrixoneCluster) string {
	if bytes, err := json.Marshal(obj); err != nil {
		logger.Error(err, err.Error(), fmt.Sprintf("Failed to serialize [%s:%s]", obj.GetObjectKind().GroupVersionKind().Kind, obj.GetName()), "name", moc.Name, "namespace", moc.Namespace)
		return fmt.Sprintf("%v", obj)
	} else {
		return string(bytes)
	}

}

func statefulSetIsEquals(obj1, obj2 object) bool {

	return true
}

func updateFn(prev, curr object) {}

func execCheckCrashStatus(sdk client.Client, moc *v1alpha1.MatrixoneCluster, event EventEmitter) {
	if moc.Spec.ForceDeleteStsPodOnError == false {
		return
	} else {
		if moc.Spec.PodManagementPolicy == "OrderedReady" {
			checkCrashStatus(sdk, moc, event)
		}
	}
}

func makeLabelsForMatrixone(name string) map[string]string {
	return map[string]string{"app": "matrixone", "matrixone_cr": name}
}

func checkCrashStatus(sdk client.Client, moc *v1alpha1.MatrixoneCluster, emitEvents EventEmitter) error {

	podList, err := readers.List(context.TODO(), sdk, moc, makeLabelsForMatrixone(moc.Name), emitEvents, func() objectList { return com.MakePodList() }, func(listObj runtime.Object) []object {
		items := listObj.(*corev1.PodList).Items
		result := make([]object, len(items))
		for i := 0; i < len(items); i++ {
			result[i] = &items[i]
		}
		return result
	})
	if err != nil {
		return err
	}

	for _, p := range podList {
		if p.(*corev1.Pod).Status.ContainerStatuses[0].RestartCount > 1 {
			for _, condition := range p.(*corev1.Pod).Status.Conditions {
				// condition.type Ready means the pod is able to service requests
				if condition.Type == corev1.ContainersReady {
					// the below condition evalutes if a pod is in
					// 1. pending state 2. failed state 3. unknown state
					// OR condtion.status is false which evalutes if neither of these conditions are met
					// 1. ContainersReady 2. PodInitialized 3. PodReady 4. PodScheduled
					if p.(*corev1.Pod).Status.Phase != corev1.PodRunning || condition.Status == corev1.ConditionFalse {
						err := writers.Delete(context.TODO(), sdk, moc, p, emitEvents, &client.DeleteOptions{})
						if err != nil {
							return err
						} else {
							msg := fmt.Sprintf("Deleted pod [%s] in namespace [%s], since it was in crashloopback state.", p.GetName(), p.GetNamespace())
							logger.Info(msg, "Object", stringifyForLogging(p, moc), "name", moc.Name, "namespace", moc.Namespace)
						}
					}
				}
			}
		}
	}

	return nil
}

func deleteUnusedResources(sdk client.Client, moc *v1alpha1.MatrixoneCluster,
	names map[string]bool, selectorLabels map[string]string, emptyListObjFn func() objectList, itemsExtractorFn func(obj runtime.Object) []object, emitEvents EventEmitter) []string {

	listOpts := []client.ListOption{
		client.InNamespace(moc.Namespace),
		client.MatchingLabels(selectorLabels),
	}

	survivorNames := make([]string, 0, len(names))

	listObj := emptyListObjFn()

	if err := sdk.List(context.TODO(), listObj, listOpts...); err != nil {
		e := fmt.Errorf("failed to list [%s] due to [%s]", listObj.GetObjectKind().GroupVersionKind().Kind, err.Error())
		logger.Error(e, e.Error(), "name", moc.Name, "namespace", moc.Namespace)
	} else {
		for _, s := range itemsExtractorFn(listObj) {
			if names[s.GetName()] == false {
				err := writers.Delete(context.TODO(), sdk, moc, s, emitEvents, &client.DeleteOptions{})
				if err != nil {
					survivorNames = append(survivorNames, s.GetName())
				}
			} else {
				survivorNames = append(survivorNames, s.GetName())
			}
		}
	}

	return survivorNames
}

func deleteOrphanPVC(sdk client.Client, moc *v1alpha1.MatrixoneCluster, emitEvents EventEmitter) error {

	podList, err := readers.List(context.TODO(), sdk, moc, makeLabelsForMatrixone(moc.Name), emitEvents, func() objectList { return com.MakePodList() }, func(listObj runtime.Object) []object {
		items := listObj.(*corev1.PodList).Items
		result := make([]object, len(items))
		for i := 0; i < len(items); i++ {
			result[i] = &items[i]
		}
		return result
	})
	if err != nil {
		return err
	}

	pvcLabels := map[string]string{
		"matrixone_cr": moc.Name,
	}

	pvcList, err := readers.List(context.TODO(), sdk, moc, pvcLabels, emitEvents, func() objectList { return com.MakePersistentVolumeClaimListEmptyObj() }, func(listObj runtime.Object) []object {
		items := listObj.(*corev1.PersistentVolumeClaimList).Items
		result := make([]object, len(items))
		for i := 0; i < len(items); i++ {
			result[i] = &items[i]
		}
		return result
	})
	if err != nil {
		return err
	}

	for _, pod := range podList {
		if pod.(*corev1.Pod).Status.Phase != corev1.PodRunning {
			return nil
		}
		for _, status := range pod.(*corev1.Pod).Status.Conditions {
			if status.Status != corev1.ConditionTrue {
				return nil
			}
		}
	}

	mountedPVC := make([]string, len(podList))
	for _, pod := range podList {
		if pod.(*corev1.Pod).Spec.Volumes != nil {
			for _, vol := range pod.(*corev1.Pod).Spec.Volumes {
				if vol.PersistentVolumeClaim != nil {
					if !utils.ContainsString(mountedPVC, vol.PersistentVolumeClaim.ClaimName) {
						mountedPVC = append(mountedPVC, vol.PersistentVolumeClaim.ClaimName)
					}
				}
			}
		}

	}

	if mountedPVC != nil {
		for i, pvc := range pvcList {

			if !utils.ContainsString(mountedPVC, pvc.GetName()) {
				err := writers.Delete(context.TODO(), sdk, moc, pvcList[i], emitEvents, &client.DeleteAllOfOptions{})
				if err != nil {
					return err
				} else {
					msg := fmt.Sprintf("Deleted orphaned pvc [%s:%s] successfully", pvcList[i].GetName(), moc.Namespace)
					logger.Info(msg, "name", moc.Name, "namespace", moc.Namespace)
				}
			}
		}
	}
	return nil
}

func getPodNames(pods []object) []string {
	var podNames []string
	for _, pod := range pods {
		podNames = append(podNames, pod.(*corev1.Pod).Name)
	}
	return podNames
}

// wrapper to patch matrixone cluster status
func matrixnoeClusterStatusPatcher(sdk client.Client, updatedStatus v1alpha1.MatrixoneClusterStatus, moc *v1alpha1.MatrixoneCluster, emitEvent EventEmitter) error {

	if !reflect.DeepEqual(updatedStatus, moc.Status) {
		patchBytes, err := json.Marshal(map[string]v1alpha1.MatrixoneClusterStatus{"status": updatedStatus})
		if err != nil {
			return fmt.Errorf("failed to serialize status patch to bytes: %v", err)
		}
		_ = writers.Patch(context.TODO(), sdk, moc, moc, true, client.RawPatch(types.MergePatchType, patchBytes), emitEvent)
	}
	return nil
}

func executeFinalizers(sdk client.Client, moc *v1alpha1.MatrixoneCluster, emitEvents EventEmitter) error {

	if utils.ContainsString(moc.ObjectMeta.Finalizers, finalizerName) {
		pvcLabels := map[string]string{
			"matrixone_cr": moc.Name,
		}

		pvcList, err := readers.List(context.TODO(), sdk, moc, pvcLabels, emitEvents, func() objectList { return com.MakePersistentVolumeClaimListEmptyObj() }, func(listObj runtime.Object) []object {
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

		stsList, err := readers.List(context.TODO(), sdk, moc, makeLabelsForMatrixone(moc.Name), emitEvents, func() objectList { return com.MakeStatefulSetListEmptyObj() }, func(listObj runtime.Object) []object {
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
		logger.Info(msg)
		if err := deleteSTSAndPVC(sdk, moc, stsList, pvcList, emitEvents); err != nil {
			return err
		} else {
			msg := fmt.Sprintf("Finalizer success for CR [%s] in namespace [%s]", moc.Name, moc.Namespace)
			logger.Info(msg)
		}

		// remove our finalizer from the list and update it.
		moc.ObjectMeta.Finalizers = utils.RemoveString(moc.ObjectMeta.Finalizers, finalizerName)

		_, err = writers.Update(context.TODO(), sdk, moc, moc, emitEvents)
		if err != nil {
			return err
		}

	}
	return nil

}

func deleteSTSAndPVC(sdk client.Client, moc *v1alpha1.MatrixoneCluster, stsList, pvcList []object, emitEvents EventEmitter) error {

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

func checkIfCRExists(sdk client.Client, moc *v1alpha1.MatrixoneCluster, emitEvents EventEmitter) bool {
	_, err := readers.Get(context.TODO(), sdk, moc, func() object { return makeMatrixoneEmptyObj() }, emitEvents)
	if err != nil {
		return false
	} else {
		return true
	}
}

func makeMatrixoneEmptyObj() *v1alpha1.MatrixoneCluster {
	return &v1alpha1.MatrixoneCluster{
		TypeMeta: metav1.TypeMeta{
			Kind:       "MatrixoneCluster",
			APIVersion: "v1alpha1",
		},
	}
}
