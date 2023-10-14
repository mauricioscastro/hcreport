package yq

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/mauricioscastro/hcreport/pkg/util/log"
	yq "github.com/mikefarah/yq/v4/cmd"

	"github.com/spf13/cobra"
)

const yqDefaultArgs = "-M"

var logger = log.Logger().Named("hcr.yqw")

type YqWrapper interface {
	EvalEach(expr string, yaml string, file ...string) (string, error)
	EvalAll(expr string, yaml string, file ...string) (string, error)
	EvalEachInplace(expr string, file ...string) (string, error)
	EvalAllInplace(expr string, file ...string) (string, error)
	ToJson(yaml string) (string, error)
}

type yqWrapper struct {
	out bytes.Buffer
	err bytes.Buffer
	cmd *cobra.Command
}

func NewYqWrapper() YqWrapper {
	yqw := yqWrapper{}
	yqw.cmd = yq.New()
	yqw.cmd.SetOut(&yqw.out)
	yqw.cmd.SetErr(&yqw.err)
	return &yqw
}

func (yqw *yqWrapper) EvalEach(expr string, yaml string, file ...string) (string, error) {
	return yqw.eval("eval", expr, yaml, file...)
}

func (yqw *yqWrapper) EvalAll(expr string, yaml string, file ...string) (string, error) {
	return yqw.eval("eval-all", expr, yaml, file...)
}

func (yqw *yqWrapper) EvalEachInplace(expr string, file ...string) (string, error) {
	return yqw.eval("eval -i", expr, "", file...)
}

func (yqw *yqWrapper) EvalAllInplace(expr string, file ...string) (string, error) {
	return yqw.eval("eval-all -i", expr, "", file...)
}

func (yqw *yqWrapper) eval(mainArgs string, expr string, yaml string, file ...string) (string, error) {
	args := strings.Split(mainArgs+" "+yqDefaultArgs, " ")
	if len(expr) != 0 {
		args = append(args, []string{"--expression", expr}...)
	}
	if len(yaml) > 0 {
		yqw.cmd.SetIn(strings.NewReader(yaml))
		args = append(args, "-")
	}
	args = append(args, file...)
	yqw.cmd.SetOut(&yqw.out)
	yqw.cmd.SetErr(&yqw.err)
	yqw.cmd.SetArgs(args)
	yqw.err.Reset()
	yqw.out.Reset()
	logger.Debug("run: " + strings.Join(args, " "))
	err := yqw.cmd.Execute()
	stderr := yqw.err.String()
	if err != nil && !strings.Contains(stderr, err.Error()) {
		err = fmt.Errorf("%s\n%s", err, yqw.err.String())
	}
	return strings.TrimSuffix(yqw.out.String(), "\n"), err
}

func (yqw *yqWrapper) ToJson(yaml string) (string, error) {
	return yqw.eval("eval -oj", "", yaml)
}
