package util

// TODO: split to files example from Kind: List example
// r := runner.NewCmdRunner().
// Kc("get pods -A").
// YqSplit(".items.[] | with(.metadata; del(.creationTimestamp) | del(.resourceVersion) | del(.uid) | del(.generateName) | del(.labels.pod-template-hash))",
// 	".metadata.name",
// 	"/tmp/yq")

import (
	b64 "encoding/base64"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/mauricioscastro/hcreport/pkg/runner"
	"github.com/mauricioscastro/hcreport/pkg/util/log"
	"github.com/mauricioscastro/hcreport/pkg/wrapper/yq"
	"github.com/santhosh-tekuri/jsonschema/v5"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/yaml.v3"

	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"time"
)

const (
	caBundle         = "/tmp/k8s-webhook-server/serving-certs/tls.crt"
	KindValidateHook = "validatingwebhookconfigurations.admissionregistration.k8s.io"
	KindMutateHook   = "mutatingwebhookconfigurations.admissionregistration.k8s.io"
)

var (
	caPEM  bytes.Buffer
	logger = log.Logger().Named("hcr.util")
)

func SetLoggerLevel(level zapcore.Level) {
	logger = log.ResetLoggerLevel(logger, level)
}

func ValidateJson(yamlInput string, jsonSchemaAsYaml string) error {
	var (
		input        map[string]interface{}
		schemaString string
		schema       *jsonschema.Schema
		err          error
	)
	if err = yaml.Unmarshal([]byte(yamlInput), &input); err != nil {
		return err
	}
	if schemaString, err = yq.NewYqWrapper().ToJson(jsonSchemaAsYaml); err != nil {
		return err
	}
	compiler := jsonschema.NewCompiler()
	if err = compiler.AddResource("schema.json", strings.NewReader(schemaString)); err != nil {
		return err
	}
	if schema, err = compiler.Compile("schema.json"); err != nil {
		return err
	}
	if err = schema.Validate(input); err != nil {
		return err
	}
	return err
}

func GetEnv(k string, d string) string {
	if env := os.Getenv(k); env == "" {
		return d
	} else {
		return env
	}
}

func InjectWebHookCA(webHookName string, webHookKind string) error {
	eCert := b64.StdEncoding.EncodeToString(caPEM.Bytes())
	r := runner.NewCmdRunner().
		Kc("get " + webHookKind + " " + webHookName).
		Yq(`with(.metadata; del(.annotations) | del(.creationTimestamp) | del(.generation) | del(.resourceVersion) | del (.uid) | del (."kubectl.kubernetes.io/last-applied-configuration"))`).
		Yq(`.webhooks[].clientConfig += {"caBundle": "` + eCert + `"}`).
		KcApply()
	logger.Debug(r.Out())
	return r.Err()
}

func GenCert() error {
	svcName := GetEnv("HCR_WEBHOOK_SERVICE_NAME", "hcr-webhook-service")
	svcNsName := GetEnv("HCR_WEBHOOK_SERVICE_NAMESPACE_NAME", "hcr")
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
	err = os.WriteFile(caBundle, certPEM.Bytes(), 0444)
	if err != nil {
		logger.Error("", zap.Error(err))
		return err
	}
	err = os.WriteFile(strings.Replace(caBundle, ".crt", ".key", -1), certPrivKeyPEM.Bytes(), 0444)
	if err != nil {
		logger.Error("", zap.Error(err))
		return err
	}
	return nil
}

func GetConf()
