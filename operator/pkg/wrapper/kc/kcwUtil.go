package kc

import (
	"encoding/csv"
	"strings"
)

const (
	cmdApiResources = "api-resources -o wide --sort-by=name --no-headers=true"
	cmdNs           = "get ns -o custom-columns=NAME:.metadata.name --sort-by=.metadata.name --no-headers=true"
)

var kcw KcWrapper

// csv with:
// name,shortnames,groupversion,namespaced,kind,verbs,categories
// shortnames and categories (when they appear) separated by ';'
func GetApiResources() ([][]string, error) {
	initKubectl()
	var apiResources [][]string
	out, err := kcw.RunSed(cmdApiResources,
		"s/\\s+/ /g",
		"s/,/;/g",
		"s/ /,/g",
		// add shortname col where it's missing
		"s/^([^\\s,]+,)((?:[^\\s,]+,){4})([^\\s,]+)*$/$1,$2$3/g")
	if err == nil {
		r := csv.NewReader(strings.NewReader(out))
		apiResources, err = r.ReadAll()
	}
	return apiResources, err
}

func GetNS() ([]string, error) {
	initKubectl()
	ns, err := kcw.Run(cmdNs)
	return strings.Split(ns, "\n"), err
}

func Cmd() KcWrapper {
	initKubectl()
	return kcw
}

func initKubectl() {
	if kcw == nil {
		kcw = NewKcWrapper()
	}
}
