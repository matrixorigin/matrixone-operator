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

package mometric

import (
	"context"
	"database/sql"
	"fmt"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sync"
	"time"
)

const (
	queryTimeout = 10 * time.Second
)

type Client struct {
	kubeCli client.Client
	secret  types.NamespacedName
	target  string

	sync.Mutex
	conn *sql.DB
}

func NewClient(target string, kubeCli client.Client, secret types.NamespacedName) *Client {
	return &Client{
		target:  target,
		kubeCli: kubeCli,
		secret:  secret,
	}
}

func (c *Client) GetServerConnection(ctx context.Context, uid string) (int, error) {
	rows, err := c.query(ctx, `
SELECT value FROM
system_metrics.server_connections
WHERE node=?
ORDER BY collecttime DESC
LIMIT 1;
`, uid)
	if err != nil {
		return 0, err
	}
	var v int
	for rows.Next() {
		if err := rows.Scan(v); err != nil {
			return 0, err
		}
	}
	return v, nil
}

func (c *Client) query(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	conn, err := c.getConnection(ctx)
	if err != nil {
		return nil, err
	}

	if _, ok := ctx.Deadline(); !ok {
		timeout, cancel := context.WithTimeout(ctx, queryTimeout)
		defer cancel()
		ctx = timeout
	}
	return conn.QueryContext(ctx, query, args...)
}

func (c *Client) getConnection(ctx context.Context) (*sql.DB, error) {
	if c.conn != nil {
		return c.conn, nil
	}
	c.Lock()
	defer c.Unlock()
	if c.conn != nil {
		return c.conn, nil
	}
	secret := &corev1.Secret{}
	err := c.kubeCli.Get(ctx, c.secret, secret)
	if err != nil {
		return nil, err
	}
	username := string(secret.Data["username"])
	pwd := string(secret.Data["password"])
	dsn := fmt.Sprintf("%s:%s@tcp(%s)/?timeout=10s", username, pwd, c.target)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}
	c.conn = db
	return c.conn, nil
}
