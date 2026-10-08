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

package cnstore

import (
	"context"
	"fmt"
	"os"
	"testing"

	reconfake "github.com/matrixorigin/controller-runtime/pkg/fake"
	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	"github.com/matrixorigin/matrixone-operator/pkg/controllers/common"
	kruise "github.com/openkruise/kruise-api/apps/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

// Only CloneSet discovery is a fixture. All Pod reads, status writes, updates
// and optimistic patches use the real API server; no fake invents RV behavior.
type upgradeAPIReader struct {
	client.Reader
	cloneSets client.Reader
}

func (r *upgradeAPIReader) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	if _, ok := obj.(*kruise.CloneSet); ok {
		return r.cloneSets.Get(ctx, key, obj, opts...)
	}
	return r.Reader.Get(ctx, key, obj, opts...)
}

func TestUpgradeRecoveryRealAPIConflicts(t *testing.T) {
	if os.Getenv("KUBEBUILDER_ASSETS") == "" {
		t.Skip("NOT_RUN: KUBEBUILDER_ASSETS is required")
	}
	environment := &envtest.Environment{}
	cfg, err := environment.Start()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := environment.Stop(); err != nil {
			t.Error(err)
		}
	})
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	cli, err := client.New(cfg, client.Options{Scheme: scheme})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := cli.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "ns"}}); err != nil {
		t.Fatal(err)
	}
	for _, protected := range []bool{false, true} {
		for _, mutation := range []string{"metadata", "cancellation", "container-status"} {
			t.Run(fmt.Sprintf("protected-%v-%s", protected, mutation), func(t *testing.T) {
				f := completedUpgradeFixture(t)
				// Run the real Observe health refresh and restore protection first.
				p := f.round(t)
				a, err := readDrainAttempt(p)
				if err != nil || a == nil {
					t.Fatalf("fixture: %v", err)
				}
				p.Name = fmt.Sprintf("upgrade-%v-%s", protected, mutation)
				p.UID = ""
				p.ResourceVersion = ""
				p.Spec.Containers = []corev1.Container{{Name: v1alpha1.ContainerMain, Image: "example.invalid/cn:test"}}
				if !protected {
					p.Finalizers = nil
				}
				status := p.Status.DeepCopy()
				if err := cli.Create(ctx, p); err != nil {
					t.Fatal(err)
				}
				p.Status = *status
				if err := cli.Status().Update(ctx, p); err != nil {
					t.Fatal(err)
				}
				a.PodUID = string(p.UID)
				a.CNUUID = v1alpha1.GetCNPodUUID(p)
				a.AttemptID = a.identityHash()
				p.Annotations[drainAttemptAnno], err = marshalDrainAttempt(a)
				if err != nil {
					t.Fatal(err)
				}
				if err := cli.Update(ctx, p); err != nil {
					t.Fatal(err)
				}
				reader := &afterReadClient{Client: cli, afterRead: func() {
					concurrent := &corev1.Pod{}
					if err := cli.Get(ctx, client.ObjectKeyFromObject(p), concurrent); err != nil {
						t.Fatal(err)
					}
					if mutation == "container-status" {
						concurrent.Status.ContainerStatuses[0].ContainerID = "containerd://unexpected"
						concurrent.Status.ContainerStatuses[0].RestartCount++
						if err := cli.Status().Update(ctx, concurrent); err != nil {
							t.Fatal(err)
						}
						return
					}
					if mutation == "cancellation" {
						recovery := *a
						recovery.Phase = drainPhaseRecovery
						concurrent.Annotations[drainAttemptAnno], err = marshalDrainAttempt(&recovery)
						if err != nil {
							t.Fatal(err)
						}
					} else {
						concurrent.Annotations["test/concurrent"] = "changed"
					}
					if err := cli.Update(ctx, concurrent); err != nil {
						t.Fatal(err)
					}
				}}
				wc := &withCNSet{Controller: &Controller{apiReader: &upgradeAPIReader{Reader: reader, cloneSets: f.cli}}}
				err = wc.OnNormal(reconfake.NewContext(p.DeepCopy(), cli, nil))
				if !apierrors.IsConflict(err) {
					t.Fatalf("want real API conflict, got %v", err)
				}
				got := &corev1.Pod{}
				if err := cli.Get(ctx, client.ObjectKeyFromObject(p), got); err != nil {
					t.Fatal(err)
				}
				stored, err := readDrainAttempt(got)
				want := drainPhaseCompleted
				if mutation == "cancellation" {
					want = drainPhaseRecovery
				}
				if err != nil || stored == nil || stored.Phase != want {
					t.Fatalf("conflict lost proof/cancellation: %#v %v", stored, err)
				}
				if controllerutil.ContainsFinalizer(got, common.CNDrainingFinalizer) != protected {
					t.Fatal("conflicted write changed protection")
				}
				if cond := common.GetReadinessCondition(got, common.CNStoreReadiness); cond == nil || cond.Status != corev1.ConditionFalse {
					t.Fatal("conflict admitted business")
				}
			})
		}
	}
}
