package kc

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/mauricioscastro/hcreport/pkg/util/log"
	"github.com/mauricioscastro/hcreport/pkg/yjq"
)

const (
	TokenPath               = "/var/run/secrets/kubernetes.io/serviceaccount/token"
	CurrentContext          = ".current-context"
	queryContextForCluster  = `(%s as $c | .contexts[] | select (.name == $c)).context as $ctx | $ctx | parent | parent | parent | .clusters[] | select(.name == $ctx.cluster) | .cluster.server // ""`
	queryContextForUserAuth = `(%s as $c | .contexts[] | select (.name == $c)).context as $ctx | $ctx | parent | parent | parent | .users[] | select(.name == $ctx.user) | .user.%s`
)

var logger = log.Logger().Named("hcr.kc")

type (
	// Kc represents a kubernetes client
	Kc interface {
		SetToken(token string) Kc
		SetCert(cert tls.Certificate) Kc
		SetCluster(cluster string) Kc
		SetJsonOutput() Kc
		SetYamlOutput() Kc
		PrettyPrintJson() Kc
		Get(apiCall string) (string, error)
		Apply(apiCall string, body string) (string, error)
		Create(apiCall string, body string) (string, error)
		Replace(apiCall string, body string) (string, error)
		Delete(apiCall string, ignoreNotFound bool) (string, error)
		SetGetParams(queryParams map[string]string) Kc
		SetGetParam(name string, value string) Kc
		SetResponseTransformer(transformer ResponseTransformer) Kc
		Version() string
		Response() string
		Err() error
		Status() int
		Api() string
		setCert(cert []byte, key []byte)
		response(resp *resty.Response) (string, error)
		send(method string, apiCall string, body string) (string, error)
	}

	kc struct {
		client          *resty.Client
		yamlOutput      bool
		prettyPrintJson bool
		readOnly        bool
		api             string
		resp            string
		err             error
		version         string
		status          int
		transformer     ResponseTransformer
	}
	// Optional transformer function to Get methods
	//
	// parameters are ('api called', 'response', 'error') in this order.
	// returns ('transformed response', 'transformed error')
	ResponseTransformer func(Kc) (string, error)
)

func NewKc() Kc {
	token, err := os.ReadFile(TokenPath)
	if err != nil {
		logger.Info("could not read default sa token. skipping to kubeconfig", zap.Error(err))
	} else {
		logger.Debug("", zap.String("token from sa account", "XXX"))
		host := os.Getenv("KUBERNETES_SERVICE_HOST")
		port := os.Getenv("KUBERNETES_SERVICE_PORT_HTTPS")
		if host != "" && port != "" {
			logger.Info("kc will auth with token and env KUBERNETES_SERVICE_...")
			logger.Debug("", zap.String("host:port from env", host+":"+port))
			return NewKcWithToken(host+":"+port, string(token))
		}
	}
	return NewKcWithContext(CurrentContext)
}

func NewKcWithContext(context string) Kc {
	home, err := os.UserHomeDir()
	if err != nil {
		logger.Error("reading home info", zap.Error(err))
		return newKc()
	}
	return NewKcWithConfigContext(home+"/.kube/config", context)
}

func NewKcWithConfig(config string) Kc {
	return NewKcWithConfigContext(config, CurrentContext)
}

