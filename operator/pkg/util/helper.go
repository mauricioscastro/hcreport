package util

import (
	"os"
	"strings"

	"github.com/mauricioscastro/hcreport/pkg/util/log"
	"github.com/mauricioscastro/hcreport/pkg/wrapper/yq"
	"github.com/santhosh-tekuri/jsonschema/v5"
	"gopkg.in/yaml.v3"
)

var logger = log.Logger().Named("hcr.util")

func SetLoggerLevel(level string) {
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
