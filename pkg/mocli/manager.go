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

package mocli

import (
	"context"
	"github.com/go-errors/errors"
	"github.com/go-logr/zapr"
	recon "github.com/matrixorigin/controller-runtime/pkg/reconciler"
	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	"github.com/matrixorigin/matrixone/pkg/logservice"
	"go.uber.org/zap"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sync"
	"time"
)

const (
	DefaultRPCTimeout = 10 * time.Second

	RefreshInterval = 15 * time.Second
)

type MORPCClientManager struct {
	logger  *zap.Logger
	done    chan struct{}
	kubeCli client.Client

	sync.Mutex
	logSetToHandler map[types.UID]handler
}

type handler struct {
	lsRef     types.NamespacedName
	clientSet *ClientSet
}

type ClientSet struct {
	Client     logservice.ProxyHAKeeperClient
	StoreCache *StoreCache

	LockServiceClient *LockServiceClient
}

func (h *ClientSet) Close() error {
	h.StoreCache.Close()
	h.LockServiceClient.Close()
	return h.Client.Close()
}

func NewManager(kubeCli client.Client, logger *zap.Logger) *MORPCClientManager {
	mgr := &MORPCClientManager{
		logger:          logger,
		done:            make(chan struct{}),
		kubeCli:         kubeCli,
		logSetToHandler: map[types.UID]handler{},
	}
	go mgr.gc()
	return mgr
}

func (m *MORPCClientManager) GetClient(ls *v1alpha1.LogSet) (*ClientSet, error) {
	// FIXME: this is would be bottleneck if we concurrently reconcile a large amount of
	// matrixone clusters, we can concurrently initialize the HAKeeper clients here if necessary.
	m.Lock()
	defer m.Unlock()
	if _, ok := m.logSetToHandler[ls.UID]; !ok {
		cs, err := m.newClientSet(ls)
		if err != nil {
			return nil, errors.WrapPrefix(err, "error new clientSet", 0)
		}
		m.logSetToHandler[ls.UID] = handler{
			clientSet: cs,
			lsRef:     client.ObjectKeyFromObject(ls),
		}
	}
	return m.logSetToHandler[ls.UID].clientSet, nil
}

func (m *MORPCClientManager) Close() {
	close(m.done)
}

func (m *MORPCClientManager) gc() {
	for {
		select {
		case <-time.Tick(30 * time.Second):
			m.doGC()
		case <-m.done:
			return
		}
	}
}

func (m *MORPCClientManager) doGC() {
	m.Lock()
	defer m.Unlock()
	for uid, v := range m.logSetToHandler {
		closeFn := func() {
			err := v.clientSet.Close()
			if err != nil {
				m.logger.Error("error closing HAKeeper client", zap.Error(err), zap.Any("logset", v.lsRef), zap.Any("uid", uid))
			}
		}
		ls := &v1alpha1.LogSet{}
		err := m.kubeCli.Get(context.TODO(), v.lsRef, ls)
		if err != nil {
			if apierrors.IsNotFound(err) {
				m.logger.Info("logset deleted, clean clientet", zap.Any("logset", v.lsRef))
				delete(m.logSetToHandler, uid)
				closeFn()
				continue
			}
			m.logger.Error("error gc HAKeeper client", zap.Error(err), zap.Any("logset", v.lsRef), zap.Any("uid", uid))
			continue
		}
		// logset has been re-created, clean stale cache
		if ls.UID != uid && recon.IsReady(ls) {
			m.logger.Info("logset recreated, clean legeacy clientet", zap.Any("logset", v.lsRef), zap.Any("old UID", uid), zap.Any("new UID", ls.UID))
			delete(m.logSetToHandler, uid)
			closeFn()
		}
	}
}

func (m *MORPCClientManager) newClientSet(ls *v1alpha1.LogSet) (*ClientSet, error) {
	ctx, cancel := context.WithTimeout(context.Background(), DefaultRPCTimeout)
	defer cancel()
	if ls.Status.Discovery.String() == "" {
		return nil, errors.Errorf("logset discovery address not ready, logset: %s/%s", ls.Namespace, ls.Name)
	}

	cli, err := logservice.NewProxyHAKeeperClient(ctx, logservice.HAKeeperClientConfig{DiscoveryAddress: ls.Status.Discovery.String()})
	if err != nil {
		return nil, errors.WrapPrefix(err, "build HAKeeper client", 0)
	}

	mc := NewCNCache(cli, RefreshInterval, zapr.NewLogger(m.logger.Named(ls.Name+"-store-cache")))
	tn, ok := mc.GetTN()
	if !ok {
		return nil, errors.Errorf("TN service not found, logset: %s/%s", ls.Namespace, ls.Name)
	}
	lcc, err := NewLockServiceClient(tn.LockServiceAddress, m.logger.Named("lockservice-cli"))
	if err != nil {
		return nil, errors.WrapPrefix(err, "build lockservice Client", 0)
	}
	handler := &ClientSet{
		Client:            cli,
		LockServiceClient: lcc,
		StoreCache:        mc,
	}
	return handler, nil
}
