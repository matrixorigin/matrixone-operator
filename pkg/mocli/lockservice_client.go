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
	"github.com/matrixorigin/matrixone/pkg/common/morpc"
	"github.com/matrixorigin/matrixone/pkg/pb/lock"
	pb "github.com/matrixorigin/matrixone/pkg/pb/query"
	gerrors "github.com/pkg/errors"
	"go.uber.org/zap"
)

type LockServiceClient struct {
	client morpc.RPCClient
	tnAddr string
}

func NewLockServiceClient(tnAddr string, logger *zap.Logger) (*LockServiceClient, error) {
	cfg := morpc.Config{}
	cfg.Adjust()
	cfg.BackendOptions = append(cfg.BackendOptions,
		morpc.WithBackendReadTimeout(DefaultRPCTimeout))
	client, err := cfg.NewClient("lock-client", logger, func() morpc.Message {
		return &pb.Response{}
	})
	if err != nil {
		return nil, gerrors.Wrap(err, "error create lock service client")
	}
	return &LockServiceClient{client: client, tnAddr: tnAddr}, nil
}

func (l *LockServiceClient) SetRestartCN(ctx context.Context, uuid string) (bool, error) {
	resp, err := l.sendToTN(ctx, &lock.Request{
		SetRestartService: lock.SetRestartServiceRequest{ServiceID: uuid},
		Method:            lock.Method_SetRestartService},
	)
	if err != nil {
		return false, gerrors.Wrap(err, "error set restart service request")
	}
	return resp.SetRestartService.OK, nil
}

func (l *LockServiceClient) CanRestartCN(ctx context.Context, uuid string) (bool, error) {
	resp, err := l.sendToTN(ctx, &lock.Request{
		CanRestartService: lock.CanRestartServiceRequest{ServiceID: uuid},
		Method:            lock.Method_CanRestartService},
	)
	if err != nil {
		return false, gerrors.Wrap(err, "error check can restart service")
	}
	return resp.CanRestartService.OK, nil
}

func (l *LockServiceClient) sendToTN(ctx context.Context, request *lock.Request) (*lock.Response, error) {
	_, ok := ctx.Deadline()
	if !ok {
		c, cancel := context.WithTimeout(ctx, DefaultRPCTimeout)
		defer cancel()
		ctx = c
	}
	f, err := l.client.Send(ctx, l.tnAddr, request)
	if err != nil {
		return nil, gerrors.Wrap(err, "error send lock rpc to TN")
	}
	defer f.Close()
	v, err := f.Get()
	if err != nil {
		return nil, gerrors.Wrap(err, "error get TN lock rpc response")
	}
	resp := v.(*lock.Response)
	if err := resp.UnwrapError(); err != nil {
		return nil, gerrors.Wrap(err, "TN lock rpc respond err")
	}
	return resp, nil
}

func (l *LockServiceClient) Close() error {
	return l.client.Close()
}
