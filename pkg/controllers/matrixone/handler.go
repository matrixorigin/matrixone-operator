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

	svc, err := com.MakeService(moc, ls, false)
	if err != nil {
		return err
	}
	if _, err := sdkCreateOrUpdateAsNeeded(
		sdk,
		svc,
		alwaysTrueIsEqualsFn[*corev1.Service],
		com.RetainClusterIP,
		moc,
		serviceNames,
		emitEvents); err != nil {
		return err
	}

	headlessSvc, err := com.MakeService(moc, ls, true)
	if err != nil {
		return err
	}
	// Create Headless Service
	if _, err := sdkCreateOrUpdateAsNeeded(
		sdk,
		headlessSvc,
		alwaysTrueIsEqualsFn[*corev1.Service],
		com.RetainClusterIP,
		moc,
		serviceNames,
		emitEvents); err != nil {
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
				_, err := Update(context.Background(), sdk, moc, moc, emitEvents)
				if err != nil {
					return err
				}
			}
		}
	}
	sts, err := com.MakeSts(moc, ls)
	if err != nil {
		return err
	}
	// Create/Update Statefulset
	if stsCreateUpdateStatus, err := sdkCreateOrUpdateAsNeeded(
		sdk,
		sts,
		alwaysTrueIsEqualsFn[*appsv1.StatefulSet],
		updateFn[*appsv1.StatefulSet],
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
			done, err := isObjectFullyDeployed[*appsv1.StatefulSet](sdk, moc, emitEvents)
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
	updatedStatus.StatefulSets = deleteUnusedResources[*appsv1.StatefulSetList](sdk, moc, statefulSetNames, ls, emitEvents)
	sort.Strings(updatedStatus.StatefulSets)

	updatedStatus.Services = deleteUnusedResources[*v1.ServiceList](sdk, moc, serviceNames, ls, emitEvents)
	sort.Strings(updatedStatus.Services)

	podList, _ := List[*v1.Pod, *v1.PodList](context.TODO(), sdk, moc, makeLabelsForMatrixone(moc.Name), emitEvents)
	updatedStatus.Pods = getObjectNames(podList)
	sort.Strings(updatedStatus.Pods)

	err = matrixnoeClusterStatusPatcher(sdk, updatedStatus, moc, emitEvents)
	if err != nil {
		return err
	}

	return nil
}

func sdkCreateOrUpdateAsNeeded[T object](
	sdk client.Client,
	obj T,
	isEqualFn func(prev, curr T) bool,
	updaterFn func(prev, curr T),
	moc *v1alpha1.MatrixoneCluster,
	names map[string]bool,
	emitEvent EventEmitter,
) (ClusterStatus, error) {
	names[obj.GetName()] = true

	addOwnerRefToObject(obj, asOwner(moc))
	addHashToObject(obj)
	prevObj := newObject[T]()
	if err := sdk.Get(context.TODO(), *namespacedName(obj.GetName(), obj.GetNamespace()), prevObj); err != nil {
		if apierrors.IsNotFound(err) {
			// resource dose not exist, create it.
			create, err := Create(context.TODO(), sdk, moc, obj, emitEvent)
			if err != nil {
				return "", err
			}
			return create, nil
		}
		e := fmt.Errorf("failed to get [%s:%s] due to [%s]",
			obj.GetObjectKind().GroupVersionKind().Kind, obj.GetName(), err.Error())
		logger.Error(e, e.Error(), "prev object", stringifyForLogging(prevObj, moc), "name", moc.Name, "namespace", moc.Namespace)
		emitEvent.EmitEventGeneric(moc, string(matrixoneObjectGetFail), "", err)
		return "", err
	}
	if obj.GetAnnotations()[matrixoneOpResourceHash] != prevObj.GetAnnotations()[matrixoneOpResourceHash] || !isEqualFn(prevObj, obj) {
		obj.SetResourceVersion(prevObj.GetResourceVersion())
		updaterFn(prevObj, obj)
		update, err := Update(context.TODO(), sdk, moc, obj, emitEvent)
		if err != nil {
			return "", err
		}
		return update, err

	}
	return "", nil
}

func isObjectFullyDeployed[T object](sdk client.Client, moc *v1alpha1.MatrixoneCluster, emitEvent EventEmitter) (bool, error) {
	obj, err := Get[T](context.TODO(), sdk, moc, emitEvent)
	if err != nil {
		return false, err
	}
	switch v := (any(obj)).(type) {
	case *appsv1.StatefulSet:
		return isStatefulsetFullyDeployed(v)
	case *appsv1.Deployment:
		return isDeploymentFullyDeployed(v)
	}
	return false, nil
}

func isStatefulsetFullyDeployed(sts *appsv1.StatefulSet) (bool, error) {
	if sts.Status.CurrentRevision != sts.Status.UpdateRevision {
		return false, nil
	} else if sts.Status.CurrentReplicas != sts.Status.ReadyReplicas {
		return false, nil
	} else {
		return sts.Status.CurrentRevision == sts.Status.UpdateRevision, nil
	}
}

