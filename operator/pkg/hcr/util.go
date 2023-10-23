package hcr

import (
	b64 "encoding/base64"
	"fmt"
	"net"

	"github.com/mauricioscastro/hcreport/pkg/util"
	"go.uber.org/zap"

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
	KindValidateHook = "validatingwebhookconfigurations.admissionregistration.k8s.io"
	KindMutateHook   = "mutatingwebhookconfigurations.admissionregistration.k8s.io"
)

var (
	caPEM      bytes.Buffer
	hookSecret string
)

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

// return false if cert was not generated/needed
func GenCert() (bool, error) {
	svcName := util.GetEnv("HCR_WEBHOOK_SERVICE_NAME", "hcr-webhook-service")
	svcNsName := util.GetEnv("HCR_WEBHOOK_SERVICE_NAMESPACE_NAME", "hcr")
	certSecretName := util.GetEnv("HCR_WEBHOOK_CERT_SECRET_NAME", "hcr-webhook-cert")
	hasSecret, err := checkHookSecretPresent(certSecretName, svcNsName)
	if err != nil {
		logger.Error("", zap.Error(err))
		return false, err
	}
	if hasSecret {
		logger.Info("certificate present in secret " + certSecretName)
		return false, nil
	}
	logger.Info("certificate not present in secret " + certSecretName + " will create one")
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
		return false, err
	}
	caBytes, err := x509.CreateCertificate(rand.Reader, ca, ca, &caPrivKey.PublicKey, caPrivKey)
	if err != nil {
		logger.Error("", zap.Error(err))
		return false, err
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
		return false, err
	}
	certBytes, err := x509.CreateCertificate(rand.Reader, cert, ca, &certPrivKey.PublicKey, caPrivKey)
	if err != nil {
		logger.Error("", zap.Error(err))
		return false, err
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
	err = updateHookSecret(
		b64.StdEncoding.EncodeToString(certPEM.Bytes()),
		b64.StdEncoding.EncodeToString(certPrivKeyPEM.Bytes()),
		b64.StdEncoding.EncodeToString(caPEM.Bytes()))
	if err != nil {
		logger.Error("", zap.Error(err))
		return false, err
	}
	return true, nil
}

func checkHookSecretPresent(name string, ns string) (bool, error) {
	cmdr.
		Kc("get secret -n" + ns + " " + name).
		Yq(`with(.metadata; del(.annotations) | del(.creationTimestamp) | del(.generation) | del(.resourceVersion) | del (.uid) | del (."kubectl.kubernetes.io/last-applied-configuration"))`)

	if cmdr.Err() != nil {
		return false, cmdr.Err()
	}
	hookSecret = cmdr.Out()
	cmdr.Yq(`.data.["tls.crt"]`).Trim()
	if cmdr.Err() != nil {
		return false, cmdr.Err()
	}
	return !cmdr.Empty(), nil
}

func updateHookSecret(cert string, key string, bundle string) error {
	yq := fmt.Sprintf(`.data.["tls.crt"] = "%s" | .data.["tls.key"] = "%s" | .data.["caBundle"] = "%s"`,
		cert, key, bundle)
	cmdr.Echo(hookSecret).Yq(yq).KcApply()
	hookSecret = cmdr.Out()
	return cmdr.Err()
}
