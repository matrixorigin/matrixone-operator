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
	"net"
	"testing"
	"time"

	reconfake "github.com/matrixorigin/controller-runtime/pkg/fake"
	recon "github.com/matrixorigin/controller-runtime/pkg/reconciler"
	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	"github.com/openkruise/kruise-api/apps/pub"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "github.com/onsi/gomega"
)

func listenLocal(t *testing.T) net.Listener {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = l.Close() })
	return l
}

func Test_probeMySQLHandshake(t *testing.T) {
	greeting := []byte{0x4a, 0x00, 0x00, 0x00, mysqlProtocolV10, '8', '.', '0'}
	serve := func(l net.Listener, payload []byte) {
		go func() {
			for {
				c, err := l.Accept()
				if err != nil {
					return
				}
				_, _ = c.Write(payload)
				_ = c.Close()
			}
		}()
	}

	t.Run("accepts MySQL v10 greeting", func(t *testing.T) {
		l := listenLocal(t)
		serve(l, greeting)
		NewGomegaWithT(t).Expect(probeMySQLHandshake(context.Background(), l.Addr().String())).To(Succeed())
	})

	t.Run("rejects non-greeting packet", func(t *testing.T) {
		l := listenLocal(t)
		serve(l, []byte{0x10, 0x00, 0x00, 0x00, 0xff, 0x00})
		NewGomegaWithT(t).Expect(probeMySQLHandshake(context.Background(), l.Addr().String())).NotTo(Succeed())
	})

	t.Run("listening but not accepting is not ready", func(t *testing.T) {
		// the kernel completes the TCP handshake from the backlog, but no greeting is ever sent
		l := listenLocal(t)
		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		defer cancel()
		NewGomegaWithT(t).Expect(probeMySQLHandshake(ctx, l.Addr().String())).NotTo(Succeed())
	})

	t.Run("connection refused", func(t *testing.T) {
		l := listenLocal(t)
		addr := l.Addr().String()
		_ = l.Close()
		NewGomegaWithT(t).Expect(probeMySQLHandshake(context.Background(), addr)).NotTo(Succeed())
	})
}

func Test_checkSQLReady(t *testing.T) {
	g := NewGomegaWithT(t)
	var gotAddr string
	r := &Actor{sqlProbe: func(_ context.Context, addr string) error {
		gotAddr = addr
		return nil
	}}
	g.Expect(r.checkSQLReady(context.Background(), &corev1.Pod{})).To(MatchError(errSQLNotReady))
	g.Expect(gotAddr).To(BeEmpty())

	pod := &corev1.Pod{Status: corev1.PodStatus{PodIP: "10.0.0.7"}}
	g.Expect(r.checkSQLReady(context.Background(), pod)).To(Succeed())
	g.Expect(gotAddr).To(Equal("10.0.0.7:6001"))

	r.sqlProbe = func(context.Context, string) error { return stderrors.New("boom") }
	g.Expect(r.checkSQLReady(context.Background(), pod)).To(MatchError(errSQLNotReady))
}

func Test_Sync_waitsForSQLReadyBeforePublishingWorking(t *testing.T) {
	g := NewGomegaWithT(t)
	claim := &v1alpha1.CNClaim{
		ObjectMeta: metav1.ObjectMeta{Name: "claim", Namespace: "ns"},
		Spec:       v1alpha1.CNClaimSpec{ClaimPodRef: v1alpha1.ClaimPodRef{PodName: "cn-0"}},
		Status:     v1alpha1.CNClaimStatus{Phase: v1alpha1.CNClaimPhasePending},
	}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cn-0",
			Namespace: "ns",
			Labels:    map[string]string{pub.LifecycleStateKey: string(pub.LifecycleStateNormal)},
		},
		Status: corev1.PodStatus{PodIP: "10.0.0.7"},
	}
	cli := newFakeClient(claim, pod)
	ctx := reconfake.NewContext(claim, cli, nil)

	// a nil clientMgr would fail patchStore, so reaching it would not yield the SQL wait requeue below
	r := &Actor{sqlProbe: func(context.Context, string) error { return stderrors.New("handshake timeout") }}
	err := r.Sync(ctx)
	var rs *recon.ReSync
	g.Expect(stderrors.As(err, &rs)).To(BeTrue())
	g.Expect(rs.RequeueAfter).To(Equal(sqlProbeInterval))
	g.Expect(claim.Status.Phase).To(Equal(v1alpha1.CNClaimPhasePending))
}
