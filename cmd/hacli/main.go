// Copyright 2024 Matrix Origin
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

package main

import (
	"context"
	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	"github.com/matrixorigin/matrixone-operator/pkg/controllers/common"
	"github.com/matrixorigin/matrixone/pkg/logservice"
	logpb "github.com/matrixorigin/matrixone/pkg/pb/logservice"
	"github.com/matrixorigin/matrixone/pkg/pb/metadata"
	"go.uber.org/zap"
	logzap "sigs.k8s.io/controller-runtime/pkg/log/zap"
	"time"
)

func main() {
	zapLogger := logzap.NewRaw()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cli, err := logservice.NewProxyHAKeeperClient(ctx, logservice.HAKeeperClientConfig{ServiceAddresses: []string{"127.0.0.1:32001"}})
	if err != nil {
		panic(err)
	}
	cluster, err := cli.GetClusterDetails(ctx)
	if err != nil {
		panic(err)
	}
	store := cluster.CNStores[0]
	err = cli.PatchCNStore(ctx, logpb.CNStateLabel{
		UUID:  store.UUID,
		State: metadata.WorkState_Working,
		Labels: common.ToStoreLabels([]v1alpha1.CNLabel{{
			Key:    "test",
			Values: []string{"testv"},
		}}),
	})
	if err != nil {
		panic(err)
	}
	err = cli.PatchCNStore(ctx, logpb.CNStateLabel{
		UUID:  store.UUID,
		State: metadata.WorkState_Working,
		Labels: common.ToStoreLabels([]v1alpha1.CNLabel{{
			Key:    "test",
			Values: []string{"testv"},
		}}),
	})
	if err != nil {
		panic(err)
	}
	err = cli.UpdateCNLabel(ctx, logpb.CNStoreLabel{
		UUID:   store.UUID,
		Labels: nil,
	})
	if err != nil {
		panic(err)
	}
	cluster, err = cli.GetClusterDetails(ctx)
	if err != nil {
		panic(err)
	}
	zapLogger.Info("resp", zap.Any("resp", cluster.CNStores[0].Labels))
}
