package util

import (
	b64 "encoding/base64"
	"os"
	"strings"

	"github.com/mauricioscastro/hcreport/pkg/util/log"
	"github.com/mauricioscastro/hcreport/pkg/wrapper/kc"
	"github.com/mauricioscastro/hcreport/pkg/wrapper/yq"
	"github.com/santhosh-tekuri/jsonschema/v5"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

const (
	caBundle         = "/tmp/k8s-webhook-server/serving-certs/tls.crt"
	KindValidateHook = "validatingwebhookconfigurations.admissionregistration.k8s.io"
	KindMutateHook   = "mutatingwebhookconfigurations.admissionregistration.k8s.io"
)

var logger = log.Logger().Named("hcr.util")

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
	yqw := yq.NewYqWrapper()
	if schemaString, err = yqw.ToJson(jsonSchemaAsYaml); err != nil {
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
	if cert, err := os.ReadFile(caBundle); err != nil {
		logger.Error("", zap.Error(err))
		return err
	} else {
		eCert := b64.StdEncoding.EncodeToString(cert)
		if d, err := kc.Cmd().RunYq(
			"get "+webHookKind+" "+webHookName,
			"with(.metadata; del(.annotations) | del(.creationTimestamp) | del(.generation) | del(.resourceVersion) | del (.uid))",
			`.webhooks[].clientConfig += {"caBundle": "`+eCert+`"}`); err != nil {
			logger.Error("", zap.Error(err))
			return err
		} else {
			logger.Info("", zap.String("webhook config deployment", d))
		}
	}
	return nil
}
