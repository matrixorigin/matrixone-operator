package common

import (
	"encoding/json"
	"fmt"

	"github.com/cespare/xxhash"
	recon "github.com/matrixorigin/matrixone-operator/runtime/pkg/reconciler"
	"github.com/matrixorigin/matrixone-operator/runtime/pkg/util"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	InstanceLabelKey  = "matrixorigin.io/instance"
	ComponentLabelKey = "matrixorigin.io/component"
	// NamespaceLabelKey is the label key for cluster-scope resources
	NamespaceLabelKey = "matrixorigin.io/namespace"
)

// SubResourceLabels generate labels for sub-resources
func SubResourceLabels(owner client.Object) map[string]string {
	return map[string]string{
		NamespaceLabelKey: owner.GetNamespace(),
		InstanceLabelKey:  owner.GetName(),
		ComponentLabelKey: owner.GetObjectKind().GroupVersionKind().Kind,
	}
}

// SyncTopology syncs the topology even spread of PodSet to the underlying pods
func SyncTopology(domains []string, podSpec *corev1.PodSpec) {
	var constraints []corev1.TopologySpreadConstraint
	for _, domain := range domains {
		constraints = append(constraints, corev1.TopologySpreadConstraint{
			MaxSkew:           1,
			TopologyKey:       domain,
			WhenUnsatisfiable: corev1.DoNotSchedule,
		})
	}
	podSpec.TopologySpreadConstraints = constraints
}

// SyncConfigMap syncs the desired configmap for pods, which will cause rolling-update if the
// data of the configmap is changed
func SyncConfigMap(kubeCli recon.KubeClient, podSpec *corev1.PodSpec, cm *corev1.ConfigMap) error {
	var currentCmName string
	vp := util.FindFirst(podSpec.Volumes, util.WithVolumeName("config"))
	if vp != nil {
		currentCmName = vp.Name
	}
	// TODO(aylei): GC stale configmaps (maybe in another worker?)
	desiredName, err := ensureConfigMap(kubeCli, currentCmName, cm)
	if err != nil {
		return err
	}
	if vp != nil {
		// update existing config volume ref
		if vp.VolumeSource.ConfigMap == nil {
			return errors.New("config volume must be sourced by a ConfigMap")
		}
		vp.VolumeSource.ConfigMap.Name = desiredName
	} else {
		// insert new config volume ref
		podSpec.Volumes = append(podSpec.Volumes, corev1.Volume{
			Name: "config",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: desiredName,
					},
				},
			},
		})
	}
	return nil
}

// ensureConfigMap ensures the configmap exist in k8s
func ensureConfigMap(kubeCli recon.KubeClient, currentCm string, desired *corev1.ConfigMap) (string, error) {
	c := desired.DeepCopy()
	if err := addConfigMapDigest(c); err != nil {
		return "", err
	}
	// config not changed, nothing to do
	if c.Name == currentCm {
		return currentCm, nil
	}
	// otherwise ensure the configmap exists
	err := util.Ignore(apierrors.IsAlreadyExists, kubeCli.CreateOwned(c))
	if err != nil {
		return "", err
	}
	return c.Name, nil
}

func addConfigMapDigest(cm *corev1.ConfigMap) error {
	s, err := json.Marshal(cm.Data)
	if err != nil {
		return err
	}
	sum := xxhash.Sum64(s)
	suffix := fmt.Sprintf("%x", sum)[0:7]
	cm.Name = fmt.Sprintf("%s-%s", cm.Name, suffix)
	return nil
}
