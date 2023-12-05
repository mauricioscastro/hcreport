package kc

import (
	"crypto/tls"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/go-resty/resty/v2"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/mauricioscastro/hcreport/pkg/util"
	"github.com/mauricioscastro/hcreport/pkg/util/log"
	"github.com/mauricioscastro/hcreport/pkg/yqjq/yq"
)

const (
	TokenPath               = "/var/run/secrets/kubernetes.io/serviceaccount/token"
	queryContextForCluster  = `(%s as $c | .contexts[] | select (.name == $c)).context as $ctx | $ctx | parent | parent | parent | .clusters[] | select(.name == $ctx.cluster) | .cluster.server // ""`
	queryContextForUserAuth = `(%s as $c | .contexts[] | select (.name == $c)).context as $ctx | $ctx | parent | parent | parent | .users[] | select(.name == $ctx.user) | .user.%s` // [client-certificate-data,client-key-data] // "" | @base64d; token // ""
)

var (
	logger = log.Logger().Named("hcr.kc")
)

type Kc interface {
	SetToken(token string) Kc
	SetCert(cert tls.Certificate) Kc
	SetCluster(cluster string) Kc
	SetJsonOutput() Kc
	PrettyPrintJson() Kc
	Get(apiCall string) (string, error)
	setCert(cert []byte, key []byte)
}

type kc struct {
	client          *resty.Client
	cluster         string
	yamlOutput      bool
	prettyPrintJson bool
}

func NewKc() Kc {
	token, err := os.ReadFile(TokenPath)
	if err != nil {
		logger.Error("could not read default sa token. skipping to kubeconfig", zap.Error(err))
	} else {
		logger.Debug("", zap.String("token from sa account", "XXX"))
		host := util.GetEnv("KUBERNETES_SERVICE_HOST", "")
		port := util.GetEnv("KUBERNETES_SERVICE_PORT_HTTPS", "")
		if host != "" && port != "" {
			logger.Info("kc will auth with token and env KUBERNETES_SERVICE_...")
			logger.Debug("", zap.String("host:port from env", host+":"+port))
			return NewKcWithToken(host+":"+port, string(token))
		}
	}
	return NewKcWithContext(".current-context")
}

func NewKcWithContext(context string) Kc {
	kc := newKc()
	home, err := os.UserHomeDir()
	if err != nil {
		logger.Error("reading home info", zap.Error(err))
		return kc
	}
	return NewKcWithConfigContext(context, home+"/.kube/config")
}

func NewKcWithConfig(config string) Kc {
	return NewKcWithConfigContext(".current-context", config)
}

func NewKcWithConfigContext(context string, config string) Kc {
	kc := newKc()
	if context != ".current-context" {
		context = `"` + context + `"`
	}
	kcfg, err := os.ReadFile(config)
	if err != nil {
		logger.Error("reading config file", zap.Error(err))
		return kc
	}
	kubeCfg := string(kcfg)
	cluster, err := yq.Eval(fmt.Sprintf(queryContextForCluster, context), kubeCfg)
	if err != nil {
		logger.Error("reading cluster info for context", zap.Error(err))
		return kc
	}
	if len(cluster) == 0 {
		logger.Error("empty cluster info reading context")
		return kc
	}
	logger.Debug("", zap.String("cluster", cluster))
	kc.SetCluster(cluster)
	token, err := yq.Eval(fmt.Sprintf(queryContextForUserAuth, context, `token // ""`), kubeCfg)
	if err != nil {
		logger.Error("reading token info for context", zap.Error(err))
		return kc
	}
	logger.Debug("", zap.String("token from kube config", "XXX"))
	if len(token) == 0 {
		logger.Debug("empty user token info reading context. trying user cert...")
		cert, err := yq.Eval(fmt.Sprintf(queryContextForUserAuth, context, `client-certificate-data // "" | @base64d`), kubeCfg)
		if err != nil {
			logger.Error("reading user cert info for context", zap.Error(err))
			return kc
		}
		if len(cert) == 0 {
			logger.Error("empty user cert info reading context. nothing else to try for auth. returning...")
			return kc
		}
		logger.Debug("", zap.String("cert", cert))
		key, err := yq.Eval(fmt.Sprintf(queryContextForUserAuth, context, `client-key-data // "" | @base64d`), kubeCfg)
		if err != nil {
			logger.Error("reading user cert key info for context", zap.Error(err))
			return kc
		}
		if len(key) == 0 {
			logger.Error("empty user cert key info reading context. nothing else to try for auth. returning...")
			return kc
		}
		logger.Debug("", zap.String("key", "XXX"))
		kc.setCert([]byte(cert), []byte(key))
	} else {
		kc.SetToken(token)
	}
	logger.Info("kc will auth with kube config")
	return kc
}

func NewKcWithCert(cluster string, cert []byte, key []byte) Kc {
	kc := newKc().SetCluster(cluster)
	kc.setCert(cert, key)
	return kc
}

func NewKcWithToken(cluster string, token string) Kc {
	return newKc().SetCluster(cluster).SetToken(token)
}

func (kc *kc) setCert(cert []byte, key []byte) {
	c, err := tls.X509KeyPair(cert, key)
	if err != nil {
		logger.Error("cert creation failure", zap.Error(err))
	} else {
		kc.SetCert(c)
	}
}

func newKc() Kc {
	kc := kc{}
	kc.client = resty.New().SetTLSClientConfig(&tls.Config{InsecureSkipVerify: true})
	kc.yamlOutput = true
	kc.prettyPrintJson = false
	return &kc
}

func (kc *kc) SetCluster(cluster string) Kc {
	kc.cluster = cluster
	return kc
}

func (kc *kc) SetToken(token string) Kc {
	kc.client.SetAuthToken(token)
	return kc
}

func (kc *kc) SetCert(cert tls.Certificate) Kc {
	kc.client.SetCertificates(cert)
	return kc
}

func (kc *kc) SetJsonOutput() Kc {
	kc.yamlOutput = false
	return kc
}

func (kc *kc) PrettyPrintJson() Kc {
	kc.yamlOutput = false
	kc.prettyPrintJson = true
	return kc
}

func (kc *kc) Get(apiCall string) (string, error) {
	resp, err := kc.client.R().Get(kc.cluster + apiCall)
	if err != nil {
		return "R.Get err", err
	}
	zapFields := []zap.Field{
		zap.String("req", apiCall),
		zap.Int("status code", resp.StatusCode()),
		zap.String("status", resp.Status()),
		zap.String("proto", resp.Proto()),
		zap.Int64("time", resp.Time().Milliseconds()),
		zap.Time("received at", resp.ReceivedAt()),
	}
	if logger.Level() == zapcore.DebugLevel {
		for name, values := range resp.Header() {
			for _, value := range values {
				zapFields = append(zapFields, zap.String(name, value))
			}
		}
	}
	logger.Debug("http resp", zapFields...)
	contentType := strings.ToLower(resp.Header().Get("Content-Type"))
	body := string(resp.Body())
	if resp.StatusCode() >= 400 {
		if strings.Contains(contentType, "json") && kc.yamlOutput {
			if ymlBody, err := yq.J2Y(body); err == nil {
				body = ymlBody
			}
		}
		return "", errors.New(resp.Status() + "\n" + body)
	}
	if strings.Contains(contentType, "json") {
		if kc.yamlOutput {
			body, err = yq.J2Y(body)
		} else if kc.prettyPrintJson {
			body, err = yq.J2JP(body)
		}
		if err != nil {
			return "", err
		}
	}
	return body, nil
}
