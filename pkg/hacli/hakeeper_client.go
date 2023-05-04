// Copyright 2023 Matrix Origin
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

package hacli

import (
	"context"
	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	"github.com/matrixorigin/matrixone/pkg/logservice"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sync"
	"time"
)

const (
	HAKeeperTimeout = 10 * time.Second
)

type HAKeeperClientManager struct {
	kubeCli client.Client
	sync.Mutex
	logSetToClients map[types.NamespacedName]logservice.ProxyHAKeeperClient
}

func NewManager(kubeCli client.Client) *HAKeeperClientManager {
	return &HAKeeperClientManager{
		kubeCli:         kubeCli,
		logSetToClients: map[types.NamespacedName]logservice.ProxyHAKeeperClient{},
	}
}

func (m *HAKeeperClientManager) GetClient(logSetRef types.NamespacedName) (logservice.ProxyHAKeeperClient, error) {
	// FIXME: this is would be bottleneck if we concurrently reconcile a large amount of
	// matrixone clusters, we can concurrently initialize the HAKeeper clients here if necessary.
	m.Lock()
	defer m.Unlock()
	if _, ok := m.logSetToClients[logSetRef]; !ok {
		cli, err := m.newHAKeeperClient(logSetRef)
		if err != nil {
			return nil, err
		}
		m.logSetToClients[logSetRef] = cli
	}
	return m.logSetToClients[logSetRef], nil
}

func (m *HAKeeperClientManager) newHAKeeperClient(logsetRef types.NamespacedName) (logservice.ProxyHAKeeperClient, error) {
	ls := &v1alpha1.LogSet{}
	ctx, cancel := context.WithTimeout(context.Background(), HAKeeperTimeout)
	defer cancel()
	err := m.kubeCli.Get(ctx, logsetRef, ls)
	if err != nil {
		return nil, errors.Wrap(err, "get LogSet")
	}
	cli, err := logservice.NewProxyHAKeeperClient(ctx, logservice.HAKeeperClientConfig{DiscoveryAddress: ls.Status.Discovery.String()})
	if err != nil {
		return nil, errors.Wrap(err, "build HAKeeper client")
	}
	return cli, nil
}
