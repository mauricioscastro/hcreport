package yq

import (
	"strings"

	"github.com/mikefarah/yq/v4/pkg/yqlib"
)

var (
	yamlPrefs = yqlib.YamlPreferences{
		LeadingContentPreProcessing: true,
		PrintDocSeparators:          true,
		UnwrapScalar:                true,
		EvaluateTogether:            true,
	}
	yamlEncoder        = yqlib.NewYamlEncoder(2, false, yamlPrefs)
	yamlDecoder        = yqlib.NewYamlDecoder(yamlPrefs)
	prettyJsonEncoder  = yqlib.NewJSONEncoder(2, false, true)
	compactJsonEncoder = yqlib.NewJSONEncoder(-1, false, true)
)

func Y2JC(yaml string) (string, error) {
	return yqlib.NewStringEvaluator().Evaluate(".", yaml, compactJsonEncoder, yamlDecoder)
}

func Y2JP(yaml string) (string, error) {
	return yqlib.NewStringEvaluator().Evaluate(".", yaml, prettyJsonEncoder, yamlDecoder)
}

func J2Y(json string) (string, error) {
	return yqlib.NewStringEvaluator().Evaluate(".", json, yamlEncoder, yqlib.NewJSONDecoder())
}

func J2JP(json string) (string, error) {
	return yqlib.NewStringEvaluator().Evaluate(".", json, prettyJsonEncoder, yqlib.NewJSONDecoder())
}

func Eval(expr string, yaml string) (string, error) {
	res, err := yqlib.NewStringEvaluator().Evaluate(expr, yaml, yamlEncoder, yamlDecoder)
	if err == nil {
		res = strings.TrimSuffix(res, "\n")
	}
	return res, err
}

func EvalJC(expr string, json string) (string, error) {
	return yqlib.NewStringEvaluator().Evaluate(expr, json, compactJsonEncoder, yqlib.NewJSONDecoder())
}

func EvalJP(expr string, json string) (string, error) {
	return yqlib.NewStringEvaluator().Evaluate(expr, json, prettyJsonEncoder, yqlib.NewJSONDecoder())
}

func EvalJY(expr string, json string) (string, error) {
	return yqlib.NewStringEvaluator().Evaluate(expr, json, yamlEncoder, yqlib.NewJSONDecoder())
}
