// Copyright 2022 Matrix Origin
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package webui

import (
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	"github.com/matrixorigin/matrixone-operator/runtime/pkg/fake"
	recon "github.com/matrixorigin/matrixone-operator/runtime/pkg/reconciler"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/onsi/gomega"
)

func TestWebuiActor_Observe(t *testing.T) {
	s := newScheme()
	tpl := &v1alpha1.WebUI{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "test",
		},
		Spec: v1alpha1.WebUISpec{
			WebUIBasic: v1alpha1.WebUIBasic{
				PodSet: v1alpha1.PodSet{
					MainContainer: v1alpha1.MainContainer{
						Image: "test:latest",
					},
					Replicas: 1,
				},
			},
		},
	}

	tests := []struct {
		name   string
		webui  *v1alpha1.WebUI
		client client.Client
		expect func(g *WithT, action recon.Action[*v1alpha1.WebUI], err error)
	}{
		{
			name:  "create when resource not exist",
			webui: tpl,
			client: &fake.Client{
				Client: fake.KubeClientBuilder().WithScheme(s).Build(),
			},
			expect: func(g *WithT, action recon.Action[*v1alpha1.WebUI], err error) {
				g.Expect(err).To(BeNil())
				g.Expect(action.String()).To(ContainSubstring("Create"))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewGomegaWithT(t)
			r := &Actor{}
			mockCtrl := gomock.NewController(t)
			eventEmitter := fake.NewMockEventEmitter(mockCtrl)
			ctx := fake.NewContext(tt.webui, tt.client, eventEmitter)
			action, err := r.Observe(ctx)
			tt.expect(g, action, err)
		})
	}

}

func newScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(v1alpha1.AddToScheme(scheme))

	return scheme

}
