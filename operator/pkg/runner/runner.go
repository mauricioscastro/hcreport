package runner

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"reflect"
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

type CmdRunner interface {
	Append() CmdRunner
	EnvSubst(arg string) CmdRunner
	Echo(arg any) CmdRunner
	Match(expr string) CmdRunner
	ChDir(arg string) CmdRunner
	Kc(cmdArgs string) CmdRunner
	Jq(expr string) CmdRunner
	Yq(expr string) CmdRunner
	YqSplit(expr string, fileNameExpr string, path string) CmdRunner
	YqCreate(expr string) CmdRunner
	ToYaml() CmdRunner
	ToJson() CmdRunner
	ToJsonPretty() CmdRunner
	KcApply() CmdRunner
	KcCmd(cmdArgs []string) CmdRunner
	JqCmd(cmdArgs []string) CmdRunner
	YqEach(expr string) CmdRunner
	Sed(expr string) CmdRunner
	Empty() bool
	List() []string
	Table() [][]string
	Bytes() []byte
	Out() string
	Err() error
}

type runner struct {
	pipe   bytes.Buffer
	err    error
	append bool
	jqw    jqw.JqWrapper
	kcw    kcw.KcWrapper
	yqw    yqw.YqWrapper
}

func NewCmdRunner() CmdRunner {
	return &runner{}
}

func (r *runner) Append() CmdRunner {
	r.append = true
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

// arg accepts string, json.RawMessage, []byte
func (r *runner) Echo(arg any) CmdRunner {
	if r.err == nil {
		t := reflect.TypeOf(arg)
		v := reflect.ValueOf(arg)
		switch {
		case t.Kind() == reflect.String:
			r.write(v.Interface().(string))
		case t.String() == "json.RawMessage":
			r.write(string(v.Interface().(json.RawMessage)))
		case t.Kind() == reflect.Slice && t.Elem().Kind() == reflect.Uint8:
			r.write(string(v.Interface().([]uint8)))
		default:
			r.error(errors.New("unknown type passed to echo"))
		}
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

func (r *runner) MkDir(arg string, perm fs.FileMode) CmdRunner {
	if r.err == nil {
		r.err = os.MkdirAll(arg, perm)
	}
	return r
}

func (r *runner) KcCmd(cmdArgs []string) CmdRunner {
	if r.err == nil {
		if r.kcw == nil {
			r.kcw = kcw.NewKcWrapper()
		}
		o, e := r.kcw.Run(cmdArgs, r.pipe.String())
		if e == nil {
			r.write(o)
		}
		r.error(e)
	}
	return r
}

func (r *runner) YqEach(expr string) CmdRunner {
	if r.err == nil {
		r.yqInit()
		o, e := r.yqw.EvalEach(expr, r.pipe.String())
		if e == nil {
			r.write(o)
		}
		r.error(e)
	}
	return r
}

func (r *runner) Yq(expr string) CmdRunner {
	if r.err == nil {
		r.yqInit()
		o, e := r.yqw.EvalAll(expr, r.pipe.String())
		if e == nil {
			r.write(o)
		}
		r.error(e)
	}
	return r
}

func (r *runner) YqSplit(expr string, fileNameExpr string, path string) CmdRunner {
	if r.err == nil {
		r.yqInit()
		r.err = r.yqw.Split(expr, fileNameExpr, r.Out(), path)
	}
	return r
}

func (r *runner) YqCreate(expr string) CmdRunner {
	if r.err == nil {
		r.yqInit()
		o, e := r.yqw.Create(expr)
		if e == nil {
			r.write(o)
		}
		r.error(e)
	}
	return r
}
func (r *runner) ToYaml() CmdRunner {
	return r.JqCmd([]string{"--yaml-output"})
}

func (r *runner) ToJson() CmdRunner {
	return r.json(true)
}

func (r *runner) ToJsonPretty() CmdRunner {
	return r.json(false)
}

func (r *runner) JqCmd(cmdArgs []string) CmdRunner {
	if r.err == nil {
		if r.jqw == nil {
			r.jqw = jqw.NewJqWrapper()
		}
		o, e := r.jqw.Run(append(cmdArgs, "-M"), r.pipe.String())
		if e == nil {
			r.write(o)
		}
		r.error(e)
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
	return r.JqCmd([]string{"-c", expr})
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

func (r *runner) List() []string {
	return strings.Split(r.pipe.String(), "\n")
}

func (r *runner) Table() [][]string {
	csv := csv.NewReader(bytes.NewReader(r.pipe.Bytes()))
	table, err := csv.ReadAll()
	if err != nil {
		r.error(err)
		return [][]string{}
	}
	return table
}

func (r *runner) Empty() bool {
	return r.pipe.Len() == 0
}

func (r *runner) Bytes() []byte {
	return r.pipe.Bytes()
}

func (r *runner) Out() string {
	return r.pipe.String()
}

func (r *runner) Err() error {
	return r.err
}

func (r *runner) write(data string) {
	if !r.append {
		r.pipe.Reset()
	}
	r.pipe.WriteString(data)
	r.append = false
}

func (r *runner) error(e error) {
	if e != nil {
		logger.Error(e.Error())
	}
	r.err = e
}

func (r *runner) json(compact bool) CmdRunner {
	if r.err == nil {
		r.yqInit()
		o, e := r.yqw.ToJson(r.pipe.String())
		if e == nil {
			r.write(o)
			if compact {
				return r.JqCmd([]string{"-c"})
			}
		}
		r.error(e)
	}
	return r
}

func (r *runner) yqInit() {
	if r.yqw == nil {
		r.yqw = yqw.NewYqWrapper()
	}
}
