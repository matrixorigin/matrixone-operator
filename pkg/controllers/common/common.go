package common

import (
	"encoding/json"
	"fmt"
	"github.com/cespare/xxhash"
	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	recon "github.com/matrixorigin/matrixone-operator/runtime/pkg/reconciler"
	"github.com/matrixorigin/matrixone-operator/runtime/pkg/util"
	kruise "github.com/openkruise/kruise-api/apps/v1alpha1"
	"github.com/pkg/errors"
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
	svcSuffix    = "-discovery"
	hSvcSuffix   = ""
	configSuffix = "-config"

	PodNameEnvKey     = "POD_NAME"
	HeadlessSvcEnvKey = "HEADLESS_SERVICE_NAME"
	NamespaceEnvKey   = "NAMESPACE"
	PodIPEnvKey       = "POD_IP"

	DataPath      = "/var/lib/matrixone/data"
	DataVolume    = "data"
	ConfigVolume  = "config"
	ConfigPath    = "/etc/matrixone/config"
	ConfigFile    = "config.toml"
	Entrypoint    = "start.sh"
	ListenAddress = "0.0.0.0"

	DNService ServiceType = "DN"
	CNService ServiceType = "CN"

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

func getHeadlessSvcObjMeta(obj client.Object) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:        GetHeadlessSvcName(obj),
		Namespace:   GetNamespace(obj),
		Annotations: map[string]string{},
		Labels:      SubResourceLabels(obj),
	}
}

func getDiscoverySvcObjMeta(obj client.Object) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:        GetDiscoverySvcName(obj),
		Namespace:   GetNamespace(obj),
		Labels:      SubResourceLabels(obj),
		Annotations: map[string]string{},
	}
}

// GetHeadlessService create a headless service
// https://kubernetes.io/docs/concepts/services-networking/service/#headless-services
func GetHeadlessService(obj client.Object, ports []corev1.ServicePort) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: getHeadlessSvcObjMeta(obj),
		Spec: corev1.ServiceSpec{
			ClusterIP: corev1.ClusterIPNone,
			Ports:     ports,
			Selector:  SubResourceLabels(obj),
		},
	}

}

// GetDiscoveryService create a service with suffix "-discovery"
// https://kubernetes.io/docs/concepts/services-networking/service
func GetDiscoveryService(
	obj client.Object, ports []corev1.ServicePort, serviceType corev1.ServiceType) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: getDiscoverySvcObjMeta(obj),

		Spec: corev1.ServiceSpec{
			Type:     serviceType,
			Ports:    ports,
			Selector: SubResourceLabels(obj),
		},
	}
}

// GetObjMeta get object metadata
func GetObjMeta[T client.Object](obj T) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:        GetName(obj),
		Namespace:   GetNamespace(obj),
		Annotations: map[string]string{},
		Labels:      SubResourceLabels(obj),
	}
}

func GetConfigMapObjMeta(obj client.Object) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:        GetConfigName(obj),
		Namespace:   GetNamespace(obj),
		Annotations: map[string]string{},
		Labels:      SubResourceLabels(obj),
	}
}

// GetCloneSet get a kruise clone set object
func GetCloneSet(obj client.Object) *kruise.StatefulSet {
	return &kruise.StatefulSet{
		ObjectMeta: GetObjMeta(obj),
		Spec: kruise.StatefulSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: SubResourceLabels(obj),
			},
			ServiceName: GetHeadlessSvcName(obj),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: GetObjMeta(obj),
			},
		},
	}
}

// GetPersistentVolumeClaim return persistent volume claim object
func GetPersistentVolumeClaim(size resource.Quantity, sc *string) corev1.PersistentVolumeClaim {
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

// GetHeadlessSvcName get headless service name
func GetHeadlessSvcName(obj client.Object) string {
	return obj.GetName() + hSvcSuffix
}

// GetDiscoverySvcName get service name
func GetDiscoverySvcName(obj client.Object) string {
	return obj.GetName() + svcSuffix
}

// GetConfigName get configmap name
func GetConfigName(obj client.Object) string {
	return obj.GetName() + configSuffix
}

// GetName get object name
func GetName(obj client.Object) string {
	return obj.GetName()
}

// GetNamespace get object namespace
func GetNamespace(obj client.Object) string {
	return obj.GetNamespace()
}

// GetDiscoveryAdr get discovery service address
func GetDiscoveryAdr(obj client.Object) string {
	return fmt.Sprintf("%s.%s.svc", GetDiscoverySvcName(obj), GetNamespace(obj))
}

// GetLocalFilesService  get local file service config
func GetLocalFilesService() map[string]interface{} {
	return map[string]interface{}{
		"name":     LocalService,
		"backend":  FileBackendType,
		"data-dir": DataPath,
	}
}

func HAKeeperClientConfig(l *v1alpha1.LogSet) map[string]interface{} {
	if l.Status.Discovery == nil {
		return nil
	}
	return map[string]interface{}{
		"hakeeper-client": map[string]interface{}{
			"discovery-address": fmt.Sprintf("%s:%d", l.Status.Discovery.Address, l.Status.Discovery.Port),
		},
	}
}

func S3FileServiceConfig(l *v1alpha1.LogSet) map[string]interface{} {
	return map[string]interface{}{
		"name":     S3Service,
		"backend":  FileBackendType,
		"data-dir": DataPath,
	}
}
