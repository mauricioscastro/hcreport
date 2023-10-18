package yq

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	"github.com/mauricioscastro/hcreport/pkg/util/log"
	yq "github.com/mikefarah/yq/v4/cmd"
	"go.uber.org/zap/zapcore"

	"github.com/spf13/cobra"
)

const yqDefaultArgs = "-M"

var logger = log.Logger().Named("hcr.yqw")

func SetLoggerLevel(level zapcore.Level) {
	logger = log.ResetLoggerLevel(logger, level)
}

type YqWrapper interface {
	Eval(args []string, expr string, yaml string, file ...string) (string, error)
	EvalEach(expr string, yaml string, file ...string) (string, error)
	EvalAll(expr string, yaml string, file ...string) (string, error)
	EvalEachInplace(expr string, file ...string) (string, error)
	EvalAllInplace(expr string, file ...string) (string, error)
	Split(expr string, fileNameExpr string, yaml string, path string) error
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
	return yqw.Eval([]string{"eval"}, expr, yaml, file...)
}

func (yqw *yqWrapper) EvalAll(expr string, yaml string, file ...string) (string, error) {
	return yqw.Eval([]string{"eval-all"}, expr, yaml, file...)
}

func (yqw *yqWrapper) EvalEachInplace(expr string, file ...string) (string, error) {
	return yqw.Eval([]string{"eval", "-i"}, expr, "", file...)
}

func (yqw *yqWrapper) EvalAllInplace(expr string, file ...string) (string, error) {
	return yqw.Eval([]string{"eval-all", "-i"}, expr, "", file...)
}

func (yqw *yqWrapper) Eval(args []string, expr string, yaml string, file ...string) (string, error) {
	if len(expr) != 0 {
		args = append(args, []string{"--expression", expr}...)
	}
	args = append(args, strings.Split(yqDefaultArgs, " ")...)
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

func (yqw *yqWrapper) Split(expr string, fileNameExpr string, yaml string, path string) error {
	oldPath, err := os.Getwd()
	if err == nil {
		err = os.Chdir(path)
		if err == nil {
			_, err = yqw.Eval([]string{"eval-all", "-s", fileNameExpr}, expr, yaml)
		}
		os.Chdir(oldPath)
	}
	return err
}

func (yqw *yqWrapper) ToJson(yaml string) (string, error) {
	return yqw.Eval([]string{"eval", "-oj"}, "", yaml)
}
