package reconciler

import (
	"fmt"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func CreateOwnedOrUpdate(kubeCli KubeClient, obj client.Object, mutateFn func() error) error {
	key := client.ObjectKeyFromObject(obj)
	if err := kubeCli.Get(key, obj); err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
		if err := mutate(mutateFn, key, obj); err != nil {
			return err
		}
		return kubeCli.CreateOwned(obj)
	}

	existing := obj.DeepCopyObject()
	if err := mutate(mutateFn, key, obj); err != nil {
		return err
	}

	if equality.Semantic.DeepEqual(existing, obj) {
		return nil
	}

	if err := kubeCli.Update(obj); err != nil {
		return err
	}
	return nil
}

func mutate(f func() error, key client.ObjectKey, obj client.Object) error {
	if err := f(); err != nil {
		return err
	}
	if newKey := client.ObjectKeyFromObject(obj); key != newKey {
		return fmt.Errorf("MutateFn cannot mutate object name and/or object namespace")
	}
	return nil
}
