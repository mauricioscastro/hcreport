package runner

import (
	"strings"

	kcw "github.com/mauricioscastro/hcreport/pkg/wrapper/kc"
)

const (
	cmdApiResources = "api-resources -o wide --sort-by=name --no-headers=true"
)

type KcCmdRunner interface {
	PipeCmdRunner
	Kc(cmdArgs string) CmdRunner
	KcApply() CmdRunner
	KcCmd(cmdArgs []string) CmdRunner
	ApiResources() ([][]string, error)
}

func NewKcCmdRunner() KcCmdRunner {
	return &runner{}
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

func (r *runner) Kc(cmdArgs string) CmdRunner {
	return r.KcCmd(strings.Split(cmdArgs+" -o yaml", " "))
}

func (r *runner) KcApply() CmdRunner {
	return r.Kc("apply -f -")
}

func (r *runner) ApiResources() ([][]string, error) {
	r.KcCmd(strings.Split(cmdApiResources, " ")).
		Sed("s/\\s+/ /g").
		Sed("s/,/;/g").
		Sed("s/ /,/g").
		// add shortname col where it's missing
		Sed("s/^([^\\s,]+,)((?:[^\\s,]+,){4})([^\\s,]+)*$/$1,$2$3/g")
	return r.Table(), r.Err()
}
