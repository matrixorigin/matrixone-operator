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
	"sync"
	"time"

	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	"github.com/matrixorigin/matrixone-operator/pkg/controllers/cnset"
	"github.com/matrixorigin/matrixone-operator/pkg/controllers/common"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
)

const (
	sqlProbeTimeout  = 3 * time.Second
	sqlProbeInterval = 3 * time.Second
	// bounds the probes that may be blocked on bootstrapping CNs at the same time
	sqlProbeConcurrency = 8
	// must exceed sqlProbeInterval so a result is still there on the next requeue
	sqlProbeResultTTL = 10 * time.Second

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

// sqlProbeKey identifies one CN process: a restart changes the start time and
// thus never reuses a result obtained for the previous process.
type sqlProbeKey struct {
	uid     types.UID
	started int64
	addr    string
}

type sqlProbeResult struct {
	err  error
	done time.Time
}

// asyncSQLProber runs probes in the background so that a CN which accepts TCP
// but never greets does not hold the single CNClaim reconcile worker.
type asyncSQLProber struct {
	probe sqlReadyProbe
	ttl   time.Duration
	now   func() time.Time
	sem   chan struct{}

	mu       sync.Mutex
	inflight map[sqlProbeKey]struct{}
	results  map[sqlProbeKey]sqlProbeResult
}

func newAsyncSQLProber(probe sqlReadyProbe, concurrency int, ttl time.Duration) *asyncSQLProber {
	return &asyncSQLProber{
		probe:    probe,
		ttl:      ttl,
		now:      time.Now,
		sem:      make(chan struct{}, concurrency),
		inflight: map[sqlProbeKey]struct{}{},
		results:  map[sqlProbeKey]sqlProbeResult{},
	}
}

// check never blocks. It returns nil only by consuming a fresh successful
// result for key; otherwise it makes sure a probe is (or will be) running and
// returns an error wrapping errSQLNotReady.
func (p *asyncSQLProber) check(key sqlProbeKey) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	now := p.now()
	for k, r := range p.results {
		if now.Sub(r.done) > p.ttl {
			delete(p.results, k)
		}
	}
	reason := "probe started"
	if r, ok := p.results[key]; ok {
		delete(p.results, key)
		if r.err == nil {
			return nil
		}
		// start the next attempt right away instead of waiting for another requeue
		reason = r.err.Error()
	}
	if _, ok := p.inflight[key]; ok {
		return fmt.Errorf("%w: %s: probe in progress", errSQLNotReady, key.addr)
	}
	select {
	case p.sem <- struct{}{}:
	default:
		return fmt.Errorf("%w: %s: too many probes in flight", errSQLNotReady, key.addr)
	}
	p.inflight[key] = struct{}{}
	go func() {
		defer func() { <-p.sem }()
		// not tied to the reconcile context, which ends before the probe; the probe has its own timeout
		err := p.probe(context.Background(), key.addr)
		p.mu.Lock()
		defer p.mu.Unlock()
		delete(p.inflight, key)
		p.results[key] = sqlProbeResult{err: err, done: p.now()}
	}()
	return fmt.Errorf("%w: %s: %s", errSQLNotReady, key.addr, reason)
}

func (r *Actor) sqlProber() *asyncSQLProber {
	r.proberOnce.Do(func() {
		probe := r.sqlProbe
		if probe == nil {
			probe = probeMySQLHandshake
		}
		r.prober = newAsyncSQLProber(probe, sqlProbeConcurrency, sqlProbeResultTTL)
	})
	return r.prober
}

func (r *Actor) checkSQLReady(pod *corev1.Pod) error {
	if pod.Status.PodIP == "" {
		return fmt.Errorf("%w: pod IP is not assigned", errSQLNotReady)
	}
	key := sqlProbeKey{
		uid:  pod.UID,
		addr: net.JoinHostPort(pod.Status.PodIP, strconv.Itoa(cnset.CNSQLPort)),
	}
	if started := common.GetCNStartedTime(pod); started != nil {
		key.started = started.UnixNano()
	}
	return r.sqlProber().check(key)
}
