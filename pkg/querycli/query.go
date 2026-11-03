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
	"github.com/matrixorigin/matrixone/pkg/util/toml"
)

var timeout = 10 * time.Second

type Client struct {
	c      morpc.RPCClient
	status morpc.RPCClient
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
	statusCli, err := (rpc.Config{MaxMessageSize: toml.ByteSize(maxPythonStatusRPCBytes)}).NewClient("", "query-service",
		func() morpc.Message { return &pythonUDFStatusResponse{} })
	if err != nil {
		_ = queryCli.Close()
		return nil, err
	}
	return &Client{c: queryCli, status: statusCli}, nil
}

// Close releases both query-service clients. They are separate because the
// normal response pool and the Python status response have different wire
// message types. Callers that own a Client must close it when their runtime
// stops; morpc futures and backend goroutines otherwise outlive the
// controller that created them.
func (c *Client) Close() error {
	if c == nil {
		return nil
	}
	var firstErr error
	if c.status != nil {
		if err := c.status.Close(); err != nil {
			firstErr = err
		}
	}
	if c.c != nil {
		if err := c.c.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
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
	defer f.Close()
	msg, err := f.Get()
	if err != nil {
		return nil, errors.WrapPrefix(err, "error get msg", 0)
	}
	if msg == nil {
		return nil, errors.New("query service returned an empty response")
	}
	resp, ok := msg.(*pb.Response)
	if !ok {
		return nil, errors.Errorf("message is not a valid response: %s", msg.DebugString())
	}
	return resp, nil
}
