// Copyright 2022 Matrix Origin
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package reconciler

import (
	"context"
	"github.com/google/go-cmp/cmp"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	recon "sigs.k8s.io/controller-runtime/pkg/reconcile"
	"testing"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

var _ recon.Reconciler = &Reconciler[client.Object]{}

func TestReconciler(t *testing.T) {
	type args struct {
		m manager.Manager
		a Actor[*corev1.Pod]
	}

	type want struct {
		result recon.Result
		err    error
	}

	cases := map[string]struct {
		args args
		want want
	}{
		"ReachDesiredState": {
			args: args{},
			want: want{
				result: recon.Result{Requeue: false},
			},
		},
		"ErrorOnObserve": {},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			g := NewGomegaWithT(t)
			r, err := newReconciler(&corev1.Pod{}, "test", tc.args.m, tc.args.a, &options{})
			g.Expect(err).To(Succeed())
			got, err := r.Reconcile(context.Background(), recon.Request{})
			if diff := cmp.Diff(tc.want.err, err); diff != "" {
				t.Errorf("\n%s\nr.Reconcile(...): -want error, +got error:\n%s", name, diff)
			}
			if diff := cmp.Diff(tc.want.result, got); diff != "" {
				t.Errorf("\n%s\nr.Reconcile(...): -want, +got:\n%s", name, diff)
			}
		})
	}
}

func TestSetupObjectFactory(t *testing.T) {
	g := NewGomegaWithT(t)
	r := &Reconciler[*corev1.Service]{}
	s := runtime.NewScheme()
	_ = corev1.AddToScheme(s)
	g.Expect(r.setupObjectFactory(s, &corev1.Service{})).To(Succeed())
	g.Expect(r.newT()).ToNot(BeNil())
}
