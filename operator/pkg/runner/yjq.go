package runner

import (
	"github.com/goccy/go-json"
	"gopkg.in/yaml.v3"

	"github.com/mauricioscastro/hcreport/pkg/yjq"
)

type JqYqCmdRunner interface {
	PipeCmdRunner
	Jq(expr string) CmdRunner
	JqPretty(expr string) CmdRunner
	Yq(expr string) CmdRunner
	YqJ2Y(expr string) CmdRunner
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

// eval yaml content with yq
func (r *runner) Yq(expr string) CmdRunner {
	if r.err == nil {
		o, e := yjq.YqEval(expr, r.pipe.String())
		if e == nil {
			r.write(o)
		}
		r.error(e)
	}
	return r
}

// eval json with yq to yaml
func (r *runner) YqJ2Y(expr string) CmdRunner {
	if r.err == nil {
		o, e := yjq.YqEvalJ2Y(expr, r.pipe.String())
		if e == nil {
			r.write(o)
		}
		r.error(e)
	}
	return r
}

// from json
func (r *runner) ToYaml() CmdRunner {
	if r.err == nil {
		o, e := yjq.J2Y(r.pipe.String())
		if e == nil {
			r.write(o)
		}
		r.error(e)
	}
	return r
}

// from yaml
func (r *runner) ToJson() CmdRunner {
	if r.err == nil {
		o, e := yjq.Y2JC(r.pipe.String())
		if e == nil {
			r.write(o)
		}
		r.error(e)
	}
	return r
}

// yaml input to json pretty
func (r *runner) ToJsonPretty() CmdRunner {
	if r.err == nil {
		o, e := yjq.Y2JP(r.pipe.String())
		if e == nil {
			r.write(o)
		}
		r.error(e)
	}
	return r
}

// json input to json compact
func (r *runner) Jq(expr string) CmdRunner {
	if r.err == nil {
		o, e := yjq.JqEval(expr, r.pipe.String())
		if e == nil {
			r.write(o)
		}
		r.error(e)
	}
	return r
}

// json input to json pretty
func (r *runner) JqPretty(expr string) CmdRunner {
	j := r.Jq(expr).String()
	if r.err == nil {
		o, e := yjq.J2JP(j)
		if e == nil {
			r.write(o)
		}
		r.error(e)
	}
	return r
}
