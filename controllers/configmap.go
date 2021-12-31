package controllers

import (
	"regexp"

	matrixonev1alpha1 "github.com/matrixorigin/matrixone-operator/api/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	configFileName string = "system_vars_config.toml"
)

func (r *MatrixoneClusterReconciler) makeConfigMap(cm *v1.ConfigMap, moc *matrixonev1alpha1.MatrixoneCluster, ls map[string]string) (*v1.ConfigMap, error) {

	cm.TypeMeta = metav1.TypeMeta{
		APIVersion: "v1",
		Kind:       "ConfigMap",
	}

	cm.ObjectMeta = metav1.ObjectMeta{
		Name:      moc.Spec.PodName.Value,
		Namespace: moc.Namespace,
	}

	cm.Labels = ls

	val, err := generateData(moc)
	if err != nil {
		return cm, err
	}

	cm.Data = map[string]string{
		configFileName: val,
	}

	if err := ctrl.SetControllerReference(moc, cm, r.Scheme); err != nil {
		return cm, err
	}

	return cm, nil
}

func generateData(moc *matrixonev1alpha1.MatrixoneCluster) (string, error) {
	join := ""
	host := moc.Spec.PodName.Value

	addrRaft := "0.0.0.0:10000"
	AddrAdvertiseRaft := host + ":10000"
	addrClient := "0.0.0.0:20000"
	AddrAdvertiseClient := host + ":20000"
	dirData := moc.Spec.StorePath
	rpcAddr := "0.0.0.0:30000"
	rpcAdertiseAddr := host + ":30000"
	clientURLs := "http://0.0.0.0:40000"
	advertiseClientURLs := "http://" + host + ":40000"
	peerURLs := "http://0.0.0.0:50000"
	advertisePeerURLS := "http://" + host + ":50000"

	reg := regexp.MustCompile(moc.Spec.PodName.ValueFrom.FieldRef.FieldPath)

	if reg.MatchString("-0") {
		join = ""
	} else {
		join = "http://" + moc.Name + "-0" + ":40000"
	}

	mn := moc.Name + "-" + moc.Namespace

	metric := Metric{
		Addr:     moc.Spec.MetircAddr,
		Interval: 1,
		Job:      mn,
		Instance: mn,
	}
	replica := Replica{
		ShardCapacityBytes: moc.Spec.ShardCapacityBytes,
	}
	raft := Raft{
		MaxEntryBytes: moc.Spec.MaxEntryBytes,
	}
	prophet := Prophet{
		Name:             moc.Name,
		RPCAddr:          rpcAddr,
		RPCAdvertiseAddr: rpcAdertiseAddr,
		StorageNode:      true,
	}
	pee := ProphetEmbedEtcd{
		Join:                join,
		ClientURLs:          clientURLs,
		AdvertiseClientURLs: advertiseClientURLs,
		PeerURLs:            peerURLs,
		AdvertisePeerURLs:   advertisePeerURLS,
	}
	ps := ProphetSchedule{
		LowSpaceRatio:   moc.Spec.LowSpaceRatio,
		HighSpaceRation: moc.Spec.HighSpaceRation,
	}
	pr := ProphetReplication{
		MaxReplicas: moc.Spec.MaxReplicas,
	}

	moConfig := MatrixoneConfig{
		AddrRaft:            addrRaft,
		AddrAdvertiseRaft:   AddrAdvertiseRaft,
		AddrClient:          addrClient,
		AddrAdvertiseClient: AddrAdvertiseClient,
		DirData:             dirData,
		Metric:              metric,
		Replication:         replica,
		Raft:                raft,
		Prophet:             prophet,
		ProphetEmbedEtcd:    pee,
		ProphetSchedule:     ps,
		ProphetReplication:  pr,
	}

	value, err := MarshalTOML(moConfig)
	if err != nil {
		return "", err
	}

	return value, nil
}
