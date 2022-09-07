// Copyright 2022 Matrix Origin
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
package logset

import (
	"fmt"
	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	recon "github.com/matrixorigin/matrixone-operator/runtime/pkg/reconciler"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/json"
)

const (
	bootstrapFile = "bootstrap.toml"
)

type bootstrapReplica struct {
	Ordinal   int `json:"ordinal"`
	ReplicaID int `json:"replicaId"`
}

// buildBootstrapConfig build the configmap that contains bootstrap information for log service
func buildBootstrapConfig(ctx *recon.Context[*v1alpha1.LogSet]) (*corev1.ConfigMap, error) {
	ls := ctx.Obj
	brs, err := bootstrap(ctx)
	if err != nil {
		return nil, err
	}
	m := map[string]interface{}{
		"bootstrap-cluster":         true,
		"num-of-log-shards":         ls.Spec.InitialConfig.LogShards,
		"num-of-dn-shards":          ls.Spec.InitialConfig.DNShards,
		"num-of-log-shard-replicas": ls.Spec.InitialConfig.LogShardReplicas,
		"init-hakeeper-members":     encodeSeeds(brs),
	}
	t := v1alpha1.NewTomlConfig(map[string]interface{}{})
	t.Set([]string{"logservice", "BootstrapConfig"}, m)
	c, err := t.ToString()
	if err != nil {
		return nil, err
	}
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ls.Namespace,
			Name:      bootstrapConfigMapName(ls),
		},
		Data: map[string]string{
			bootstrapFile: c,
		},
	}, nil
}

func bootstrap(ctx *recon.Context[*v1alpha1.LogSet]) ([]bootstrapReplica, error) {
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
		rid := IDRangeStart + i
		if rid > IDRangeEnd {
			return nil, errors.Errorf("ReplicaID %d exceed range, max allowed: %d", rid, IDRangeEnd)
		}
		replicas = append(replicas, bootstrapReplica{
			Ordinal:   i,
			ReplicaID: rid,
		})
	}
	serialized, err := json.Marshal(replicas)
	if err != nil {
		return nil, errors.Wrap(err, "error serialize bootstrap replicas")
	}
	if ctx.Obj.Annotations == nil {
		ctx.Obj.Annotations = map[string]string{}
	}
	ctx.Obj.Annotations[BootstrapAnnoKey] = string(serialized)
	return replicas, ctx.Update(ctx.Obj)
}

// encodeSeeds encode the bootstrap replicas decision to the configuration format
// accepted by logservice
func encodeSeeds(brs []bootstrapReplica) []string {
	var seeds []string
	for _, r := range brs {
		seeds = append(seeds, fmt.Sprintf("%d:%s", r.ReplicaID, encodeOrdinal(r.Ordinal)))
	}
	return seeds
}

// encodeOrdinal encode the pod ordinal to UUID
func encodeOrdinal(ordinal int) string {
	return fmt.Sprintf("00000000-0000-0000-0000-%012x", ordinal)
}

func bootstrapConfigMapName(ls *v1alpha1.LogSet) string {
	return resourceName(ls) + "-bootstrap"
}
