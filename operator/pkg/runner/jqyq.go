package runner

import (
	"encoding/json"

	"gopkg.in/yaml.v3"

	jqw "github.com/mauricioscastro/hcreport/pkg/wrapper/jq"
	yqw "github.com/mauricioscastro/hcreport/pkg/wrapper/yq"
)

type JqYqCmdRunner interface {
	PipeCmdRunner
	Jq(expr string) CmdRunner
	Yq(expr string) CmdRunner
	YqSplit(expr string, fileNameExpr string, path string) CmdRunner
	YqCreate(expr string) CmdRunner
	ToYaml() CmdRunner
	ToJson() CmdRunner
	MarshalJson(i any) CmdRunner
	MarshalYaml(i any) CmdRunner
	ToJsonPretty() CmdRunner
	JqCmd(cmdArgs []string) CmdRunner
	YqEach(expr string) CmdRunner
}

func NewJqYqCmdRunner() JqYqCmdRunner {
	return &runner{}
}

func (r *runner) MarshalYaml(i any) CmdRunner {
	if r.err == nil {
		o, e := yaml.Marshal(i)
		if e == nil {
			r.write(string(o))
		}
		r.error(e)
	}
	return r
}

func (r *runner) MarshalJson(i any) CmdRunner {
	if r.err == nil {
		o, e := json.Marshal(i)
		if e == nil {
			r.write(string(o))
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

func (r *runner) Jq(expr string) CmdRunner {
	return r.JqCmd([]string{"-c", expr})
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
