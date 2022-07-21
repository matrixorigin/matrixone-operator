package logset

import (
	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	recon "github.com/matrixorigin/matrixone-operator/runtime/pkg/reconciler"
	kruisev1 "github.com/openkruise/kruise-api/apps/v1beta1"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/json"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	BootstrapAnnoKey = "logset.matrixorigin.io/bootstrap"

	IDRangeStart int = 131072
	IDRangeEnd   int = 262144
)

var _ recon.Actor[*v1alpha1.LogSet] = &LogSetActor{}

type LogSetActor struct{}

func (r *LogSetActor) Observe(ctx *recon.Context[*v1alpha1.LogSet]) (recon.Action[*v1alpha1.LogSet], error) {
	ls := ctx.Obj
	sts := &kruisev1.StatefulSet{}
	err := ctx.Get(client.ObjectKey{Namespace: ls.Namespace, Name: stsName(ls)}, sts)
	if err != nil && apierrors.IsNotFound(err) {
		return r.Create, nil
	} else if err != nil {
		return nil, errors.Wrap(err, "get subresource statefulset")
	}
	// TODO(aylei): add scaling / rolling-update / failover Actions
	return nil, nil
}

func (r *LogSetActor) Finalize(ctx *recon.Context[*v1alpha1.LogSet]) (bool, error) {
	ls := ctx.Obj
	// subresources should be deleted by owner reference
	svcExist, err := ctx.CheckExists(client.ObjectKey{Namespace: ls.Namespace, Name: headlessSvcName(ls)}, &corev1.Service{})
	if err != nil {
		return false, err
	}
	stsExist, err := ctx.CheckExists(client.ObjectKey{Namespace: ls.Namespace, Name: stsName(ls)}, &kruisev1.StatefulSet{})
	if err != nil {
		return false, err
	}
	return (!svcExist) && (!stsExist), nil
}

func (r *LogSetActor) Create(ctx *recon.Context[*v1alpha1.LogSet]) error {
	ls := ctx.Obj
	svc := buildHeadlessSvc(ls)
	sts := buildStatefulSet(ls, svc)
	syncReplicas(ls, sts)
	syncPodMeta(ls, sts)
	syncPodSpec(ls, sts)
	syncPersistentVolumeClaim(ls, sts)
	// TODO(aylei): automatically add ownerReference when create (maybe another method?)
	if err := ctx.Create(svc); err != nil {
		return errors.Wrap(err, "create headless service")
	}
	if err := ctx.Create(sts); err != nil {
		return errors.Wrap(err, "create statefulset")
	}
	return nil
}

type bootstrapReplica struct {
	ordinal int
	uuid    int
}

func (r *LogSetActor) bootstrap(ctx *recon.Context[*v1alpha1.LogSet]) ([]bootstrapReplica, error) {
	var replicas []bootstrapReplica
	previousDecision, hasBootstrapped := ctx.Obj.GetAnnotations()[BootstrapAnnoKey]
	if hasBootstrapped {
		if err := json.Unmarshal([]byte(previousDecision), &replicas); err != nil {
			return nil, errors.Wrap(err, "error deserialize boostrap replicas")
		}
		return replicas, nil
	}

	// if the bootstrap decision has not yet been made,pick a bootstrap decision
	n := *ctx.Obj.Spec.InitialConfig.HAKeeperReplicas
	// pick first N pods as initial HAKeeperReplicas
	for i := 0; i < n; i++ {
		uid := IDRangeStart + i
		if uid > IDRangeEnd {
			return nil, errors.Errorf("UID %d exceed range, max allowed: %d", uid, IDRangeEnd)
		}
		replicas = append(replicas, bootstrapReplica{
			ordinal: i,
			uuid:    uid,
		})
	}
	serialized, err := json.Marshal(replicas)
	if err != nil {
		return nil, errors.Wrap(err, "error serialize bootstrap replicas")
	}
	annos := ctx.Obj.GetAnnotations()
	annos[BootstrapAnnoKey] = string(serialized)
	ctx.Obj.SetAnnotations(annos)
	return replicas, ctx.Update(ctx.Obj)
}
