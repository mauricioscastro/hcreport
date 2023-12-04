package runner

import (
	"fmt"
	"slices"
	"strings"

	"github.com/mauricioscastro/hcreport/pkg/kc"
	"go.uber.org/zap"
)

var (
	kcReadOnly = false
)

type KcCmdRunner interface {
	PipeCmdRunner
	KcGet(api string) CmdRunner
	KcApiResources() CmdRunner
}

func (r *runner) KcGet(api string) CmdRunner {
	if r.err == nil {
		if r.kc == nil {
			r.kc = kc.NewKc()
		}
		o, e := r.kc.Get(api)
		if e == nil {
			r.write(o)
		}
		if e != nil {
			r.error(fmt.Errorf("problem getting %s. error: %s", api, e.Error()))
		}
	}
	return r
}

func (r *runner) KcApiResources() CmdRunner {
	version := r.KcGet("/api").Yq(".versions[-1]").String()
	if r.err != nil {
		return r
	}
	apiList := []string{"/api/" + version, "/apis/apps/" + version}
	otherApiListYq := `.groups[].preferredVersion.groupVersion | select(. != "apps/%s")`
	otherApis := r.KcGet("/apis").Yq(fmt.Sprintf(otherApiListYq, version)).Sed("s;^;/apis/;g").List()
	if r.err != nil {
		return r
	}
	slices.Sort(otherApis)
	apiList = append(apiList, otherApis...)
	apiRLYq := `[.resources[] | select(.singularName != "") | del(.storageVersionHash) | del (.singularName) | .groupVersion = "%s" | .available = "%s"]`
	apiRLHeaderYq := `{ "kind": "APIResourceList", "groupVersion": "%s", "resources": . }`
	apiYamlList := ""
	for _, api := range apiList {
		r.KcGet(api)
		gv := strings.TrimPrefix(api, "/api/")
		gv = strings.TrimPrefix(gv, "/apis/")
		if r.err == nil {
			apiYamlList += r.Yq(fmt.Sprintf(apiRLYq, gv, "true")).String() + "\n"
		} else {
			logger.Error("error getting api resource", zap.String("api", api), zap.Error(r.err))
			r.IgnoreError("503")
			badApiYq := `. + {"groupVersion": "%s", "available": "false"}`
			apiYamlList += r.Echo("[]").Yq(fmt.Sprintf(badApiYq, gv)).String() + "\n"
		}
	}
	return r.Echo(apiYamlList).Yq(fmt.Sprintf(apiRLHeaderYq, version))
}
