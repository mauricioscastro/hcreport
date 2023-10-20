package util

// import (
// 	b64 "encoding/base64"
// 	"fmt"
// 	"net"
// 	"os"
// 	"path/filepath"
// 	"strings"

// 	hcrv1 "github.com/mauricioscastro/hcreport/api/v1"
// 	"github.com/mauricioscastro/hcreport/pkg/runner"
// 	"go.uber.org/zap"

// 	"bytes"
// 	"crypto/rand"
// 	"crypto/rsa"
// 	"crypto/x509"
// 	"crypto/x509/pkix"
// 	"encoding/pem"
// 	"math/big"
// 	"time"
// )

// const (
// 	statusTemplate   = `{"resourceVersionGeneration": "", "phase": "", "conditions": []}`
// 	reportPath       = "/_data"
// 	caBundle         = "/tmp/k8s-webhook-server/serving-certs/tls.crt"
// 	KindValidateHook = "validatingwebhookconfigurations.admissionregistration.k8s.io"
// 	KindMutateHook   = "mutatingwebhookconfigurations.admissionregistration.k8s.io"
// )

// var (
// 	caPEM bytes.Buffer
// 	cmdr  runner.CmdRunner
// )

// func init() {
// 	cmdr = runner.NewCmdRunner()
// }

// func statusUpdate(cfg *hcrv1.Config) {

// func ExtractReportData(cfg *hcrv1.Config) {
// 	// localDir := "/tmp"
// 	// hcrDir := localDir + reportPath + "/" + cfg.Name
// 	// if _, err := os.Stat(hcrDir); os.IsNotExist(err) {
// 	// 	logger.Debug("create dir " + hcrDir)
// 	// 	err := os.Mkdir(hcrDir, os.ModePerm)
// 	// 	if err != nil {
// 	// 		logger.Warn("error creating report data directory " + reportPath + "/" + cfg.Name)
// 	// 	}
// 	// }
// 	// logger.Info("ExtractReportData", zap.String("hcrDir", hcrDir))

// 	// s := cmdr.Echo(string(cfg.Status)).Yq(`.phase = "new" | .conditions += { "ts": "2023-10-18T22:09:45-03:00" }`).ToJson().Out()

// 	// s := cmdr.Echo(emptyStatus).Yq(`.conditions += { "lastTransitionTime": "HELLO" }`).ToJson().Out()

// 	s := cmdr.Echo(string(cfg.Status)).Yq(`.conditions += { "lastTransitionTime": "HELLO" }`).ToJson().Out()

// 	fmt.Println("\n\n\n" + cfg.ResourceVersion + "." + fmt.Sprint(cfg.Generation) + "\n\n")

