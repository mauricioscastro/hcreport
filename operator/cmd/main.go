/*
Copyright 2023.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"flag"
	"os"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	hcreportv1 "github.com/mauricioscastro/hcreport/api/v1"
	"github.com/mauricioscastro/hcreport/internal/controller"
	"github.com/mauricioscastro/hcreport/pkg/util"
	"github.com/mauricioscastro/hcreport/pkg/util/log"
	//+kubebuilder:scaffold:imports
)

const caBundle = "/tmp/k8s-webhook-server/serving-certs/ca"

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
	logger   = log.Logger().Named("hcr")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(hcreportv1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

func main() {
	logger.Info("hcreport running...")
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	opts := zap.Options{
		Development: true,
	}
	if loggerEnv := os.Getenv("LOGGER_ENV"); loggerEnv == "prod" {
		opts.Development = false
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		MetricsBindAddress:     metricsAddr,
		Port:                   9443,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "803a3daa.csa.latam.redhat.com",
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	if env := util.GetEnv("HCR_WEBHOOK_ONLY", "false"); env == "false" {
		if err = (&controller.ConfigReconciler{
			Client: mgr.GetClient(),
			Scheme: mgr.GetScheme(),
		}).SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", "Config")
			os.Exit(1)
		}
	} else {
		logger.Info("running in webhook mode only")
	}

	if env := util.GetEnv("HCR_WEBHOOK_ENABLE", "true"); env == "true" {
		if err = (&hcreportv1.Config{}).SetupWebhookWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create webhook", "webhook", "Config")
			os.Exit(1)
		}
		// TODO: inject caBundle into webhook configs
		// '.webhooks[].clientConfig += {"caBundle": load_str("/tmp/k8s-webhook-server/serving-certs/ca")}'
		// if d, err := kc.Cmd().Run("get validatingwebhookconfigurations.admissionregistration.k8s.io operator-validating-webhook-configuration -o yaml"); err != nil {
		// 	setupLog.Error(err, "problem getting hook info")
		// 	os.Exit(1)
		// } else {
		// 	logger.Info("webhook", z.String("deployment", d))
		// }

	} else {
		logger.Info("webhook is turned off")
	}
	///////////////////////////////////////////////////////
	///////////////////////////////////////////////////////
	///////////////////////////////////////////////////////
	// ca, err := os.ReadFile("/tmp/k8s-webhook-server/serving-certs/tls.crt")
	// if err != nil {
	// 	logger.Error("impossible to read caBundle from file")
	// }
	// logger.Debug("cert: " + string(ca))
	// logger.Debug("caBundle: " + b64.StdEncoding.EncodeToString(ca))

	// if d, err := kc.Cmd().RunYq(
	// 	"get validatingwebhookconfigurations.admissionregistration.k8s.io hcr-validating-webhook-configuration -o yaml",
	// 	"with(.metadata; del(.annotations) | del(.creationTimestamp) | del(.generation) | del(.resourceVersion) | del (.uid))",
	// 	`.webhooks[].clientConfig += {"caBundle": load_str("/tmp/k8s-webhook-server/serving-certs/ca")}`); err != nil {
	// 	setupLog.Error(err, "problem getting hook info")
	// 	// os.Exit(1)
	// } else {
	// 	logger.Info("webhook:\n" + d)
	// }

	///////////////////////////////////////////////////////
	///////////////////////////////////////////////////////
	///////////////////////////////////////////////////////

	// +kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
