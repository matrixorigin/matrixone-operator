// Copyright 2024 Matrix Origin
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
	"os"

	"github.com/go-logr/zapr"
	"github.com/matrixorigin/matrixone-operator/pkg/controllers/br"
	"github.com/matrixorigin/matrixone-operator/pkg/controllers/cnclaim"
	"github.com/matrixorigin/matrixone-operator/pkg/controllers/cnclaimset"
	"github.com/matrixorigin/matrixone-operator/pkg/controllers/cnpool"
	"github.com/matrixorigin/matrixone-operator/pkg/mocli"
	"github.com/matrixorigin/matrixone-operator/pkg/querycli"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/matrixorigin/controller-runtime/pkg/metrics"
	"github.com/matrixorigin/matrixone-operator/api/features"
	"github.com/matrixorigin/matrixone-operator/pkg/controllers/bucketclaim"
	"github.com/matrixorigin/matrixone-operator/pkg/controllers/cnstore"
	"github.com/matrixorigin/matrixone-operator/pkg/controllers/common"
	"github.com/matrixorigin/matrixone-operator/pkg/controllers/proxyset"
	"github.com/matrixorigin/matrixone/pkg/logutil"
	kruisev1alpha1 "github.com/openkruise/kruise-api/apps/v1alpha1"
	kruisepolicy "github.com/openkruise/kruise-api/policy/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	"github.com/matrixorigin/matrixone-operator/pkg/controllers/cnset"
	"github.com/matrixorigin/matrixone-operator/pkg/controllers/dnset"
	"github.com/matrixorigin/matrixone-operator/pkg/controllers/logset"
	"github.com/matrixorigin/matrixone-operator/pkg/controllers/mocluster"
	hookctrl "github.com/matrixorigin/matrixone-operator/pkg/controllers/webhook"
	"github.com/matrixorigin/matrixone-operator/pkg/controllers/webui"
	mowebhook "github.com/matrixorigin/matrixone-operator/pkg/webhook"
	kruisev1 "github.com/openkruise/kruise-api/apps/v1beta1"
	zaporigin "go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	controllermetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
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
	// +kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(v1alpha1.AddToScheme(scheme))
	utilruntime.Must(kruisev1.AddToScheme(scheme))
	utilruntime.Must(kruisev1alpha1.AddToScheme(scheme))
	utilruntime.Must(kruisepolicy.AddToScheme(scheme))
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	var webhookCertDir string
	var operatorCfgDir string
	var caFile string
	var failover bool
	var operatorCfg common.OperatorConfig

	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.StringVar(&webhookCertDir, "webhook-certificate-directory", "/tmp/k8s-webhook-server/serving-certs", "the directory that provide certificates for the webhook server")
	flag.StringVar(&operatorCfgDir, "operator-cfg-directory", "/etc/mo-operator-cfg", "the directory that mo opeartor configmap mount, should consistent with your mount config")
	flag.StringVar(&caFile, "ca-file", "caBundle", "the filename of caBundle")
	flag.BoolVar(&failover, "failover", true, "enable failover feature-gate")
	opts := &zap.Options{
		Development: false,
		TimeEncoder: zapcore.RFC3339TimeEncoder,
		// add caller to log line
		ZapOpts: []zaporigin.Option{zaporigin.AddCaller()},
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()
	zapLogger := zap.NewRaw(zap.UseFlagOptions(opts))
	ctrl.SetLogger(zapr.NewLogger(zapLogger))
	logutil.SetupMOLogger(&logutil.LogConfig{
		Level: zapcore.ErrorLevel.String(),
	})

	err := common.LoadOperatorConfig(operatorCfgDir, &operatorCfg)
	exitIf(err, "failed to load operator configmap")

	err = features.DefaultMutableFeatureGate.SetFromMap(operatorCfg.FeatureGates)
	exitIf(err, "failed to set feature gate")

	restCfg := ctrl.GetConfigOrDie()
	mgr, err := ctrl.NewManager(restCfg, ctrl.Options{
		Scheme:                 scheme,
		Host:                   "0.0.0.0",
		Port:                   9443,
		MetricsBindAddress:     metricsAddr,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "0c8ab548.matrixorigin.io",
		WebhookServer: webhook.NewServer(webhook.Options{
			CertDir: webhookCertDir,
		}),
	})
	exitIf(err, "failed to start manager")
	directClient, err := client.New(restCfg, client.Options{
		Scheme: scheme,
	})
	exitIf(err, "failed to create direct client")

	collector := metrics.NewMetricsCollector("matrixone", mgr.GetClient())
	err = collector.RegisterResource(&v1alpha1.LogSetList{})
	exitIf(err, "unable to regist metrics of logset")
	err = collector.RegisterResource(&v1alpha1.DNSetList{})
	exitIf(err, "unable to regist metrics of dnset")
	err = collector.RegisterResource(&v1alpha1.CNSetList{})
	exitIf(err, "unable to regist metrics of cnset")
	err = collector.RegisterResource(&v1alpha1.MatrixOneClusterList{})
	exitIf(err, "unable to regist metrics of matrixonecluster")
	controllermetrics.Registry.MustRegister(collector)

	if os.Getenv("ENABLE_WEBHOOKS") != "false" {
		mowebhook.ServiceDefaultArgs = operatorCfg.DefaultArgs
		err := mowebhook.RegisterWebhooks(mgr)
		exitIf(err, "unable to set up webhook")

		caBundle, err := os.ReadFile(fmt.Sprintf("%s/%s", webhookCertDir, caFile))
		exitIf(err, "unable to read caBundle of wehbook server")
		err = hookctrl.Setup(hookctrl.TypeMutating, mgr, caBundle)
		exitIf(err, "unable to setup mutating webhook controller")
		err = hookctrl.Setup(hookctrl.TypeValidating, mgr, caBundle)
		exitIf(err, "unable to setup validating webhook controller")
	}

	logSetActor := &logset.Actor{FailoverEnabled: failover}
	err = logSetActor.Reconcile(mgr)
	exitIf(err, "unable to set up log service controller")

	dnSetActor := &dnset.Actor{}
	err = dnSetActor.Reconcile(mgr)
	exitIf(err, "unable to set up dn service controller")

	cnSetActor := &cnset.Actor{}
	err = cnSetActor.Reconcile(mgr)
	exitIf(err, "unable to setup  cn service controller")

	webuiActor := &webui.Actor{}
	err = webuiActor.Reconcile(mgr)
	exitIf(err, "unable to setup webui service controller")

	moActor := &mocluster.MatrixOneClusterActor{}
	err = moActor.Reconcile(mgr)
	exitIf(err, "unable to set up matrixone cluster controller")

	if features.DefaultFeatureGate.Enabled(features.BRSupport) {
		backupActor := br.NewBackupActor(operatorCfg.BRConfig.Image)
		err = backupActor.Reconcile(mgr)
		exitIf(err, "unable to setup backup actor")

		restoreActor := br.NewRestoreActor(operatorCfg.BRConfig.Image)
		err = restoreActor.Reconcile(mgr)
		exitIf(err, "unable to setup restore actor")

		backupGC := &br.GCActor[*v1alpha1.BackupJob]{
			ConditionType: v1alpha1.JobConditionTypeEnded,
		}
		err = br.StartJobGCer(mgr, backupGC, &v1alpha1.BackupJob{})
		exitIf(err, "unable to setup backup GCer")

		restoreGC := &br.GCActor[*v1alpha1.RestoreJob]{
			ConditionType: v1alpha1.JobConditionTypeEnded,
		}
		err = br.StartJobGCer(mgr, restoreGC, &v1alpha1.RestoreJob{})
		exitIf(err, "unable to setup restore GCer")
	}

	if features.DefaultFeatureGate.Enabled(features.ProxySupport) {
		proxyActor := &proxyset.Actor{}
		err = proxyActor.Reconcile(mgr)
		exitIf(err, "unable to set up proxyset controller")
	} else {
		setupLog.Info(fmt.Sprintf("proxy support not enabled, skip setup proxy actor"))
	}

	if features.DefaultFeatureGate.Enabled(features.S3Reclaim) {
		bucketActor := bucketclaim.New(
			bucketclaim.WithImage(operatorCfg.BucketCleanJob.Image),
			bucketclaim.WithImagePullSecrets(operatorCfg.BucketCleanJob.ImagePullSecrets),
		)
		err = bucketActor.Reconcile(mgr)
		exitIf(err, "unable to set up bucketclaim cluster controller")
	} else {
		setupLog.Info(fmt.Sprintf("s3 reclaim feature not enabled, skip setup bucketclaim actor"))
	}

	qc, err := querycli.New(zapLogger.Named("querycli"))
	exitIf(err, "unable to create query client")
	haCliMgr := mocli.NewManager(mgr.GetClient(), zapLogger.Named("mocli-manager"))
	if features.DefaultFeatureGate.Enabled(features.CNLabel) {
		cnLabelController := cnstore.NewController(haCliMgr, qc)
		err = cnLabelController.Reconcile(mgr)
		exitIf(err, "unable to set up cnlabel controller")
	} else {
		setupLog.Info(fmt.Sprintf("cn label not enabled, skip setup cnlabel"))
	}

	if features.DefaultFeatureGate.Enabled(features.CNPool) {
		if !features.DefaultFeatureGate.Enabled(features.CNLabel) {
			panic("cn label must be enabled when cn pool is enabled")
		}
		poolController := cnpool.Actor{Logger: mgr.GetLogger().WithName("pool-controller")}
		err = poolController.Start(mgr)
		exitIf(err, "error running pool controller")

		claimController := cnclaim.NewActor(haCliMgr)
		err = claimController.Start(mgr)
		exitIf(err, "error running claim controller")

		claimSetController := cnclaimset.NewActor(directClient)
		err = claimSetController.Start(mgr)
		exitIf(err, "error running claim set controller")
	} else {
		setupLog.Info(fmt.Sprintf("cn pool not enabled, skip setup pool related controller"))
	}

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
