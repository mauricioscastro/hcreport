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
	"fmt"
	"os"
	"path/filepath"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrlRuntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	z "sigs.k8s.io/controller-runtime/pkg/log/zap"

	hcrv1 "github.com/mauricioscastro/hcreport/api/v1"
	ctrl "github.com/mauricioscastro/hcreport/internal/controller"
	"github.com/mauricioscastro/hcreport/pkg/kc"
	"github.com/mauricioscastro/hcreport/pkg/util"
	"github.com/mauricioscastro/hcreport/pkg/yjq"

	"github.com/mauricioscastro/hcreport/pkg/util/log"
	//+kubebuilder:scaffold:imports
)

var (
	scheme = runtime.NewScheme()
	logger = log.Logger().Named("hcr")

	metricsAddr          string
	enableLeaderElection bool
	probeAddr            string
	kcdump               bool
	gzip                 bool
	nologs               bool
	ns                   bool
	gkv                  bool
	//TODO: add -config (file) + -context + -xns (exclude ns regex list) + -xgkv (exclude gk,v regex list)
)

func init() {
	yjq.SilenceYqLogs()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(hcrv1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

func main() {
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection,
		"leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.BoolVar(&kcdump,
		"kcdump", false,
		"use manager as cli tool to dump the cluster")
	flag.BoolVar(&nologs,
		"nologs", false,
		"do not output pod's logs")
	flag.BoolVar(&gzip,
		"gzip", false,
		"gzip output")
	flag.BoolVar(&ns, "ns", false, "print namespaces  list")
	flag.BoolVar(&gkv, "gkv", false, "print group version kind with format 'gv,k'")

	opts := z.Options{
		Development: true,
	}

	if loggerEnv := os.Getenv("LOGGER_ENV"); loggerEnv == "prod" {
		opts.Development = false
	}

	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	if filepath.Base(os.Args[0]) == "kcdump" || kcdump {
		log.SetLoggerLevelFatal()
		os.Exit(dump())
	}

	ctrlRuntime.SetLogger(z.New(z.UseFlagOptions(&opts)))

	logger.Info("hcreport running...")

	mgr, err := ctrlRuntime.NewManager(ctrlRuntime.GetConfigOrDie(), ctrlRuntime.Options{
		Scheme:                 scheme,
		MetricsBindAddress:     metricsAddr,
		Port:                   9443,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "803a3daa.csa.latam.redhat.com",
	})
	if err != nil {
		logger.Error("unable to start manager", zap.Error(err))
		os.Exit(1)
	}
	var (
		envWhOnly   = util.GetEnv("HCR_WEBHOOK_ONLY", "false")
		envWhEnable = util.GetEnv("HCR_WEBHOOK_ENABLE", "true")
	)
	if envWhOnly == "false" {
		if err = (&ctrl.ConfigReconciler{
			Client: mgr.GetClient(),
			Scheme: mgr.GetScheme(),
		}).SetupWithManager(mgr); err != nil {
			logger.Error("unable to create controller", zap.Error(err))
			os.Exit(1)
		}
	} else {
		logger.Info("running in webhook mode only")
	}
	if envWhOnly == "true" || envWhEnable == "true" {
		if err = (&hcrv1.Config{}).SetupWebhookWithManager(mgr); err != nil {
			logger.Error("unable to create webhook", zap.Error(err))
			os.Exit(1)
		}
	} else {
		logger.Info("webhook is turned off")
	}
	// +kubebuilder:scaffold:builder
	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		logger.Error("unable to set up health check", zap.Error(err))
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		logger.Error("unable to set up ready check", zap.Error(err))
		os.Exit(1)
	}
	logger.Info("starting manager")
	if err := mgr.Start(ctrlRuntime.SetupSignalHandler()); err != nil {
		logger.Error("problem running manager", zap.Error(err))
		os.Exit(1)
	}
}

func dump() int {
	if ns {
		n, e := kc.Ns()
		if e != nil {
			return 1
		}
		ny, e := yjq.YqEval("[.items[].metadata.name] | sort | .[]", n)
		if e != nil {
			return 2
		}
		fmt.Println(ny)
	}
	if gkv {
		g, e := kc.ApiResources()
		if e != nil {
			return 3
		}
		gy, e := yjq.YqEval(`[.items[].groupVersion + "," + .items[].kind] | unique | sort | .[]`, g)
		if e != nil {
			return 4
		}
		if ns {
			fmt.Println()
		}
		fmt.Println(gy)
	}
	if !ns && !gkv {
		fmt.Println(filepath.Base(os.Args[0]))
		fmt.Println(gzip)
		fmt.Println(nologs)
	}
	return 0
}
