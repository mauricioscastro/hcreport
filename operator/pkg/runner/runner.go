package runner

import (
	"bytes"
	"encoding/csv"
	"strings"

	"github.com/drone/envsubst"
	"github.com/mauricioscastro/hcreport/pkg/util/log"
	jqw "github.com/mauricioscastro/hcreport/pkg/wrapper/jq"
	kcw "github.com/mauricioscastro/hcreport/pkg/wrapper/kc"
	yqw "github.com/mauricioscastro/hcreport/pkg/wrapper/yq"
	"github.com/rwtodd/Go.Sed/sed"
	"go.uber.org/zap/zapcore"
)

var logger = log.Logger().Named("hcr.runner")

func SetLoggerLevel(level zapcore.Level) {
	logger = log.ResetLoggerLevel(logger, level)
}

type CmdRunner interface {
	Append() CmdRunner
	EnvSubst(arg string) CmdRunner
	Echo(arg string) CmdRunner
	Kc(cmdArgs string) CmdRunner
	Jq(expr string) CmdRunner
	Yq(expr string) CmdRunner
	KcApply() CmdRunner
	KcCmd(cmdArgs []string) CmdRunner
	JqCmd(cmdArgs []string) CmdRunner
	YqEach(expr string) CmdRunner
	Sed(expr string) CmdRunner
	List() []string
	Table() [][]string
	Out() string
	Err() error
	write(data string)
	error(e error)
}

type runner struct {
	out    bytes.Buffer
	err    error
	append bool
	jqw    jqw.JqWrapper
	kcw    kcw.KcWrapper
	yqw    yqw.YqWrapper
}

func NewCmdRunner() CmdRunner {
	r := runner{}
	r.jqw = jqw.NewJqWrapper()
	r.kcw = kcw.NewKcWrapper()
	r.yqw = yqw.NewYqWrapper()
	return &r
}

func (r *runner) Append() CmdRunner {
	r.append = true
	return r
}

func (r *runner) EnvSubst(arg string) CmdRunner {
	if r.err == nil {
		o, e := envsubst.EvalEnv(arg)
		if e != nil {
			r.error(e)
		} else {
			r.write(o)
		}
	}
	return r
}

func (r *runner) Echo(arg string) CmdRunner {
	if r.err == nil {
		r.write(arg)
	}
	return r
}

func (r *runner) KcCmd(cmdArgs []string) CmdRunner {
	if r.err == nil {
		ret, err := r.kcw.Run(cmdArgs, r.out.String())
		if err != nil {
			r.error(err)
		} else {
			r.write(ret)
		}
	}
	return r
}

func (r *runner) YqEach(expr string) CmdRunner {
	if r.err == nil {
		ret, err := yqw.NewYqWrapper().EvalEach(expr, r.out.String())
		if err != nil {
			r.error(err)
		} else {
			r.write(ret)
		}
	}
	return r
}

func (r *runner) Yq(expr string) CmdRunner {
	if r.err == nil {
		ret, err := yqw.NewYqWrapper().EvalAll(expr, r.out.String())
		if err != nil {
			r.error(err)
		} else {
			r.write(ret)
		}
	}
	return r
}

func (r *runner) JqCmd(cmdArgs []string) CmdRunner {
	if r.err == nil {
		o, err := r.jqw.Run(cmdArgs, r.out.String())
		if err != nil {
			r.error(err)
		} else {
			r.write(o)
		}
	}
	return r
}

func (r *runner) Kc(cmdArgs string) CmdRunner {
	return r.KcCmd(strings.Split(cmdArgs+" -o yaml", " "))
}

func (r *runner) KcApply() CmdRunner {
	return r.Kc("apply -f -")
}

func (r *runner) Jq(expr string) CmdRunner {
	return r.JqCmd([]string{expr})
}

func (r *runner) Sed(expr string) CmdRunner {
	if r.err == nil {
		s, err := sed.New(strings.NewReader(expr))
		if err == nil {
			res, err := s.RunString(r.out.String())
			if err == nil {
				r.write(res)
			} else {
				r.error(err)
			}
		} else {
			r.error(err)
		}
	}
	return r
}

func (r *runner) List() []string {
	return strings.Split(r.out.String(), "\n")
}

func (r *runner) Table() [][]string {
	csv := csv.NewReader(strings.NewReader(r.out.String()))
	table, err := csv.ReadAll()
	if err != nil {
		r.error(err)
		return [][]string{}
	}
	return table
}

func (r *runner) write(data string) {
	if !r.append {
		r.out.Reset()
	}
	r.out.WriteString(data)
	r.append = false
}

func (r *runner) error(e error) {
	logger.Error(e.Error())
	r.err = e
}

func (r *runner) Out() string {
	return r.out.String()
}

func (r *runner) Err() error {
	return r.err
}