func NewKcWithConfigContext(config string, context string) Kc {
	kc := newKc()
	if context != CurrentContext {
		context = `"` + context + `"`
	}
	kcfg, err := os.ReadFile(config)
	if err != nil {
		logger.Error("reading config file", zap.Error(err))
		return kc
	}
	kubeCfg := string(kcfg)
	cluster, err := yjq.YqEval(fmt.Sprintf(queryContextForCluster, context), kubeCfg)
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
	token, err := yjq.YqEval(fmt.Sprintf(queryContextForUserAuth, context, `token // ""`), kubeCfg)
	if err != nil {
		logger.Error("reading token info for context", zap.Error(err))
		return kc
	}
	if len(token) == 0 {
		logger.Debug("empty user token info reading context. trying user cert...")
		cert, err := yjq.YqEval(fmt.Sprintf(queryContextForUserAuth, context, `client-certificate-data // "" | @base64d`), kubeCfg)
		if err != nil {
			logger.Error("reading user cert info for context", zap.Error(err))
			return kc
		}
		if len(cert) == 0 {
			logger.Error("empty user cert info reading context. nothing else to try for auth. returning...")
			return kc
		}
		logger.Debug("", zap.String("cert", cert))
		key, err := yjq.YqEval(fmt.Sprintf(queryContextForUserAuth, context, `client-key-data // "" | @base64d`), kubeCfg)
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
		logger.Debug("", zap.String("token from kube config", "XXX"))
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
	kc.client = resty.New().
		SetTLSClientConfig(&tls.Config{InsecureSkipVerify: true}).
		SetTimeout(5 * time.Minute).
		SetRetryCount(5).
		SetRetryWaitTime(250 * time.Millisecond)
	kc.yamlOutput = true
	kc.prettyPrintJson = false
	kc.readOnly = false
	yjq.SilenceYqLogs()
	return &kc
}

func (kc *kc) SetCluster(cluster string) Kc {
	kc.client.SetBaseURL(cluster)
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
	kc.prettyPrintJson = false
	return kc
}

func (kc *kc) SetYamlOutput() Kc {
	kc.yamlOutput = true
	return kc
}

func (kc *kc) PrettyPrintJson() Kc {
	kc.yamlOutput = false
	kc.prettyPrintJson = true
	return kc
}

func (kc *kc) SetResponseTransformer(transformer ResponseTransformer) Kc {
	kc.transformer = transformer
	return kc
}

func (kc *kc) Response() string {
	return kc.resp
}

func (kc *kc) Err() error {
	return kc.err
}

func (kc *kc) Status() int {
	return kc.status
}

func (kc *kc) Api() string {
	return kc.api
}

func (kc *kc) Get(apiCall string) (string, error) {
	kc.api = apiCall
	resp, err := kc.client.R().Get(apiCall)
	if err != nil {
		return "", err
	}
	logResponse(apiCall, resp)
	kc.resp, kc.err = kc.response(resp)
	kc.status = resp.StatusCode()
	if kc.transformer != nil {
		kc.resp, kc.err = kc.transformer(kc)
	}
	return kc.resp, kc.err
}

func (kc *kc) Apply(apiCall string, body string) (string, error) {
	return kc.send(http.MethodPatch, apiCall, body)
}

func (kc *kc) Create(apiCall string, body string) (string, error) {
	return kc.send(http.MethodPost, apiCall, body)
}

func (kc *kc) Replace(apiCall string, body string) (string, error) {
	return kc.send(http.MethodPut, apiCall, body)
}

func (kc *kc) Delete(apiCall string, ignoreNotFound bool) (string, error) {
	if kc.readOnly {
		return "", errors.New("trying to delete in read only mode")
	}
	kc.api = apiCall
	resp, err := kc.client.R().Delete(apiCall)
	if err != nil {
		return "", err
	}
	logResponse(apiCall, resp)
	if resp.StatusCode() == http.StatusNotFound && ignoreNotFound {
		return "", nil
	}
	return kc.response(resp)
}

func (kc *kc) Version() string {
	if kc.version == "" {
		kc.Get("/api")
		if kc.err == nil {
			kc.version, kc.err = yjq.YqEval(`.versions[-1] // ""`, kc.Response())
		}
		if kc.err != nil || kc.version == "" {
			logger.Error("unable to get version", zap.Error(kc.err))
		}
	}
	return kc.version
}

func (kc *kc) SetGetParams(queryParams map[string]string) Kc {
	kc.client.SetQueryParams(queryParams)
	return kc
}

func (kc *kc) SetGetParam(name string, value string) Kc {
	kc.client.SetQueryParam(name, value)
	return kc
}

func logResponse(api string, resp *resty.Response) {
	zapFields := []zap.Field{
		zap.String("req", api),
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
}

func (kc *kc) response(resp *resty.Response) (string, error) {
	contentType := strings.ToLower(resp.Header().Get("Content-Type"))
	body := string(resp.Body())
	if resp.StatusCode() >= 400 {
		if strings.Contains(contentType, "json") && kc.yamlOutput {
			if ymlBody, err := yjq.J2Y(body); err == nil {
				body = ymlBody
			}
		}
		return "", errors.New(resp.Status() + "\n" + body)
	}
	if strings.Contains(contentType, "json") {
		var err error
		if kc.yamlOutput {
			body, err = yjq.J2Y(body)
		} else if kc.prettyPrintJson {
			body, err = yjq.J2JP(body)
		}
		if err != nil {
			return "", err
		}
	}
	return body, nil
}

func (kc *kc) send(method string, apiCall string, body string) (string, error) {
	if kc.readOnly {
		return "", errors.New("trying to write in read only mode")
	}
	kc.api = apiCall
	var (
		req = kc.client.R().SetBody(body).SetHeader("Content-Type", "application/yaml")
		res *resty.Response
	)
	switch {
	case http.MethodPatch == method:
		res, kc.err = req.SetQueryParam("fieldManager", "skc-client-side-apply").
			SetHeader("Content-Type", "application/apply-patch+yaml").
			Patch(apiCall)
	case http.MethodPost == method:
		res, kc.err = req.Post(apiCall)
	case http.MethodPut == method:
		r, _ := kc.Get(apiCall)
		if rv, err := yjq.YqEval(`.metadata.resourceVersion`, r); err == nil && kc.err == nil {
			if body, kc.err = yjq.YqEval(`.metadata.resourceVersion = "`+rv+`"`, body); kc.err == nil {
				res, kc.err = req.SetBody(body).Put(apiCall)
			}
		} else if err != nil {
			kc.err = err
		}
	}
	if kc.err == nil {
		logResponse(apiCall, res)
		kc.resp, kc.err = kc.response(res)
	}
	return kc.resp, kc.err
}
