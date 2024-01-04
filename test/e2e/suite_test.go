// Copyright 2024 Matrix Origin
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
package e2e

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/matrixorigin/matrixone-operator/test/e2e/e2eminio"
	e2eutil "github.com/matrixorigin/matrixone-operator/test/e2e/util"

	"math/rand"
	"os"
	"testing"
	"time"

	"k8s.io/client-go/rest"

	"github.com/matrixorigin/controller-runtime/pkg/util"
	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	kruisev1 "github.com/openkruise/kruise-api/apps/v1beta1"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var errWait = fmt.Errorf("wait for condition met")

// node 1
var namespacePrefix string

// all nodes
var restConfig *rest.Config
var kubeconfig string
var moVersion string
var moImageRepo string
var kubeCli client.Client
var ctx context.Context
var logger *zap.SugaredLogger
var env Env
var minioForwardHandler *e2eutil.PortForwardHandler

type Env struct {
	Namespace string
}

func TestMain(m *testing.M) {
	flags := flag.CommandLine
	flags.StringVar(&kubeconfig, "kube-config", os.Getenv("KUBECONFIG"), "the kubeconfig path to access infra apiserver")
	flags.StringVar(&namespacePrefix, "namespace", "e2e", "the namespace prefix to run e2e test")
	flags.StringVar(&moVersion, "mo-version", "latest", "the version of mo to run e2e test")
	flags.StringVar(&moImageRepo, "mo-image-repo", "matrixorigin/matrixone", "the image repository of mo to run e2e test")
	flag.Parse()

	RegisterFailHandler(Fail)
	rand.Seed(time.Now().UnixNano())
	os.Exit(m.Run())
}

func TestE2E(t *testing.T) {
	RunSpecs(t, "E2E Suite")
}

var _ = SynchronizedBeforeSuite(func() []byte {
	// randomize a namespace name to avoid collision
	env.Namespace = fmt.Sprintf("%s-%d", namespacePrefix, time.Now().UnixNano())

	restConfig, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	Expect(err).To(Succeed())
	Expect(v1alpha1.AddToScheme(scheme.Scheme)).To(Succeed())
	Expect(kruisev1.AddToScheme(scheme.Scheme)).To(Succeed())
	kubeCli, err = client.New(restConfig, client.Options{
		Scheme: scheme.Scheme,
	})
	ctx = context.Background()
	Expect(err).To(Succeed())

	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: env.Namespace,
		},
	}
	Expect(util.Ignore(apierrors.IsAlreadyExists, kubeCli.Create(ctx, ns))).To(Succeed())

	buf, err := json.Marshal(env)
	Expect(err).Should(BeNil())

	By("port forward minio")
	minioForwardHandler, err = e2eminio.PortForward(kubeCli, restConfig)
	Expect(err).Should(BeNil())
	Expect(minioForwardHandler.Ready(portForwardTimeout)).To(Succeed(), "port-forward should complete within timeout")
	return buf

}, func(fromNode1 []byte) {
	// run on all nodes
	var local Env
	err := json.Unmarshal(fromNode1, &local)
	Expect(err).Should(BeNil())
	env = local

	baseLog, err := zap.NewDevelopment()
	Expect(err).To(Succeed())
	defer func(baseLog *zap.Logger) {
		_ = baseLog.Sync()
	}(baseLog)
	logger = baseLog.Sugar()

	restConfig, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
	Expect(err).To(Succeed())
	Expect(v1alpha1.AddToScheme(scheme.Scheme)).To(Succeed())
	Expect(kruisev1.AddToScheme(scheme.Scheme)).To(Succeed())
	kubeCli, err = client.New(restConfig, client.Options{
		Scheme: scheme.Scheme,
	})
	Expect(err).To(Succeed())
	ctx = context.Background()
})

var _ = SynchronizedAfterSuite(func() {
	// run on all nodes
}, func() {
	// run synchronized
	ReclaimReleasedBucket()
	e2eminio.StopForward(minioForwardHandler)
})

func e2eResourceLabels() map[string]string {
	return map[string]string{
		"matrixorigin.io/usage": "e2e",
		"managed-by":            "e2e-suite",
	}
}