// 	cfg.Status = []byte(s)
// 	// `{"conditions":[{"lastProbeTime":null,"lastTransitionTime":"2023-10-14T11:08:50Z","status":"True","type":"Initialized"},{"lastProbeTime":null,"lastTransitionTime":"2023-10-15T09:56:11Z","message":"containers with unready status: [manager]","reason":"ContainersNotReady","status":"False","type":"Ready"},{"lastProbeTime":null,"lastTransitionTime":"2023-10-15T09:56:11Z","message":"containers with unready status: [manager]","reason":"ContainersNotReady","status":"False","type":"ContainersReady"},{"lastProbeTime":null,"lastTransitionTime":"2023-10-14T11:08:48Z","status":"True","type":"PodScheduled"}],"containerStatuses":[{"containerID":"containerd://3f53f85c4831f4d9517c6448059a9fa74b2eb726d06c80edc1f476fc53236b90","image":"gcr.io/kubebuilder/kube-rbac-proxy:v0.14.1","imageID":"gcr.io/kubebuilder/kube-rbac-proxy@sha256:928e64203edad8f1bba23593c7be04f0f8410c6e4feb98d9e9c2d00a8ff59048","lastState":{},"name":"kube-rbac-proxy","ready":true,"restartCount":0,"started":true,"state":{"running":{"startedAt":"2023-10-14T11:08:51Z"}}},{"containerID":"containerd://f351e51e16a4e2e7ef90679e17d4aa5a079bac0b4723eca900588d966fd48bd1","image":"quay.io/hcreport/controller:latest","imageID":"quay.io/hcreport/controller@sha256:f5ae8bcf83c4e3344b9c1fd0ca277195df3d2a3fde8fbf900ad7af4364c8037a","lastState":{"terminated":{"containerID":"containerd://f351e51e16a4e2e7ef90679e17d4aa5a079bac0b4723eca900588d966fd48bd1","exitCode":1,"finishedAt":"2023-10-19T09:12:11Z","reason":"Error","startedAt":"2023-10-19T09:12:10Z"}},"name":"manager","ready":false,"restartCount":840,"started":false,"state":{"waiting":{"message":"back-off 5m0s restarting failed container=manager pod=hcr-controller-manager-6d94f87994-fsm57_hcr(0d9862de-8e1a-4364-ac0d-2a9202045e05)","reason":"CrashLoopBackOff"}}}],"hostIP":"192.168.0.107","initContainerStatuses":[{"containerID":"containerd://59dd6288f96c8d038f012b8ff871cbd641ae42f6342fa9ef7f0e93e24aafd5b7","image":"docker.io/alpine/openssl:latest","imageID":"docker.io/alpine/openssl@sha256:9ce43aca9251a42186d81aff2638f75eea7ba1569465f6dd939848f528a33844","lastState":{},"name":"cert-provider","ready":true,"restartCount":0,"state":{"terminated":{"containerID":"containerd://59dd6288f96c8d038f012b8ff871cbd641ae42f6342fa9ef7f0e93e24aafd5b7","exitCode":0,"finishedAt":"2023-10-14T11:08:50Z","reason":"Completed","startedAt":"2023-10-14T11:08:50Z"}}}],"phase":"Running","podIP":"10.42.0.12","podIPs":[{"ip":"10.42.0.12"}],"qosClass":"Burstable","startTime":"2023-10-14T11:08:48Z"}`)

// 	//statusAddPhase(cfg, "extracting")

// 	// spec := string(cfg.Spec)
// 	// logLevel := cmdr.Echo(spec).Yq(".logLevel").Out()
// 	// logger.Info("setting log level to " + logLevel)
// 	// SetLoggerLevel(logLevel)
// 	// logger.Debug("log level set")

// }

// func statusAddPhase(cfg *hcrv1.Config, phaseName string) {
// 	ts := time.Now().Format(time.RFC3339)
// 	// s := cfg.Status
// 	// logger.Sugar().Infof("%p", &s)
// 	status := string(cfg.Status)
// 	logger.Debug("statusAddPhase", zap.String("current status", status))
// 	update := fmt.Sprintf(`.phase = "%s" | .conditions += "{"lastTransitionTime": "%s"}"`, phaseName, ts)
// 	if len(status) > 0 {
// 		// status = cmdr.Echo(status).Yq(update).ToJson().Out()
// 	} else {
// 		status = cmdr.Echo(emptyStatus).Yq(update).ToJson().Out()
// 	}
// 	logger.Debug("statusAddPhase", zap.String("new status", status))
// 	cfg.Status = []byte(status)
// }

// func InjectWebHookCA(webHookName string, webHookKind string) error {
// 	eCert := b64.StdEncoding.EncodeToString(caPEM.Bytes())
// 	cmdr.
// 		Kc("get " + webHookKind + " " + webHookName).
// 		Yq(`with(.metadata; del(.annotations) | del(.creationTimestamp) | del(.generation) | del(.resourceVersion) | del (.uid) | del (."kubectl.kubernetes.io/last-applied-configuration"))`).
// 		Yq(`.webhooks[].clientConfig += {"caBundle": "` + eCert + `"}`).
// 		KcApply()
// 	logger.Debug(cmdr.Out())
// 	return cmdr.Err()
// }

