package runner

import (
	"errors"
	"slices"
	"strings"

	kcw "github.com/mauricioscastro/hcreport/pkg/wrapper/kc"
)

const (
	cmdApiResources = "api-resources -o wide --sort-by=name --no-headers=true"
	cmdNs           = "get ns -o custom-columns=NAME:.metadata.name --sort-by=.metadata.name --no-headers=true"
)

var (
	kcReadOnly   bool
	readOnlyCmds = []string{"get", "explain", "cluster-info", "top", "describe", "logs", "api-resources", "api-versions", "version"}
)

type KcCmdRunner interface {
	PipeCmdRunner
	Kc(cmdArgs string) CmdRunner
	KcApply() CmdRunner
	KcCmd(cmdArgs []string) CmdRunner
	KcApiResources() ([][]string, error)
	KcNs() ([]string, error)
}

func NewKcCmdRunner() KcCmdRunner {
	kcReadOnly = false
	return &runner{}
}

func (r *runner) KcCmd(cmdArgs []string) CmdRunner {
	if r.err == nil {
		if kcReadOnly && !slices.Contains(readOnlyCmds, cmdArgs[0]) {
			r.error(errors.New("trying to non read only command in readOnly mode"))
			return r
		}
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

func (r *runner) Kc(cmdArgs string) CmdRunner {
	return r.KcCmd(strings.Split(cmdArgs+" -o yaml", " "))
}

func (r *runner) KcApply() CmdRunner {
	return r.Kc("apply -f -")
}

func (r *runner) KcApiResources() ([][]string, error) {
	r.KcCmd(strings.Split(cmdApiResources, " ")).
		Sed("s/\\s+/ /g").
		ReplaceAll(",", ";").
		ReplaceAll(" ", ",").
		// add shortname col where it's missing
		Sed("s/^([^\\s,]+,)((?:[^\\s,]+,){4})([^\\s,]+)*$/$1,$2$3/g")
	return r.Table(), r.Err()
}

func (r *runner) KcNs() ([]string, error) {
	r.KcCmd(strings.Split(cmdNs, " "))
	return r.List(), r.Err()
}