func isDeploymentFullyDeployed(obj *appsv1.Deployment) (bool, error) {
	for _, condition := range obj.Status.Conditions {
		// This detects a failure condition, operator should send a rolling deployment failed event
		if condition.Type == appsv1.DeploymentReplicaFailure {
			return false, errors.New(condition.Reason)
		} else if condition.Type == appsv1.DeploymentProgressing && condition.Status != corev1.ConditionTrue || obj.Status.ReadyReplicas != obj.Status.Replicas {
			return false, nil
		} else {
			return obj.Status.ReadyReplicas == obj.Status.Replicas, nil
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
	sha, err := getObjectHash(obj)
	if err != nil {
		return err
	}
	annotations := obj.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
		obj.SetAnnotations(annotations)
	}
	annotations[matrixoneOpResourceHash] = sha
	return nil
}

func getObjectHash(obj object) (string, error) {
	bytes, err := json.Marshal(obj)

	if err != nil {
		return "", err
	}
	sha1Bytes := sha1.Sum(bytes)
	return base64.StdEncoding.EncodeToString(sha1Bytes[:]), nil
}

func namespacedName(name, namespace string) *types.NamespacedName {
	return &types.NamespacedName{Name: name, Namespace: namespace}
}

func alwaysTrueIsEqualsFn[T object](prev, curr T) bool {
	return true
}

func stringifyForLogging(obj object, moc *v1alpha1.MatrixoneCluster) string {
	bytes, err := json.Marshal(obj)
	if err != nil {
		logger.Error(err, err.Error(), fmt.Sprintf("failed to serialize [%s:%s]", obj.GetObjectKind().GroupVersionKind().Kind, obj.GetName()), "name", moc.Name, "namespace", moc.Namespace)
		return fmt.Sprintf("%v", obj)
	}
	return string(bytes)

}

func updateFn[T object](prev, curr T) {}

func execCheckCrashStatus(sdk client.Client, moc *v1alpha1.MatrixoneCluster, event EventEmitter) {
	if moc.Spec.ForceDeleteStsPodOnError == false {
		return
	}
	if moc.Spec.PodManagementPolicy == "OrderedReady" {
		checkCrashStatus(sdk, moc, event)
	}
}

func makeLabelsForMatrixone(name string) map[string]string {
	return map[string]string{"app": "matrixone", "matrixone_cr": name}
}

func checkCrashStatus(sdk client.Client, moc *v1alpha1.MatrixoneCluster, emitEvents EventEmitter) error {

	podList, err := List[*v1.Pod, *v1.PodList](context.TODO(), sdk, moc, makeLabelsForMatrixone(moc.Name), emitEvents)
	if err != nil {
		return err
	}

	for _, p := range podList {
		if p.Status.ContainerStatuses[0].RestartCount > 1 {
			for _, condition := range p.Status.Conditions {
				// condition.type Ready means the pod is able to service requests
				if condition.Type == corev1.ContainersReady {
					// the below condition evalutes if a pod is in
					// 1. pending state 2. failed state 3. unknown state
					// OR condtion.status is false which evalutes if neither of these conditions are met
					// 1. ContainersReady 2. PodInitialized 3. PodReady 4. PodScheduled
					if p.Status.Phase != corev1.PodRunning || condition.Status == corev1.ConditionFalse {
						err := Delete(context.TODO(), sdk, moc, p, emitEvents, &client.DeleteOptions{})
						if err != nil {
							return err
						}
						msg := fmt.Sprintf("deleted pod [%s] in namespace [%s], since it was in crashloopback state", p.GetName(), p.GetNamespace())
						logger.Info(msg, "object", stringifyForLogging(p, moc), "name", moc.Name, "namespace", moc.Namespace)
					}
				}
			}
		}
	}

	return nil
}

func deleteUnusedResources[TList objectList](sdk client.Client, moc *v1alpha1.MatrixoneCluster,
	names map[string]bool, selectorLabels map[string]string, emitEvents EventEmitter) []string {

	listOpts := []client.ListOption{
		client.InNamespace(moc.Namespace),
		client.MatchingLabels(selectorLabels),
	}

	survivorNames := make([]string, 0, len(names))

	listObj := newObject[TList]()

	if err := sdk.List(context.TODO(), listObj, listOpts...); err != nil {
		e := fmt.Errorf("failed to list [%s] due to [%s]", listObj.GetObjectKind().GroupVersionKind().Kind, err.Error())
		logger.Error(e, e.Error(), "name", moc.Name, "namespace", moc.Namespace)
	} else {
		items, err := extractList[object](listObj)
		if err != nil {
			logger.Error(err, err.Error(), "name", moc.Name, "namespace", moc.Namespace)
		}
		for _, s := range items {
			if names[s.GetName()] == false {
				err := Delete(context.TODO(), sdk, moc, s, emitEvents, &client.DeleteOptions{})
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

	podList, err := List[*v1.Pod, *v1.PodList](context.TODO(), sdk, moc, makeLabelsForMatrixone(moc.Name), emitEvents)
	if err != nil {
		return err
	}

	pvcLabels := map[string]string{
		"matrixone_cr": moc.Name,
	}

	pvcList, err := List[*v1.PersistentVolumeClaim, *v1.PersistentVolumeClaimList](context.TODO(), sdk, moc, pvcLabels, emitEvents)
	if err != nil {
		return err
	}

	for _, pod := range podList {
		if pod.Status.Phase != corev1.PodRunning {
			return nil
		}
		for _, status := range pod.Status.Conditions {
			if status.Status != corev1.ConditionTrue {
				return nil
			}
		}
	}

	mountedPVC := make([]string, len(podList))
	for _, pod := range podList {
		if pod.Spec.Volumes != nil {
			for _, vol := range pod.Spec.Volumes {
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
				err := Delete(context.TODO(), sdk, moc, pvcList[i], emitEvents, &client.DeleteAllOfOptions{})
				if err != nil {
					return err
				}
				msg := fmt.Sprintf("deleted orphaned pvc [%s:%s] successfully", pvcList[i].GetName(), moc.Namespace)
				logger.Info(msg, "name", moc.Name, "namespace", moc.Namespace)
			}
		}
	}
	return nil
}

func getObjectNames[T object](objList []T) []string {
	var names []string
	for _, obj := range objList {
		names = append(names, obj.GetName())
	}
	return names
}

// wrapper to patch matrixone cluster status
func matrixnoeClusterStatusPatcher(sdk client.Client, updatedStatus v1alpha1.MatrixoneClusterStatus, moc *v1alpha1.MatrixoneCluster, emitEvent EventEmitter) error {

	if !reflect.DeepEqual(updatedStatus, moc.Status) {
		patchBytes, err := json.Marshal(map[string]v1alpha1.MatrixoneClusterStatus{"status": updatedStatus})
		if err != nil {
			return fmt.Errorf("failed to serialize status patch to bytes: %v", err)
		}
		_ = Patch(context.TODO(), sdk, moc, moc, true, client.RawPatch(types.MergePatchType, patchBytes), emitEvent)
	}
	return nil
}

func executeFinalizers(sdk client.Client, moc *v1alpha1.MatrixoneCluster, emitEvents EventEmitter) error {

	if utils.ContainsString(moc.ObjectMeta.Finalizers, finalizerName) {
		pvcLabels := map[string]string{
			"matrixone_cr": moc.Name,
		}

		pvcList, err := List[*v1.PersistentVolumeClaim, *v1.PersistentVolumeClaimList](context.TODO(), sdk, moc, pvcLabels, emitEvents)
		if err != nil {
			return err
		}

		stsList, err := List[*appsv1.StatefulSet, *appsv1.StatefulSetList](context.TODO(), sdk, moc, makeLabelsForMatrixone(moc.Name), emitEvents)
		if err != nil {
			return err
		}

		msg := fmt.Sprintf("trigerring finalizer for CR [%s] in namespace [%s]", moc.Name, moc.Namespace)
		logger.Info(msg)

		err = deleteSTSAndPVC(sdk, moc, stsList, pvcList, emitEvents)
		if err != nil {
			return err
		}
		msg = fmt.Sprintf("finalizer success for CR [%s] in namespace [%s]", moc.Name, moc.Namespace)
		logger.Info(msg)

		// remove our finalizer from the list and update it.
		moc.ObjectMeta.Finalizers = utils.RemoveString(moc.ObjectMeta.Finalizers, finalizerName)

		_, err = Update(context.TODO(), sdk, moc, moc, emitEvents)
		if err != nil {
			return err
		}

	}
	return nil

}

func deleteSTSAndPVC(sdk client.Client, moc *v1alpha1.MatrixoneCluster, stsList []*appsv1.StatefulSet, pvcList []*v1.PersistentVolumeClaim, emitEvents EventEmitter) error {

	for i := range stsList {
		err := Delete(context.TODO(), sdk, moc, stsList[i], emitEvents, &client.DeleteAllOfOptions{})
		if err != nil {
			return err
		}
	}

	for i := range pvcList {
		err := Delete(context.TODO(), sdk, moc, pvcList[i], emitEvents, &client.DeleteAllOfOptions{})
		if err != nil {
			return err
		}
	}

	return nil
}

func checkIfCRExists(sdk client.Client, moc *v1alpha1.MatrixoneCluster, emitEvents EventEmitter) bool {
	_, err := Get[*v1alpha1.MatrixoneCluster](context.TODO(), sdk, moc, emitEvents)
	if err != nil {
		return false
	}
	return true
}