// func GenCert() error {
// 	svcName := GetEnv("HCR_WEBHOOK_SERVICE_NAME", "hcr-webhook-service")
// 	svcNsName := GetEnv("HCR_WEBHOOK_SERVICE_NAMESPACE_NAME", "hcr")
// 	ca := &x509.Certificate{
// 		SerialNumber: big.NewInt(2019),
// 		Subject: pkix.Name{
// 			Organization: []string{"hcreport"},
// 		},
// 		NotBefore:             time.Now(),
// 		NotAfter:              time.Now().AddDate(10, 0, 0),
// 		IsCA:                  true,
// 		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
// 		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
// 		BasicConstraintsValid: true,
// 	}
// 	caPrivKey, err := rsa.GenerateKey(rand.Reader, 2048)
// 	if err != nil {
// 		logger.Error("", zap.Error(err))
// 		return err
// 	}
// 	caBytes, err := x509.CreateCertificate(rand.Reader, ca, ca, &caPrivKey.PublicKey, caPrivKey)
// 	if err != nil {
// 		logger.Error("", zap.Error(err))
// 		return err
// 	}
// 	pem.Encode(&caPEM, &pem.Block{
// 		Type:  "CERTIFICATE",
// 		Bytes: caBytes,
// 	})
// 	var caPrivKeyPEM bytes.Buffer
// 	pem.Encode(&caPrivKeyPEM, &pem.Block{
// 		Type:  "RSA PRIVATE KEY",
// 		Bytes: x509.MarshalPKCS1PrivateKey(caPrivKey),
// 	})
// 	cert := &x509.Certificate{
// 		SerialNumber: big.NewInt(1658),
// 		Subject: pkix.Name{
// 			Organization: []string{"hcreport"},
// 		},
// 		IPAddresses:  []net.IP{net.IPv4(127, 0, 0, 1), net.IPv6loopback},
// 		NotBefore:    time.Now(),
// 		NotAfter:     time.Now().AddDate(10, 0, 0),
// 		SubjectKeyId: []byte{1, 2, 3, 4, 6},
// 		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
// 		KeyUsage:     x509.KeyUsageDigitalSignature,
// 		DNSNames:     []string{svcName + "." + svcNsName + ".svc.cluster.local", svcName + "." + svcNsName + ".svc"},
// 	}
// 	certPrivKey, err := rsa.GenerateKey(rand.Reader, 4096)
// 	if err != nil {
// 		logger.Error("", zap.Error(err))
// 		return err
// 	}
// 	certBytes, err := x509.CreateCertificate(rand.Reader, cert, ca, &certPrivKey.PublicKey, caPrivKey)
// 	if err != nil {
// 		logger.Error("", zap.Error(err))
// 		return err
// 	}
// 	var certPEM bytes.Buffer
// 	pem.Encode(&certPEM, &pem.Block{
// 		Type:  "CERTIFICATE",
// 		Bytes: certBytes,
// 	})
// 	var certPrivKeyPEM bytes.Buffer
// 	pem.Encode(&certPrivKeyPEM, &pem.Block{
// 		Type:  "RSA PRIVATE KEY",
// 		Bytes: x509.MarshalPKCS1PrivateKey(certPrivKey),
// 	})
// 	caPEM.WriteString(certPEM.String())
// 	os.MkdirAll(filepath.Join(os.TempDir(), "k8s-webhook-server", "serving-certs"), os.ModePerm)
// 	err = os.WriteFile(caBundle, certPEM.Bytes(), 0444)
// 	if err != nil {
// 		logger.Error("", zap.Error(err))
// 		return err
// 	}
// 	err = os.WriteFile(strings.Replace(caBundle, ".crt", ".key", -1), certPrivKeyPEM.Bytes(), 0444)
// 	if err != nil {
// 		logger.Error("", zap.Error(err))
// 		return err
// 	}
// 	return nil
// }

// // TODO: split to files example from Kind: List example
// // r := runner.NewCmdRunner().
// // Kc("get pods -A").
// // YqSplit(".items.[] | with(.metadata; del(.creationTimestamp) | del(.resourceVersion) | del(.uid) | del(.generateName) | del(.labels.pod-template-hash))",
// // 	".metadata.name",
// // 	"/tmp/yq")
