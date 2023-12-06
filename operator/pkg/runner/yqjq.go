package runner

import (
	"github.com/goccy/go-json"
	"gopkg.in/yaml.v3"

	"github.com/mauricioscastro/hcreport/pkg/yqjq/jq"
	"github.com/mauricioscastro/hcreport/pkg/yqjq/yq"
)

type JqYqCmdRunner interface {
	PipeCmdRunner
	Jq(expr string) CmdRunner
	JqPretty(expr string) CmdRunner
	Yq(expr string) CmdRunner
	YqJ(expr string) CmdRunner
	ToYaml() CmdRunner
	ToJson() CmdRunner
	ToJsonPretty() CmdRunner
	MarshalJson(i any) CmdRunner
	MarshalYaml(i any) CmdRunner
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

func (r *runner) Yq(expr string) CmdRunner {
	if r.err == nil {
		o, e := yq.Eval(expr, r.pipe.String())
		if e == nil {
			r.write(o)
		}
		r.error(e)
	}
	return r
}

func (r *runner) YqJ(expr string) CmdRunner {
	if r.err == nil {
		o, e := yq.EvalJY(expr, r.pipe.String())
		if e == nil {
			r.write(o)
		}
		r.error(e)
	}
	return r
}

func (r *runner) ToYaml() CmdRunner {
	if r.err == nil {
		o, e := yq.J2Y(r.pipe.String())
		if e == nil {
			r.write(o)
		}
		r.error(e)
	}
	return r
}

func (r *runner) ToJson() CmdRunner {
	if r.err == nil {
		o, e := yq.Y2JC(r.pipe.String())
		if e == nil {
			r.write(o)
		}
		r.error(e)
	}
	return r
}

func (r *runner) ToJsonPretty() CmdRunner {
	if r.err == nil {
		o, e := yq.Y2JP(r.pipe.String())
		if e == nil {
			r.write(o)
		}
		r.error(e)
	}
	return r
}

func (r *runner) Jq(expr string) CmdRunner {
	if r.err == nil {
		o, e := jq.Eval(expr, r.pipe.String())
		if e == nil {
			r.write(o)
		}
		r.error(e)
	}
	return r
}

func (r *runner) JqPretty(expr string) CmdRunner {
	return r.Jq(expr).ToJsonPretty()
}
