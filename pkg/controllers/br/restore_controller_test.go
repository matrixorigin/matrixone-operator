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

package br

import (
	"context"
	"strings"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/matrixorigin/controller-runtime/pkg/fake"
	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	"github.com/matrixorigin/matrixone-operator/pkg/controllers/common"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestRestoreJobIndependentCredentials(t *testing.T) {
	for _, tt := range []struct{ name, source, target string }{
		{"distinct static secrets", "source-secret", "target-secret"},
		{"IAM source static target", "", "target-secret"},
		{"static source IAM target", "source-secret", ""},
		{"IAM source IAM target", "", ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			for _, add := range []func(*runtime.Scheme) error{v1alpha1.AddToScheme, batchv1.AddToScheme, corev1.AddToScheme} {
				if err := add(scheme); err != nil {
					t.Fatal(err)
				}
			}
			storage := func(secret string) v1alpha1.SharedStorageProvider {
				s3 := &v1alpha1.S3Provider{Path: "test-bucket/prefix"}
				if secret != "" {
					s3.SecretRef = &corev1.LocalObjectReference{Name: secret}
				}
				return v1alpha1.SharedStorageProvider{S3: s3}
			}
			backup := &v1alpha1.Backup{ObjectMeta: metav1.ObjectMeta{Name: "backup"}, Meta: v1alpha1.BackupMeta{ID: "backup-id", Location: storage(tt.source)}}
			restore := &v1alpha1.RestoreJob{ObjectMeta: metav1.ObjectMeta{Name: "restore", Namespace: "test"}, Spec: v1alpha1.RestoreJobSpec{BackupName: backup.Name, Target: storage(tt.target)}}
			cli := fake.KubeClientBuilder().WithScheme(scheme).WithStatusSubresource(restore).WithObjects(backup, restore).Build()
			ctx := fake.NewContext(restore, cli, fake.NewMockEventEmitter(gomock.NewController(t)))
			if err := NewRestoreActor("test-image").syncJob(ctx); err != nil {
				t.Fatal(err)
			}
			job := &batchv1.Job{}
			if err := cli.Get(context.Background(), client.ObjectKeyFromObject(restore), job); err != nil {
				t.Fatal(err)
			}
			container := job.Spec.Template.Spec.Containers[0]
			for _, want := range []struct{ env, secret, key string }{
				{common.AWSAccessKeyID, tt.source, common.AWSAccessKeyID},
				{common.AWSSecretAccessKey, tt.source, common.AWSSecretAccessKey},
				{RestoreAccessEnvKey, tt.target, common.AWSAccessKeyID},
				{RestoreSecretEnvKey, tt.target, common.AWSSecretAccessKey},
			} {
				var found *corev1.EnvVar
				for i := range container.Env {
					if container.Env[i].Name == want.env {
						found = &container.Env[i]
					}
				}
				if want.secret == "" {
					if found != nil {
						t.Errorf("unexpected static credential %s", want.env)
					}
					continue
				}
				if found == nil || found.ValueFrom == nil || found.ValueFrom.SecretKeyRef == nil {
					t.Errorf("missing secret reference %s", want.env)
					continue
				}
				if got := found.ValueFrom.SecretKeyRef; got.Name != want.secret || got.Key != want.key {
					t.Errorf("%s references %s/%s, want %s/%s", want.env, got.Name, got.Key, want.secret, want.key)
				}
			}
			command := strings.Join(container.Command, " ")
			if strings.Contains(command, "--backup_access_key_id=") != (tt.source != "") {
				t.Errorf("wrong source credential flags: %s", command)
			}
			if strings.Contains(command, "--restore_access_key_id=") != (tt.target != "") {
				t.Errorf("wrong target credential flags: %s", command)
			}
		})
	}
}
