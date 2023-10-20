package jqw

import (
	"bytes"
	"errors"
	"strings"

	"github.com/itchyny/gojq/cli"
)

type JqWrapper interface {
	Run(args []string, stdin string) (string, error)
}

type jqWrapper struct {
	out bytes.Buffer
	err bytes.Buffer
	in  bytes.Buffer
}

func NewJqWrapper() JqWrapper {
	jqw := jqWrapper{}
	return &jqw
}

func (jqw *jqWrapper) Run(args []string, stdin string) (string, error) {
	jqw.out.Reset()
	jqw.err.Reset()
	jqw.in.Reset()
	jqw.in.WriteString(stdin)
	if cli.CmdRun(&jqw.in, &jqw.out, &jqw.err, args) != 0 {
		return "", errors.New(jqw.err.String())
	}
	return strings.TrimSuffix(jqw.out.String(), "\n"), nil
}
