// Copyright 2021 Matrix Origin
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

package main

import (
	"flag"
	"fmt"
	"github.com/matrixorigin/matrixone-operator/pkg/controllers/cnset"
	"github.com/matrixorigin/matrixone-operator/pkg/controllers/dnset"
	"github.com/matrixorigin/matrixone-operator/pkg/controllers/logset"
	"github.com/matrixorigin/matrixone-operator/pkg/controllers/mocluster"
	hookctrl "github.com/matrixorigin/matrixone-operator/pkg/controllers/webhook"
	"github.com/matrixorigin/matrixone-operator/pkg/controllers/webui"
	kruisev1 "github.com/openkruise/kruise-api/apps/v1beta1"
	"go.uber.org/zap/zapcore"
	"os"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	//+kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(v1alpha1.AddToScheme(scheme))
	utilruntime.Must(kruisev1.AddToScheme(scheme))
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	var webhookCertDir string
	var caFile string
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.StringVar(&webhookCertDir, "webhook-certificate-directory", "/tmp/k8s-webhook-server/serving-certs", "the directory that provide certificates for the webhook server")
	flag.StringVar(&caFile, "ca-file", "caBundle", "the filename of caBundle")
	opts := &zap.Options{
		Development: true,
		TimeEncoder: zapcore.RFC3339TimeEncoder,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(opts)))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		Host:                   "0.0.0.0",
		Port:                   9443,
		MetricsBindAddress:     metricsAddr,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "0c8ab548.matrixorigin.io",
		WebhookServer: &webhook.Server{
			CertDir: webhookCertDir,
		},
	})
	exitIf(err, "failed to start manager")

	if os.Getenv("ENABLE_WEBHOOKS") != "false" {
		err := v1alpha1.RegisterWebhooks(mgr)
		exitIf(err, "unable to set up webhook")

		caBundle, err := os.ReadFile(fmt.Sprintf("%s/%s", webhookCertDir, caFile))
		exitIf(err, "unable to read caBundle of wehbook server")
		err = hookctrl.Setup(hookctrl.WebhookTypeMutating, mgr, caBundle)
		exitIf(err, "unable to setup mutating webhook controller")
		err = hookctrl.Setup(hookctrl.WebhookTypeValidating, mgr, caBundle)
		exitIf(err, "unable to setup validating webhook controller")
	}

	logSetActor := &logset.LogSetActor{}
	err = logSetActor.Reconcile(mgr)
	exitIf(err, "unable to set up log service controller")

	dnSetActor := &dnset.DNSetActor{}
	err = dnSetActor.Reconcile(mgr)
	exitIf(err, "unable to set up dn service controller")

	cnSetActor := &cnset.CNSetActor{}
	err = cnSetActor.Reconcile(mgr)
	exitIf(err, "unable to setup  cn service controller")

	webuiActor := &webui.WebUIActor{}
	err = webuiActor.Reconcile(mgr)
	exitIf(err, "unable to setup webui service controller")

	moActor := &mocluster.MatrixOneClusterActor{}
	err = moActor.Reconcile(mgr)
	exitIf(err, "unable to set up matrixone cluster controller")

	err = mgr.AddHealthzCheck("healthz", healthz.Ping)
	exitIf(err, "unable to set up health check")
	err = mgr.AddReadyzCheck("readyz", healthz.Ping)
	exitIf(err, "unable to set up ready check")

	setupLog.Info("starting manager")
	err = mgr.Start(ctrl.SetupSignalHandler())
	exitIf(err, "problem running manager")
}

func exitIf(err error, msg string) {
	if err != nil {
		setupLog.Error(err, msg)
		os.Exit(1)
	}
	return
}
