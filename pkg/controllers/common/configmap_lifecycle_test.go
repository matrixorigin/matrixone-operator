// Copyright 2026 Matrix Origin
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package common

import (
	"context"
	"testing"
	"time"

	recon "github.com/matrixorigin/controller-runtime/pkg/reconciler"
	mo "github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// Shared helper: no provider-specific behavior or cloud endpoint is used.
func TestConfigMapUpdatePreservesLifecycleMetadata(t *testing.T) {
	inUse := "old config still mounted"
	digest := DataDigest([]byte(inUse))
	oldKey := ConfigFile + "-" + digest
	existing := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
		Name: "config", Namespace: "synthetic", UID: "cm-uid", CreationTimestamp: metav1.NewTime(time.Unix(100, 0)),
		Labels: map[string]string{"operator-owned": "current", "external-label": "keep"}, Annotations: map[string]string{"external-annotation": "keep"},
		Finalizers:      []string{"external.example/protect"},
		OwnerReferences: []metav1.OwnerReference{{APIVersion: mo.GroupVersion.String(), Kind: "ProxySet", Name: "proxy", UID: "proxy-uid", Controller: pointer.Bool(true), BlockOwnerDeletion: pointer.Bool(true)}, {APIVersion: "v1", Kind: "ConfigMap", Name: "observer", UID: "observer-uid"}},
	}, Data: map[string]string{oldKey: inUse, "obsolete": "discard"}}
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "running", Namespace: "synthetic", Annotations: map[string]string{ConfigSuffixAnno: digest}}}
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(existing, pod).Build()
	ctx := &recon.Context[*mo.ProxySet]{Context: context.Background(), Client: cli}
	require.NoError(t, ctx.Get(client.ObjectKeyFromObject(existing), existing))
	before := existing.DeepCopy()
	desired := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "config", Namespace: "synthetic", Labels: map[string]string{"operator-owned": "fresh-template"}}, Data: map[string]string{ConfigFile: "new config", Entrypoint: "synthetic entrypoint"}}
	beforeDesired := desired.DeepCopy()
	name, err := ensureConfigMap(ctx, desired)
	require.NoError(t, err)
	require.Equal(t, "config", name)
	got := &corev1.ConfigMap{}
	require.NoError(t, ctx.Get(client.ObjectKeyFromObject(existing), got))
	require.NotEqual(t, before.ResourceVersion, got.ResourceVersion)
	gotMeta := got.ObjectMeta.DeepCopy()
	gotMeta.ResourceVersion = before.ResourceVersion
	require.Equal(t, &before.ObjectMeta, gotMeta, "data updates must preserve live lifecycle metadata")
	require.Equal(t, map[string]string{ConfigFile: "new config", Entrypoint: "synthetic entrypoint", oldKey: inUse}, got.Data)
	require.Equal(t, beforeDesired, desired, "caller template remains unchanged")

	// The old digest is removed only after no fake Pod references it.
	require.NoError(t, ctx.Delete(pod))
	_, err = ensureConfigMap(ctx, desired)
	require.NoError(t, err)
	require.NoError(t, ctx.Get(client.ObjectKeyFromObject(existing), got))
	require.Equal(t, desired.Data, got.Data)
	require.Equal(t, before.OwnerReferences, got.OwnerReferences)
	require.Equal(t, before.Finalizers, got.Finalizers)
}

type configMapConcurrentUpdateClient struct {
	client.Client
	t      *testing.T
	inject bool
}

func (c *configMapConcurrentUpdateClient) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	if c.inject {
		c.inject = false
		current := &corev1.ConfigMap{}
		require.NoError(c.t, c.Client.Get(ctx, client.ObjectKeyFromObject(obj), current))
		if current.Annotations == nil {
			current.Annotations = map[string]string{}
		}
		current.Annotations["concurrent-writer"] = "keep"
		require.NoError(c.t, c.Client.Update(ctx, current))
	}
	return c.Client.Update(ctx, obj, opts...)
}

func TestConfigMapConcurrentUpdateConflictAndFreshReadRecovery(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	existing := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "config", Namespace: "synthetic", Finalizers: []string{"external.example/protect"}}, Data: map[string]string{"config": "old"}}
	base := fake.NewClientBuilder().WithScheme(scheme).WithObjects(existing).Build()
	cli := &configMapConcurrentUpdateClient{Client: base, t: t, inject: true}
	ctx := &recon.Context[*mo.ProxySet]{Context: context.Background(), Client: cli}
	desired := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "config", Namespace: "synthetic"}, Data: map[string]string{"config": "new"}}
	before := desired.DeepCopy()
	_, err := ensureConfigMap(ctx, desired)
	require.True(t, apierrors.IsConflict(err), "same-read resourceVersion must expose the real fake-client conflict: %v", err)
	got := &corev1.ConfigMap{}
	require.NoError(t, ctx.Get(client.ObjectKeyFromObject(existing), got))
	require.Equal(t, existing.Data, got.Data, "failed write cannot publish desired data")
	require.Equal(t, "keep", got.Annotations["concurrent-writer"])
	require.Equal(t, existing.Finalizers, got.Finalizers)
	require.Equal(t, before, desired)
	_, err = ensureConfigMap(ctx, desired)
	require.NoError(t, err, "helper re-reads latest metadata on next attempt")
	require.NoError(t, ctx.Get(client.ObjectKeyFromObject(existing), got))
	require.Equal(t, desired.Data, got.Data)
	require.Equal(t, "keep", got.Annotations["concurrent-writer"])
	require.Equal(t, existing.Finalizers, got.Finalizers)
	require.Equal(t, before, desired)
}
