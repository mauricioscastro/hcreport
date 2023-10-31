package runner

import (
	"bytes"
	"io/fs"
	"os"
	"regexp"
	"strings"

	"github.com/drone/envsubst"
	"github.com/mauricioscastro/hcreport/pkg/util/log"
	jqw "github.com/mauricioscastro/hcreport/pkg/wrapper/jq"
	kcw "github.com/mauricioscastro/hcreport/pkg/wrapper/kc"
	yqw "github.com/mauricioscastro/hcreport/pkg/wrapper/yq"
	"github.com/rwtodd/Go.Sed/sed"
)

var logger = log.Logger().Named("hcr.runner")

func SetLoggerLevel(level string) {
	logger = log.ResetLoggerLevel(logger, level)
}

type runner struct {
	pipe   bytes.Buffer
	err    error
	append bool
	jqw    jqw.JqWrapper
	kcw    kcw.KcWrapper
	yqw    yqw.YqWrapper
}

type CmdRunner interface {
	JqYqCmdRunner
	KcCmdRunner
	EnvSubst(arg string) CmdRunner
	Match(expr string) CmdRunner
	ChDir(arg string) CmdRunner
	MkDir(arg string) CmdRunner
	WriteFile(path string) CmdRunner
	ReadFile(path string) CmdRunner
	ReplaceAll(old string, new string) CmdRunner
	RegexReplaceAll(expr string, new string) CmdRunner
	Sed(expr string) CmdRunner
}

func NewCmdRunner() CmdRunner {
	return &runner{}
}

func NewCmdRunnerWithArgs(data []byte) CmdRunner {
	r := runner{}
	r.pipe.Write(data)
	return &r
}

func (r *runner) ReplaceAll(old string, new string) CmdRunner {
	if r.err == nil {
		r.write(strings.ReplaceAll(r.pipe.String(), old, new))
	}
	return r
}

func (r *runner) RegexReplaceAll(expr string, new string) CmdRunner {
	if r.err == nil {
		re, e := regexp.Compile(expr)
		if e != nil {
			r.error(e)
		} else {
			r.write(re.ReplaceAllString(r.pipe.String(), new))
		}
	}
	return r
}

func (r *runner) WriteFile(path string) CmdRunner {
	if r.err == nil {
		r.error(os.WriteFile(path, r.Bytes(), fs.ModePerm))
	}
	return r
}

func (r *runner) ReadFile(path string) CmdRunner {
	if r.err == nil {
		o, e := os.ReadFile(path)
		if e == nil {
			r.writeBytes(o)
		}
		r.error(e)
	}
	return r
}

func (r *runner) EnvSubst(arg string) CmdRunner {
	if r.err == nil {
		o, e := envsubst.EvalEnv(arg)
		if e == nil {
			r.write(o)
		}
		r.error(e)
	}
	return r
}

func (r *runner) ChDir(arg string) CmdRunner {
	if r.err == nil {
		r.err = os.Chdir(arg)
	}
	return r
}

func (r *runner) Match(pattern string) CmdRunner {
	if r.err == nil {
		var ml bytes.Buffer
		for _, l := range r.List() {
			if m, e := regexp.MatchString(pattern, l); e != nil {
				r.error(e)
				break
			} else if m {
				ml.WriteString(l)
			}
		}
		r.write(ml.String())
	}
	return r
}

func (r *runner) MkDir(arg string) CmdRunner {
	if r.err == nil {
		r.err = os.MkdirAll(arg, fs.ModePerm)
	}
	return r
}

func (r *runner) Sed(expr string) CmdRunner {
	if r.err == nil {
		s, e := sed.New(strings.NewReader(expr))
		if e == nil {
			o, e := s.RunString(r.pipe.String())
			if e == nil {
				// looks like sed pkg adds an extra
				// new line? I did not care to check
				r.write(strings.TrimSuffix(o, "\n"))
			}
		}
		r.error(e)
	}
	return r
}
