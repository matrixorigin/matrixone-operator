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
	"github.com/go-logr/logr"
	"github.com/matrixorigin/matrixone/pkg/logservice"
	logpb "github.com/matrixorigin/matrixone/pkg/pb/logservice"
	"github.com/matrixorigin/matrixone/pkg/pb/metadata"
	"sync"
	"time"
)

type StoreCache struct {
	logger          logr.Logger
	client          logservice.ProxyHAKeeperClient
	refreshInterval time.Duration
	mu              struct {
		sync.RWMutex
		cnServices map[string]metadata.CNService
	}
	done chan struct{}
}

func NewCNCache(
	client logservice.ProxyHAKeeperClient,
	refreshInterval time.Duration,
	logger logr.Logger) *StoreCache {
	c := &StoreCache{
		client:          client,
		logger:          logger,
		refreshInterval: refreshInterval,
		done:            make(chan struct{}),
	}
	c.mu.cnServices = make(map[string]metadata.CNService, 1024)
	c.doRefresh()
	go c.refresh()
	return c
}

func (c *StoreCache) GetCN(uuid string) (metadata.CNService, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	cn, ok := c.mu.cnServices[uuid]
	return cn, ok
}

func (c *StoreCache) refresh() {
	for {
		select {
		case <-time.Tick(c.refreshInterval):
			c.doRefresh()
		case <-c.done:
			return
		}
	}
}

func (c *StoreCache) Close() {
	close(c.done)
}

func (c *StoreCache) doRefresh() {
	c.logger.Info("refresh from HAKeeper")
	ctx, cancel := context.WithTimeout(context.Background(), c.refreshInterval)
	defer cancel()
	details, err := c.client.GetClusterDetails(ctx)
	if err != nil {
		c.logger.Error(err, "failed to refresh cluster details from hakeeper")
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	for k := range c.mu.cnServices {
		delete(c.mu.cnServices, k)
	}
	for _, cn := range details.CNStores {
		v := newCNService(cn)
		c.mu.cnServices[cn.UUID] = v
	}
}

func newCNService(cn logpb.CNStore) metadata.CNService {
	return metadata.CNService{
		ServiceID:              cn.UUID,
		PipelineServiceAddress: cn.ServiceAddress,
		SQLAddress:             cn.SQLAddress,
		LockServiceAddress:     cn.LockServiceAddress,
		WorkState:              cn.WorkState,
		Labels:                 cn.Labels,
		QueryAddress:           cn.QueryAddress,
	}
}
