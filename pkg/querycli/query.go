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

package querycli

import (
	"context"
	"github.com/matrixorigin/matrixone/pkg/common/morpc"
	pb "github.com/matrixorigin/matrixone/pkg/pb/query"
	"github.com/matrixorigin/matrixone/pkg/txn/rpc"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"time"
)

var timeout = 10 * time.Second

type Client struct {
	c morpc.RPCClient
}

func New(logger *zap.Logger) (*Client, error) {
	pool := morpc.NewMessagePool(
		func() *pb.Request { return &pb.Request{} },
		func() *pb.Response { return &pb.Response{} })

	queryCli, err := rpc.Config{}.NewClient("query-service",
		logger,
		func() morpc.Message { return pool.AcquireResponse() })
	if err != nil {
		return nil, err
	}
	return &Client{c: queryCli}, nil
}

func (c *Client) ShowProcessList(ctx context.Context, address string) (*pb.ShowProcessListResponse, error) {
	queryCtx := ctx
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		queryCtx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	f, err := c.c.Send(queryCtx, address, &pb.Request{
		CmdMethod: pb.CmdMethod_ShowProcessList,
		ShowProcessListRequest: &pb.ShowProcessListRequest{
			SysTenant: true,
		},
	})
	if err != nil {
		return nil, errors.Wrap(err, "error show process list")
	}
	msg, err := f.Get()
	if err != nil {
		return nil, errors.Wrap(err, "error get msg")
	}
	resp, ok := msg.(*pb.Response)
	if !ok {
		return nil, errors.Errorf("message is not a valid showprocesslist response: %s", msg.DebugString())
	}
	return resp.ShowProcessListResponse, nil
}
