package runner

import (
	"github.com/goccy/go-json"
	"gopkg.in/yaml.v3"

	"github.com/mauricioscastro/hcreport/pkg/yjq"
)

type (
	YJqCmdRunner interface {
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
		UnMarshalYaml() map[string]interface{}
	}
)

func (r *runner) UnMarshalYaml() map[string]interface{} {
	if r.err == nil {
		var i map[string]interface{}
		e := yaml.Unmarshal(r.Bytes(), &i)
		if e == nil {
			return i
		}
		r.error(e)
	}
	return nil
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

// Eval yaml content with yq expression issuing result in yaml
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

// Eval json content with yq expression issuing result in yaml
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

// From json to yaml
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

// From yaml to compact json
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

// From yaml to pretty json
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

// Eval json content with jq expression issuing result in json compact
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

// / Eval json content with jq expression issuing result in json pretty
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
