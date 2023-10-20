package controller

import (
	"context"
	b64 "encoding/base64"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	hcrv1 "github.com/mauricioscastro/hcreport/api/v1"
	"github.com/mauricioscastro/hcreport/pkg/runner"
	"github.com/mauricioscastro/hcreport/pkg/util"
	"go.uber.org/zap"
	ctrl "sigs.k8s.io/controller-runtime"

	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"time"
)

type reconciler struct {
	r   *ConfigReconciler
	ctx context.Context
	cfg *hcrv1.Config
}

const (
	reportPath       = "/_data"
	crtFile          = "/tmp/k8s-webhook-server/serving-certs/tls.crt"
	KindValidateHook = "validatingwebhookconfigurations.admissionregistration.k8s.io"
	KindMutateHook   = "mutatingwebhookconfigurations.admissionregistration.k8s.io"
)

var (
	caPEM bytes.Buffer
	cmdr  runner.CmdRunner
)

func init() {
	cmdr = runner.NewCmdRunner()
}

func RunReport(r *ConfigReconciler, ctx context.Context, cfg *hcrv1.Config) (ctrl.Result, error) {
	logger.Info("ExtractReportData...")
	rec := reconciler{r, ctx, cfg}
	statusCheck(rec)
	if err := statusAdd("extracting", rec); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func statusCheck(hcr reconciler) {
	if len(hcr.cfg.Status) == 0 {
		hcr.cfg.Status = cmdr.Echo(statusTemplate).ToJson().BytesOut()
	}
}

func statusAdd(phase string, hcr reconciler) error {
	ts := time.Now().Format(time.RFC3339)
	jq := fmt.Sprintf(`.phase = "%s" | .transitions.last = "%s" | .transitions.next = "unscheduled"`, phase, ts)
	if cmdr.Echo(hcr.cfg.Status).Jq(jq).Err() == nil {
		hcr.cfg.Status = cmdr.BytesOut()
		if err := hcr.r.Status().Update(hcr.ctx, hcr.cfg); err != nil {
			logger.Error("unable to update status", zap.Error(err))
			return err
		}
	}
	return nil
}

func InjectWebHookCA(webHookName string, webHookKind string) error {
	eCert := b64.StdEncoding.EncodeToString(caPEM.Bytes())
	cmdr.
		Kc("get " + webHookKind + " " + webHookName).
		Yq(`with(.metadata; del(.annotations) | del(.creationTimestamp) | del(.generation) | del(.resourceVersion) | del (.uid) | del (."kubectl.kubernetes.io/last-applied-configuration"))`).
		Yq(`.webhooks[].clientConfig += {"caBundle": "` + eCert + `"}`).
		KcApply()
	logger.Sugar().Debugf("InjectWebHookCA:\n%s", cmdr.Out())
	return cmdr.Err()
}

func GenCert() error {
	svcName := util.GetEnv("HCR_WEBHOOK_SERVICE_NAME", "hcr-webhook-service")
	svcNsName := util.GetEnv("HCR_WEBHOOK_SERVICE_NAMESPACE_NAME", "hcr")
	ca := &x509.Certificate{
		SerialNumber: big.NewInt(2019),
		Subject: pkix.Name{
			Organization: []string{"hcreport"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		IsCA:                  true,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}
	caPrivKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		logger.Error("", zap.Error(err))
		return err
	}
	caBytes, err := x509.CreateCertificate(rand.Reader, ca, ca, &caPrivKey.PublicKey, caPrivKey)
	if err != nil {
		logger.Error("", zap.Error(err))
		return err
	}
	pem.Encode(&caPEM, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: caBytes,
	})
	var caPrivKeyPEM bytes.Buffer
	pem.Encode(&caPrivKeyPEM, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(caPrivKey),
	})
	cert := &x509.Certificate{
		SerialNumber: big.NewInt(1658),
		Subject: pkix.Name{
			Organization: []string{"hcreport"},
		},
		IPAddresses:  []net.IP{net.IPv4(127, 0, 0, 1), net.IPv6loopback},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().AddDate(10, 0, 0),
		SubjectKeyId: []byte{1, 2, 3, 4, 6},
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:     x509.KeyUsageDigitalSignature,
		DNSNames:     []string{svcName + "." + svcNsName + ".svc.cluster.local", svcName + "." + svcNsName + ".svc"},
	}
	certPrivKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		logger.Error("", zap.Error(err))
		return err
	}
	certBytes, err := x509.CreateCertificate(rand.Reader, cert, ca, &certPrivKey.PublicKey, caPrivKey)
	if err != nil {
		logger.Error("", zap.Error(err))
		return err
	}
	var certPEM bytes.Buffer
	pem.Encode(&certPEM, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certBytes,
	})
	var certPrivKeyPEM bytes.Buffer
	pem.Encode(&certPrivKeyPEM, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(certPrivKey),
	})
	caPEM.WriteString(certPEM.String())
	os.MkdirAll(filepath.Join(os.TempDir(), "k8s-webhook-server", "serving-certs"), os.ModePerm)
	err = os.WriteFile(crtFile, certPEM.Bytes(), 0444)
	if err != nil {
		logger.Error("", zap.Error(err))
		return err
	}
	err = os.WriteFile(strings.Replace(crtFile, ".crt", ".key", -1), certPrivKeyPEM.Bytes(), 0444)
	if err != nil {
		logger.Error("", zap.Error(err))
		return err
	}
	return nil
}

// TODO: split to files example from Kind: List example
// r := runner.NewCmdRunner().
// Kc("get pods -A").
// YqSplit(".items.[] | with(.metadata; del(.creationTimestamp) | del(.resourceVersion) | del(.uid) | del(.generateName) | del(.labels.pod-template-hash))",
// 	".metadata.name",
// 	"/tmp/yq")
