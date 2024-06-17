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
	"strings"

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
	kcli "github.com/mauricioscastro/hcreport/pkg/kc"
	"github.com/mauricioscastro/hcreport/pkg/util"
	"github.com/mauricioscastro/hcreport/pkg/yjq"

	"github.com/mauricioscastro/hcreport/pkg/util/log"
	//+kubebuilder:scaffold:imports
)

type nsExcludeList []string
type gvkExcludeList []string

var (
	scheme = runtime.NewScheme()
	logger = log.Logger().Named("hcr")
	home   string

	metricsAddr          string
	enableLeaderElection bool
	probeAddr            string

	// cli dump options
	kcdump    bool
	gzip      bool
	tgz       bool
	nologs    bool
	ns        bool
	gvk       bool
	xns       nsExcludeList
	xgvk      gvkExcludeList
	targetDir string
	format    string
	config    string
	context   string
	logLevel  string
)

func init() {
	yjq.SilenceYqLogs()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(hcrv1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
	var err error
	home, err = os.UserHomeDir()
	if err != nil {
		logger.Error("reading home info", zap.Error(err))
		os.Exit(-1)
	}
}

func main() {
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false, "Enable leader election for controller manager. Enabling this will ensure there is only one active controller manager.")
	flag.BoolVar(&kcdump, "kcdump", false, "use manager as cli tool to dump the cluster")
	flag.BoolVar(&nologs, "nologs", false, "do not output pod's logs")
	flag.BoolVar(&gzip, "gzip", false, "gzip output")
	flag.BoolVar(&tgz, "tgz", false, "gzip output")
	flag.BoolVar(&ns, "ns", false, "print namespaces  list")
	flag.BoolVar(&gvk, "gvk", false, "print group version kind with format 'gv,k'")
	flag.Var(&xns, "xns", "regex to match and exclude unwanted namespaces. can be used multiple times.")
	flag.Var(&xgvk, "xgvk", "regex to match and exclude unwanted groupVersion and kind. format is 'gv,k' where gv is regex to capture gv and k is regex to capture kind. ex: -xgvk metrics.*,Pod.*")
	flag.StringVar(&targetDir, "targetDir", ".kcdump", "target directory where the extracted cluster data goes. directory will be recreated from scratch.")
	flag.StringVar(&format, "format", "yaml", "output format. one of 'yaml', 'json', 'json_pretty', 'json_lines', 'json_lines_wrapped'. default is yaml")
	flag.StringVar(&config, "config", filepath.FromSlash(home+"/.kube/config"), "kube config file or read from stdin.")
	flag.StringVar(&context, "context", kc.CurrentContext, "kube config context to use")
	flag.StringVar(&logLevel, "logLevel", "fatal", "use one of: info, warn, ")

	flag.Parse()

	if filepath.Base(os.Args[0]) == "kcdump" || kcdump {
		os.Exit(dump())
	}

	opts := z.Options{
		Development: true,
	}

	if loggerEnv := os.Getenv("LOGGER_ENV"); loggerEnv == "prod" {
		opts.Development = false
	}

	opts.BindFlags(flag.CommandLine)

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
	log.SetLoggerLevel(logLevel)
	kc := kcli.NewKcWithConfigContext(config, context)
	if kc == nil {
		fmt.Fprintf(os.Stderr, "unable to start k8s client from config file '%s' and context '%s'\n", config, context)
		os.Exit(-1)
	}
	if ns {
		n, e := kc.Ns()
		if e != nil {
			return 1
		}
		for _, re := range xns {
			n, e = yjq.YqEval(`del(.items[] | select(.metadata.name | test("%s")))`, n, re)
			if e != nil {
				return 2
			}
		}
		n, e = yjq.YqEval("[.items[].metadata.name] | sort | .[]", n)
		if e != nil {
			return 3
		}
		// fmt.Println("namespace")
		fmt.Println(n)
	}
	if gvk {
		g, e := kc.ApiResources()
		if e != nil {
			return 4
		}
		for _, re := range xgvk {
			r := strings.Split(re, ",")
			if len(r) == 1 {
				r = append(r, ".*")
			}
			g, e = yjq.YqEval(`del(.items[] | select(.groupVersion | test("%s") and .kind | test("%s")))`, g, r[0], r[1])
			if e != nil {
				return 5
			}
		}
		g, e = yjq.YqEval(`with(.items[]; .verbs = (.verbs | to_entries)) | .items[] | select(.available and .verbs[].value == "get") | [.groupVersion + "," + .kind] | sort | .[]`, g)
		if e != nil {
			return 6
		}
		if ns {
			fmt.Println()
		}
		// fmt.Println("groupVersion,kind")
		fmt.Println(g)
	}
	if !ns && !gvk {
		outputfmt, e := kcli.FormatCodeFromString(format)
		if e != nil {
			fmt.Fprintf(os.Stderr, "%s\n", e.Error())
			return 7
		}
		if e = kc.Dump(targetDir, xns, xgvk, nologs, gzip, tgz, outputfmt, 0, nil); e != nil {
			fmt.Fprintf(os.Stderr, "%s\n", e.Error())
			return 8
		}
	}
	return 0
}

func (i *nsExcludeList) String() string {
	return fmt.Sprint(*i)
}

func (i *nsExcludeList) Set(value string) error {
	*i = append(*i, value)
	return nil
}

func (i *gvkExcludeList) String() string {
	return fmt.Sprint(*i)
}

func (i *gvkExcludeList) Set(value string) error {
	*i = append(*i, value)
	return nil
}
