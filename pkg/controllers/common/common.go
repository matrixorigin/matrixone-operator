package common

import (
	"encoding/json"
	"fmt"
	"github.com/cespare/xxhash"
	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	recon "github.com/matrixorigin/matrixone-operator/runtime/pkg/reconciler"
	"github.com/matrixorigin/matrixone-operator/runtime/pkg/util"
	kruise "github.com/openkruise/kruise-api/apps/v1beta1"
	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type FileType string
type ServiceType string
type BackendType string

const (
	PodNameEnvKey     = "POD_NAME"
	HeadlessSvcEnvKey = "HEADLESS_SERVICE_NAME"
	NamespaceEnvKey   = "NAMESPACE"

	DataPath      = "/var/lib/matrixone/data"
	DataVolume    = "data"
	ConfigVolume  = "config"
	ConfigPath    = "/etc/matrixone/config"
	ConfigFile    = "config.toml"
	Entrypoint    = "start.sh"
	ListenAddress = "0.0.0.0"

	S3Service    FileType = "s3"
	LocalService FileType = "local"
	MinioService FileType = "minio"
	NFSService   FileType = "nfs"

	MemoryEngine BackendType = "MEM"
	TAEEngine    BackendType = "TAE"

	HakeeperPort  = 32001
	DNServicePort = 41010
	CNServicePort = 6001

	InstanceLabelKey  = "matrixorigin.io/instance"
	ComponentLabelKey = "matrixorigin.io/component"
	// NamespaceLabelKey is the label key for cluster-scope resources
	NamespaceLabelKey = "matrixorigin.io/namespace"

	FileBackendType = "DISK"

	AWSAccessKeyID     = "AWS_ACCESS_KEY_ID"
	AWSSecretAccessKey = "AWS_SECRET_ACCESS_KEY"
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

func SetStorageProviderConfig(sp v1alpha1.SharedStorageProvider, podSpec *corev1.PodSpec) {
	for i, _ := range podSpec.Containers {
		if s3p := sp.S3; s3p != nil {
			if s3p.SecretRef != nil {
				for _, key := range []string{AWSAccessKeyID, AWSSecretAccessKey} {
					podSpec.Containers[i].Env = util.UpsertByKey(podSpec.Containers[i].Env, corev1.EnvVar{Name: key, ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: *s3p.SecretRef,
							Key:                  key,
						},
					}}, util.EnvVarKey)
				}
			}
		}
	}
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
			Name:         "config",
			VolumeSource: util.ConfigMapVolume(desiredName),
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

// HeadlessServiceTemplate returns a headless service as template
// https://kubernetes.io/docs/concepts/services-networking/service/#headless-services
func HeadlessServiceTemplate(obj client.Object, name string) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: ObjMetaTemplate(obj, name),
		Spec: corev1.ServiceSpec{
			ClusterIP: corev1.ClusterIPNone,
			Selector:  SubResourceLabels(obj),
		},
	}

}

func ServiceTemplate(obj client.Object, name string, st corev1.ServiceType) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: ObjMetaTemplate(obj, name),
		Spec: corev1.ServiceSpec{
			Type:     st,
			Selector: SubResourceLabels(obj),
		},
	}
}

// ObjMetaTemplate get object metadata
func ObjMetaTemplate[T client.Object](obj T, name string) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:        name,
		Namespace:   obj.GetNamespace(),
		Annotations: map[string]string{},
		Labels:      SubResourceLabels(obj),
	}
}

// StatefulSetTemplate return a kruise statefulset as template
func StatefulSetTemplate(obj client.Object, name string, svcName string) *kruise.StatefulSet {
	return &kruise.StatefulSet{
		ObjectMeta: ObjMetaTemplate(obj, name),
		Spec: kruise.StatefulSetSpec{
			ServiceName: svcName,
			UpdateStrategy: kruise.StatefulSetUpdateStrategy{
				Type: appsv1.RollingUpdateStatefulSetStrategyType,
				RollingUpdate: &kruise.RollingUpdateStatefulSetStrategy{
					PodUpdatePolicy: kruise.InPlaceIfPossiblePodUpdateStrategyType,
				},
			},
			PodManagementPolicy: appsv1.ParallelPodManagement,
			Selector: &metav1.LabelSelector{
				MatchLabels: SubResourceLabels(obj),
			},
			PersistentVolumeClaimRetentionPolicy: &kruise.StatefulSetPersistentVolumeClaimRetentionPolicy{
				WhenDeleted: kruise.DeletePersistentVolumeClaimRetentionPolicyType,
				WhenScaled:  kruise.DeletePersistentVolumeClaimRetentionPolicyType,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      SubResourceLabels(obj),
					Annotations: map[string]string{},
				},
			},
		},
	}
}

// DeploymentTemplate return a deployment as template
func DeploymentTemplate(obj client.Object, name string) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: ObjMetaTemplate(obj, name),
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: SubResourceLabels(obj),
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      SubResourceLabels(obj),
					Annotations: map[string]string{},
				},
			},
		},
	}

}

// PersistentVolumeClaimTemplate returns a persistent volume claim object
func PersistentVolumeClaimTemplate(size resource.Quantity, sc *string, name string) corev1.PersistentVolumeClaim {
	return corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name: DataVolume,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources: corev1.ResourceRequirements{
				Requests: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceStorage: size,
				},
			},
			StorageClassName: sc,
		},
	}
}

// GetLocalFilesService  get local file service config
func GetLocalFilesService(path string) map[string]interface{} {
	return map[string]interface{}{
		"name":     LocalService,
		"backend":  FileBackendType,
		"data-dir": path,
	}
}

func S3FileServiceConfig(l *v1alpha1.LogSet) map[string]interface{} {
	return map[string]interface{}{
		"name":     S3Service,
		"backend":  FileBackendType,
		"data-dir": DataPath,
	}
}

// FileServiceConfig config common file service(local, s3 etc.) for all sets
func FileServiceConfig(fsPath, fsType string) (res map[string]interface{}) {
	switch fsType {
	case string(LocalService):
		// local file service
		res = map[string]interface{}{
			"name":     fsType,
			"backend":  LocalService,
			"data-dir": fsPath,
		}
	case string(S3Service):
	}

	return res
}
