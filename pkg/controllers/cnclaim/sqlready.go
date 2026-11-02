// Copyright 2026 Matrix Origin
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

package cnclaim

import (
	"context"
	stderrors "errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"time"

	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	"github.com/matrixorigin/matrixone-operator/pkg/controllers/cnset"
	"github.com/matrixorigin/matrixone-operator/pkg/controllers/common"
	corev1 "k8s.io/api/core/v1"
)

const (
	sqlProbeTimeout  = 3 * time.Second
	sqlProbeInterval = 3 * time.Second

	mysqlProtocolV10 = 0x0a
)

var errSQLNotReady = stderrors.New("CN SQL service is not ready")

// sqlReadyProbe reports nil only if the server at addr sends a MySQL v10
// initial handshake packet. MO listens on the SQL port before bootstrap
// completes but only starts accepting (and thus sending the greeting) after,
// so a successful TCP connect alone does not imply readiness.
type sqlReadyProbe func(ctx context.Context, addr string) error

func probeMySQLHandshake(ctx context.Context, addr string) error {
	ctx, cancel := context.WithTimeout(ctx, sqlProbeTimeout)
	defer cancel()
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", addr)
	if err != nil {
		return err
	}
	defer conn.Close()
	if dl, ok := ctx.Deadline(); ok {
		if err := conn.SetReadDeadline(dl); err != nil {
			return err
		}
	}
	// 3 bytes payload length + 1 byte sequence id, followed by the protocol version
	var header [5]byte
	if _, err := io.ReadFull(conn, header[:]); err != nil {
		return err
	}
	if header[3] != 0 || header[4] != mysqlProtocolV10 {
		return fmt.Errorf("unexpected MySQL handshake packet: seq=%d first-byte=%#x", header[3], header[4])
	}
	return nil
}

// needSQLProbe reports whether the current CN process may not have been verified
// yet: the pod was never bound by this claim, or its main container started at or
// after the last bind (e.g. in-place update or restart). Times are persisted with
// second precision, so a start within the bound second is treated as a restart.
func needSQLProbe(c *v1alpha1.CNClaim, pod *corev1.Pod) bool {
	bound := c.Status.Store.BoundTime
	if c.Status.Store.PodName != pod.Name || bound == nil {
		return true
	}
	started := common.GetCNStartedTime(pod)
	return started == nil || !started.Before(bound.Time)
}

func (r *Actor) checkSQLReady(ctx context.Context, pod *corev1.Pod) error {
	if pod.Status.PodIP == "" {
		return fmt.Errorf("%w: pod IP is not assigned", errSQLNotReady)
	}
	probe := r.sqlProbe
	if probe == nil {
		probe = probeMySQLHandshake
	}
	addr := net.JoinHostPort(pod.Status.PodIP, strconv.Itoa(cnset.CNSQLPort))
	if err := probe(ctx, addr); err != nil {
		return fmt.Errorf("%w: %s: %v", errSQLNotReady, addr, err)
	}
	return nil
}
