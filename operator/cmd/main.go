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
	"github.com/mauricioscastro/hcreport/pkg/util/log"
	//+kubebuilder:scaffold:imports
)

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
		// LeaderElectionReleaseOnCancel defines if the leader should step down voluntarily
		// when the Manager ends. This requires the binary to immediately end when the
		// Manager is stopped, otherwise, this setting is unsafe. Setting this significantly
		// speeds up voluntary leader transitions as the new leader don't have to wait
		// LeaseDuration time first.
		//
		// In the default scaffold provided, the program ends immediately after
		// the manager stops, so would be fine to enable this option. However,
		// if you are doing or is intended to do any operation such as perform cleanups
		// after the manager stops then its usage might be unsafe.
		// LeaderElectionReleaseOnCancel: true,
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	if err = (&controller.ConfigReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Config")
		os.Exit(1)
	}

	if env := os.Getenv("ENABLE_WEBHOOKS"); env == "true" {
		if err = (&hcreportv1.Config{}).SetupWebhookWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create webhook", "webhook", "Config")
			os.Exit(1)
		}
	}
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

////////////////////////////////////////////////////////////////////////////////////////////
// SANDBOX
////////////////////////////////////////////////////////////////////////////////////////////

// package main

// import (
// 	"fmt"
// 	"strings"

// 	"github.com/mauricioscastro/hcreport/pkg/util"
// 	"github.com/mauricioscastro/hcreport/pkg/util/log"
// 	kc "github.com/mauricioscastro/hcreport/pkg/wrapper"
// 	yq "github.com/mauricioscastro/hcreport/pkg/wrapper"
// )

// var schemaText = `
// $schema: https://json-schema.org/draft/2020-12/schema
// $id: http://hcreport
// title: Product
// description: A product from Acme's catalog
// type: object
// properties:
//   productId:
//     description: The unique identifier for a product
//     type: integer
//   productName:
//     description: Name of the product
//     type: string
//   price:
//     description: The price of the product
//     type: number
//     exclusiveMinimum: 0
//   tags:
//     description: Tags for the product
//     type: array
//     items:
//       type: string
//     minItems: 1
//     uniqueItems: true
//   dimensions:
//     type: object
//     properties:
//       length:
//         type: number
//       width:
//         type: number
//       height:
//         type: number
//     required:
//       - length
//       - width
//       - height
// required:
//   - productId
//   - productName
//   - price
// `

// var yamlText = `
// productId: 1
// #productName: A green door
// price: 12.50
// tags:
// - home
// - green
// `

// func main() {

// 	log.SilenceKcLogs()
// 	log.SilenceYqLogs()

// 	logger := log.Logger()
// 	logger.Info("running...")

// 	// yqw := yq.NewYqWrapper()
// 	// 	res, err := yqw.EvalEach(".me | length",
// 	// 		`me: Mauricio
// 	// her: Debora`)

// 	// res, err := yqw.ToJson(schemaText)
// 	// if err != nil {
// 	// 	fmt.Println("err:")
// 	// 	fmt.Println(err)
// 	// } else {
// 	// 	fmt.Println("out:")
// 	// 	fmt.Println(strings.TrimSuffix(res, "\n"))
// 	// }

// 	kcw := kc.NewKcWrapper()

// 	res1, err1 := kcw.RunYq("get nodes",
// 		".items[].metadata",
// 		".labels",
// 		`."beta.kubernetes.io/instance-type"`)

// 	// res1, err1 := kc.GetNS()
// 	// res1, err1 := kc.GetApiResources()

// 	// if err != nil {
// 	// 	fmt.Println("err:")
// 	// 	fmt.Println(err)
// 	// } else {
// 	// 	fmt.Println("out:")
// 	// 	for _, r := range res {
// 	// 		fmt.Println(r)
// 	// 	}
// 	// }

// 	// res1, err1 := kc.Cmd().Run("api-versions")
// 	// res1, err1 := kc.Cmd().Run("get nodes")
// 	// res1, err1 := kc.Cmd().Run("cluster-info")
// 	// // res1, err1 := kc.Cmd().Run("version")
// 	// // res1, err1 := kc.Cmd().Run("logs -n openshift-ingress router-mscastro-net-6759d57968-qjm2h")
// 	// // res1, err1 := kc.Cmd().Run("config current-context ")

// 	if err1 != nil {
// 		fmt.Println("err:")
// 		fmt.Println(err1)
// 	} else {
// 		fmt.Println("out:")
// 		fmt.Println(res1)
// 	}

// 	err2 := util.Validate(yamlText, schemaText)

// 	if err2 != nil {
// 		fmt.Println("err:")
// 		fmt.Println(err2)
// 	} else {
// 		fmt.Println("VALID DOCUMENT")
// 	}

// 	// if e != nil {
// 	// 	fmt.Println("err:")
// 	// 	fmt.Println(e)
// 	// } else {
// 	// 	fmt.Println("ret:\n" + ret)
// 	// }

// 	// ns := "hello\nhi"

// 	// fmt.Print(ns)

// 	// s, e := util.Sed("3d", "hello\nhi\n\n")

// 	// if e != nil {
// 	// 	panic(e)
// 	// }

// 	// fmt.Print(s)
// 	//	fmt.Println(e)

// 	// ns, e := util.GetNS()

// 	// if e != nil {
// 	// 	panic(e)
// 	// }

// 	// for _, name := range ns {
// 	// 	fmt.Println(name)
// 	// }
// 	// fmt.Println(len(ns))

// 	// kcCmd := kcw.NewKCWrapper()

// 	// out, err := kcCmd.RunSed("api-resources -o wide --sort-by=name --no-headers=true",
// 	// 	"s/\\s+/ /g",
// 	// 	"s/,/;/g",
// 	// 	"s/ /,/g",
// 	// 	"s/^([^\\s,]+,)((?:[^\\s,]+,){4})([^\\s,]+)*$/$1,$2$3/g")

// 	// if err != nil {
// 	// 	panic(err)
// 	// }

// 	// r := csv.NewReader(strings.NewReader(out))

// 	// var resources [][]string
// 	// resources, err = r.ReadAll()

// 	// if err != nil {
// 	// 	panic(err)
// 	// }

// 	// fmt.Print(resources)
// }
