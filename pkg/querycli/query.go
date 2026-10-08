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

package querycli

import (
	"context"
	"time"

	"github.com/go-errors/errors"
	"github.com/matrixorigin/matrixone-operator/pkg/metric"
	"github.com/matrixorigin/matrixone/pkg/common/morpc"
	pb "github.com/matrixorigin/matrixone/pkg/pb/query"
	"github.com/matrixorigin/matrixone/pkg/txn/rpc"
)

var timeout = 10 * time.Second

type Client struct {
	c morpc.RPCClient
}

func New() (*Client, error) {
	pool := morpc.NewMessagePool(
		func() *pb.Request { return &pb.Request{} },
		func() *pb.Response { return &pb.Response{} })

	queryCli, err := rpc.Config{}.NewClient("", "query-service",
		func() morpc.Message { return pool.AcquireResponse() })
	if err != nil {
		return nil, err
	}
	return &Client{c: queryCli}, nil
}

func (c *Client) ShowProcessList(ctx context.Context, address string) (*pb.ShowProcessListResponse, error) {
	resp, err := c.SendReq(ctx, address, &pb.Request{
		CmdMethod: pb.CmdMethod_ShowProcessList,
		ShowProcessListRequest: &pb.ShowProcessListRequest{
			SysTenant: true,
		},
	})
	if err != nil {
		return nil, errors.WrapPrefix(err, "error send request", 0)
	}
	return resp.ShowProcessListResponse, nil
}

func (c *Client) GetPipelineInfo(ctx context.Context, address string) (*pb.GetPipelineInfoResponse, error) {
	resp, err := c.SendReq(ctx, address, &pb.Request{
		CmdMethod:              pb.CmdMethod_GetPipelineInfo,
		GetPipelineInfoRequest: &pb.GetPipelineInfoRequest{},
	})
	if err != nil {
		return nil, errors.WrapPrefix(err, "error send request", 0)
	}
	return resp.GetPipelineInfoResponse, nil
}

func (c *Client) GetReplicaCount(ctx context.Context, address string) (pb.GetReplicaCountResponse, error) {
	resp, err := c.SendReq(ctx, address, &pb.Request{
		CmdMethod:       pb.CmdMethod_GetReplicaCount,
		GetReplicaCount: pb.GetReplicaCountRequest{},
	})
	if err != nil {
		return pb.GetReplicaCountResponse{}, errors.WrapPrefix(err, "error send request", 0)
	}
	return resp.GetReplicaCount, nil
}

// GetLockServiceIdentity reads the exact lock-service incarnation from the CN
// query endpoint. A UUID-only response from an older CN is not sufficient.
func (c *Client) GetLockServiceIdentity(ctx context.Context, address string) (string, string, error) {
	resp, err := c.SendReq(ctx, address, &pb.Request{
		CmdMethod:          pb.CmdMethod_GetLockInfo,
		GetLockInfoRequest: &pb.GetLockInfoRequest{IdentityOnly: true},
	})
	if err != nil {
		return "", "", errors.WrapPrefix(err, "query CN lock-service identity", 0)
	}
	if resp.GetLockInfoResponse == nil || resp.GetLockInfoResponse.CnId == "" || resp.GetLockInfoResponse.LockServiceID == "" {
		return "", "", errors.New("CN did not provide a complete lock-service identity")
	}
	return resp.GetLockInfoResponse.CnId, resp.GetLockInfoResponse.LockServiceID, nil
}

func (c *Client) SendReq(ctx context.Context, address string, req *pb.Request) (*pb.Response, error) {
	var err error
	start := time.Now()
	defer func() {
		resp := "ok"
		if err != nil {
			resp = "error"
		}
		metric.CnRPCDuration.WithLabelValues(req.GetCmdMethod().String(), address, resp).Observe(time.Since(start).Seconds())
	}()

	queryCtx := ctx
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		queryCtx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	f, err := c.c.Send(queryCtx, address, req)
	if err != nil {
		return nil, errors.WrapPrefix(err, "error send request", 0)
	}
	msg, err := f.Get()
	if err != nil {
		return nil, errors.WrapPrefix(err, "error get msg", 0)
	}
	resp, ok := msg.(*pb.Response)
	if !ok {
		return nil, errors.Errorf("message is not a valid response: %s", msg.DebugString())
	}
	return resp, nil
}
