package jq

import (
	"bytes"

	gjs "github.com/goccy/go-json"
	"github.com/itchyny/gojq"
)

func Eval(expr string, json string) (string, error) {
	query, err := gojq.Parse(expr)
	if err != nil {
		return "", err
	}
	code, err := gojq.Compile(query)
	if err != nil {
		return "", err
	}
	result := make(map[string]interface{})
	gjs.Unmarshal([]byte(json), &result)

	var out bytes.Buffer
	iter := code.Run(result)
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
