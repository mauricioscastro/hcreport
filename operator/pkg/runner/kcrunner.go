package runner

import (
	"strings"
)

const (
	cmdApiResources = "api-resources -o wide --sort-by=name --no-headers=true"
	cmdNs           = "get ns -o custom-columns=NAME:.metadata.name --sort-by=.metadata.name --no-headers=true"
)

type KcCmdRunner interface {
	CmdRunner
	GetApiResources() ([][]string, error)
	GetNS() ([]string, error)
}

func NewKcCmdRunner() KcCmdRunner {
	return &runner{}
}

func (r *runner) GetApiResources() ([][]string, error) {
	r.KcCmd(strings.Split(cmdApiResources, " ")).
		Sed("s/\\s+/ /g").
		Sed("s/,/;/g").
		Sed("s/ /,/g").
		// add shortname col where it's missing
		Sed("s/^([^\\s,]+,)((?:[^\\s,]+,){4})([^\\s,]+)*$/$1,$2$3/g")
	return r.Table(), r.Err()
}

func (r *runner) GetNS() ([]string, error) {
	r.KcCmd(strings.Split(cmdNs, " "))
	return r.List(), r.Err()
}
