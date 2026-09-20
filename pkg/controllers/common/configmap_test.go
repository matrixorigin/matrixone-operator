// Copyright 2025-2026 Matrix Origin
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
package common

import (
	"testing"

	reconfake "github.com/matrixorigin/controller-runtime/pkg/fake"
	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	. "github.com/onsi/gomega"
	"golang.org/x/exp/utf8string"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestDataDigest(t *testing.T) {
	// need fuzz?
	cmList := []*corev1.ConfigMap{
		newCM(""),
		newCM(" "),
		newCM("hello world"),
		newCM("你好世界"),
		newCM("こんにちは世界"),
	}
	g := NewGomegaWithT(t)
	for _, cm := range cmList {
		digest := DataDigest([]byte(cm.Data["config"]))
		g.Expect(utf8string.NewString(digest).IsASCII()).To(BeTrue())
	}
}

func TestLegacyConfigMapNameIsContentAddressed(t *testing.T) {
	cm := newCM("legacy")
	name, err := LegacyConfigMapName(cm)
	if err != nil {
		t.Fatal(err)
	}
	if name == cm.Name || name != "foo-"+DataDigest([]byte(`{"config":"legacy"}`)) {
		t.Fatalf("legacy ConfigMap name = %q, want content-addressed name", name)
	}
	if cm.Name != "foo" {
		t.Fatalf("LegacyConfigMapName mutated input name to %q", cm.Name)
	}
}

func newCM(data string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "foo"},
		Data: map[string]string{
			"config": data,
		},
	}
}

func TestEnsureConfigMapPreservesOwnershipMetadata(t *testing.T) {
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(v1alpha1.AddToScheme(scheme))
	owner := &v1alpha1.CNSet{ObjectMeta: metav1.ObjectMeta{Name: "cn", Namespace: "ns", UID: "cn-uid"}}
	old := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "config",
			Namespace:       "ns",
			UID:             "config-uid",
			ResourceVersion: "7",
			OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(owner, v1alpha1.GroupVersion.WithKind("CNSet"))},
			Finalizers:      []string{"example.com/cleanup"},
		},
		Data: map[string]string{"config.toml": "old"},
	}
	cli := reconfake.KubeClientBuilder().WithScheme(scheme).WithObjects(old).Build()
	ctx := reconfake.NewContext(owner, cli, nil)
	desired := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "config", Namespace: "ns"},
		Data:       map[string]string{"config.toml": "new"},
	}
	if _, err := ensureConfigMap(ctx, desired); err != nil {
		t.Fatal(err)
	}
	got := &corev1.ConfigMap{}
	if err := cli.Get(ctx, client.ObjectKey{Namespace: "ns", Name: "config"}, got); err != nil {
		t.Fatal(err)
	}
	if !metav1.IsControlledBy(got, owner) || got.UID != old.UID || got.ResourceVersion == "" {
		t.Fatalf("ownership metadata changed: %#v", got.ObjectMeta)
	}
	if len(got.Finalizers) != 1 || got.Finalizers[0] != old.Finalizers[0] {
		t.Fatalf("finalizers changed: %#v", got.Finalizers)
	}
}
