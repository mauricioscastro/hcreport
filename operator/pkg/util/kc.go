package util

import (
	"github.com/mauricioscastro/hcreport/pkg/runner"
)

const (
	cmdApiResources = "api-resources -o wide --sort-by=name --no-headers=true"
	cmdNs           = "get ns -o custom-columns=NAME:.metadata.name --sort-by=.metadata.name --no-headers=true"
)

func GetApiResources() ([][]string, error) {
	r := runner.NewCmdRunner().
		Kc(cmdApiResources).
		Sed("s/\\s+/ /g").
		Sed("s/,/;/g").
		Sed("s/ /,/g").
		// add shortname col where it's missing
		Sed("s/^([^\\s,]+,)((?:[^\\s,]+,){4})([^\\s,]+)*$/$1,$2$3/g")
	return r.Table(), r.Err()
}

func GetNS() ([]string, error) {
	r := runner.NewCmdRunner().Kc(cmdNs)
	return r.List(), r.Err()
}
