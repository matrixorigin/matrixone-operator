// Copyright 2025-2026 Matrix Origin
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
	"github.com/matrixorigin/matrixone/pkg/common/morpc"
	"github.com/matrixorigin/matrixone/pkg/pb/lock"
	"go.uber.org/zap"
)

type LockServiceClient struct {
	client morpc.RPCClient
	tnAddr string
}

// DrainProof is bound to one lock-service incarnation and one allocator epoch.
// It is not interchangeable with the legacy UUID restart response.
type DrainProof struct {
	ServiceID        string
	AttemptID        string
	AllocatorID      string
	AllocatorVersion uint64
}

func (l *LockServiceClient) BeginDrain(ctx context.Context, serviceID, attemptID string) (DrainProof, error) {
	resp, err := l.sendToTN(ctx, &lock.Request{
		Method:     lock.Method_BeginDrain,
		BeginDrain: lock.BeginDrainRequest{ServiceID: serviceID, AttemptID: attemptID},
	})
	if err != nil {
		return DrainProof{}, errors.WrapPrefix(err, "begin instance-bound lock drain", 0)
	}
	p := resp.BeginDrain
	if !p.OK || p.ServiceID != serviceID || p.AttemptID != attemptID || p.AllocatorID == "" || p.AllocatorVersion == 0 {
		return DrainProof{}, errors.New("lock-service rejected or returned an incomplete drain proof")
	}
	return DrainProof{ServiceID: p.ServiceID, AttemptID: p.AttemptID,
		AllocatorID: p.AllocatorID, AllocatorVersion: p.AllocatorVersion}, nil
}

func (l *LockServiceClient) QueryDrain(ctx context.Context, proof DrainProof) (bool, error) {
	if proof.ServiceID == "" || proof.AttemptID == "" || proof.AllocatorID == "" || proof.AllocatorVersion == 0 {
		return false, errors.New("lock-service drain proof is incomplete")
	}
	resp, err := l.sendToTN(ctx, &lock.Request{
		Method: lock.Method_QueryDrain,
		QueryDrain: lock.QueryDrainRequest{
			ServiceID: proof.ServiceID, AttemptID: proof.AttemptID,
			AllocatorID: proof.AllocatorID, AllocatorVersion: proof.AllocatorVersion,
		},
	})
	if err != nil {
		return false, errors.WrapPrefix(err, "query instance-bound lock drain", 0)
	}
	p := resp.QueryDrain
	if p.ServiceID != proof.ServiceID || p.AttemptID != proof.AttemptID ||
		p.AllocatorID != proof.AllocatorID || p.AllocatorVersion != proof.AllocatorVersion {
		return false, errors.New("lock-service drain response identity changed")
	}
	return p.Safe, nil
}

func NewLockServiceClient(tnAddr string, logger *zap.Logger) (*LockServiceClient, error) {
	cfg := morpc.Config{}
	cfg.Adjust()
	logger.Info("new lockservice client", zap.String("TN addr", tnAddr))
	cfg.BackendOptions = append(cfg.BackendOptions,
		morpc.WithBackendReadTimeout(DefaultRPCTimeout))
	client, err := cfg.NewClient("", "lock-client", func() morpc.Message {
		return &lock.Response{}
	})
	if err != nil {
		return nil, errors.WrapPrefix(err, "error create lock service client", 0)
	}
	return &LockServiceClient{client: client, tnAddr: tnAddr}, nil
}

func (l *LockServiceClient) SetRestartCN(ctx context.Context, uuid string) (bool, error) {
	resp, err := l.sendToTN(ctx, &lock.Request{
		SetRestartService: lock.SetRestartServiceRequest{ServiceID: uuid},
		Method:            lock.Method_SetRestartService},
	)
	if err != nil {
		return false, errors.WrapPrefix(err, "error set restart service request", 0)
	}
	return resp.SetRestartService.OK, nil
}

func (l *LockServiceClient) CanRestartCN(ctx context.Context, uuid string) (bool, error) {
	resp, err := l.sendToTN(ctx, &lock.Request{
		CanRestartService: lock.CanRestartServiceRequest{ServiceID: uuid},
		Method:            lock.Method_CanRestartService},
	)
	if err != nil {
		return false, errors.WrapPrefix(err, "error check can restart service", 0)
	}
	return resp.CanRestartService.OK, nil
}

func (l *LockServiceClient) RemainTxnCount(ctx context.Context, uuid string) (int, error) {
	resp, err := l.sendToTN(ctx, &lock.Request{
		RemainTxnInService: lock.RemainTxnInServiceRequest{ServiceID: uuid},
		Method:             lock.Method_RemainTxnInService},
	)
	if err != nil {
		return 0, err
	}
	return int(resp.RemainTxnInService.RemainTxn), nil
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
		return nil, errors.WrapPrefix(err, "error send lock rpc to TN", 0)
	}
	defer f.Close()
	v, err := f.Get()
	if err != nil {
		return nil, errors.WrapPrefix(err, "error get TN lock rpc response", 0)
	}
	resp := v.(*lock.Response)
	if err := resp.UnwrapError(); err != nil {
		return nil, errors.WrapPrefix(err, "TN lock rpc respond err", 0)
	}
	return resp, nil
}

func (l *LockServiceClient) Close() error {
	return l.client.Close()
}
