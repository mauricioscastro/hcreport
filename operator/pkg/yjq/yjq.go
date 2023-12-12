package yjq

import (
	"bytes"
	"io"
	"strings"

	gjs "github.com/goccy/go-json"
	"github.com/itchyny/gojq"
	"github.com/mikefarah/yq/v4/pkg/yqlib"

	"gopkg.in/op/go-logging.v1"
)

var (
	yamlPrefs = yqlib.YamlPreferences{
		LeadingContentPreProcessing: true,
		PrintDocSeparators:          true,
		UnwrapScalar:                true,
		EvaluateTogether:            true,
	}
)

func SilenceYqLogs() {
	bke := logging.NewLogBackend(io.Discard, "", 0)
	bkel := logging.AddModuleLevel(bke)
	yqlib.GetLogger().SetBackend(bkel)
}

func Y2JC(yaml string) (string, error) {
	yamlDecoder := yqlib.NewYamlDecoder(yamlPrefs)
	compactJsonEncoder := yqlib.NewJSONEncoder(-1, false, true)
	return yqlib.NewStringEvaluator().Evaluate(".", yaml, compactJsonEncoder, yamlDecoder)
}

func Y2JP(yaml string) (string, error) {
	yamlDecoder := yqlib.NewYamlDecoder(yamlPrefs)
	prettyJsonEncoder := yqlib.NewJSONEncoder(2, false, true)
	return yqlib.NewStringEvaluator().Evaluate(".", yaml, prettyJsonEncoder, yamlDecoder)
}

func J2Y(json string) (string, error) {
	yamlEncoder := yqlib.NewYamlEncoder(2, false, yamlPrefs)
	return yqlib.NewStringEvaluator().Evaluate(".", json, yamlEncoder, yqlib.NewJSONDecoder())
}

func J2JP(json string) (string, error) {
	prettyJsonEncoder := yqlib.NewJSONEncoder(2, false, true)
	return yqlib.NewStringEvaluator().Evaluate(".", json, prettyJsonEncoder, yqlib.NewJSONDecoder())
}

func YqEval(expr string, yaml string) (string, error) {
	yamlEncoder := yqlib.NewYamlEncoder(2, false, yamlPrefs)
	yamlDecoder := yqlib.NewYamlDecoder(yamlPrefs)
	res, err := yqlib.NewStringEvaluator().Evaluate(expr, yaml, yamlEncoder, yamlDecoder)
	if err == nil {
		res = strings.TrimSuffix(res, "\n")
	}
	return res, err
}

func YqEvalJ2JC(expr string, json string) (string, error) {
	compactJsonEncoder := yqlib.NewJSONEncoder(-1, false, true)
	return yqlib.NewStringEvaluator().Evaluate(expr, json, compactJsonEncoder, yqlib.NewJSONDecoder())
}

func YqEvalJ2JP(expr string, json string) (string, error) {
	prettyJsonEncoder := yqlib.NewJSONEncoder(2, false, true)
	return yqlib.NewStringEvaluator().Evaluate(expr, json, prettyJsonEncoder, yqlib.NewJSONDecoder())
}

func YqEvalJ2Y(expr string, json string) (string, error) {
	yamlEncoder := yqlib.NewYamlEncoder(2, false, yamlPrefs)
	return yqlib.NewStringEvaluator().Evaluate(expr, json, yamlEncoder, yqlib.NewJSONDecoder())
}

func JqEval(expr string, json string) (string, error) {
	query, err := gojq.Parse(expr)
	if err != nil {
		return "", err
	}
	code, err := gojq.Compile(query)
	if err != nil {
		return "", err
	}
	jsonIn := make(map[string]interface{})
	gjs.Unmarshal([]byte(json), &jsonIn)

	var out bytes.Buffer
	iter := code.Run(jsonIn)
	for {
		v, ok := iter.Next()
		if !ok {
			break
		}
		if err, ok := v.(error); ok {
			return "", err
		}
		if r, err := gojq.Marshal(v); err == nil {
			out.Write(r)
			out.Write([]byte("\n"))
		} else {
			return "", err
		}
	}
	return string(out.Bytes()[0 : out.Len()-1]), nil
}
