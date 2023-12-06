package runner

import (
	"fmt"
	"slices"
	"strings"

	"github.com/mauricioscastro/hcreport/pkg/kc"
	"go.uber.org/zap"
)

type KcCmdRunner interface {
	PipeCmdRunner
	KcGet(api string) CmdRunner
	KcApiResources() CmdRunner
	KcNs() CmdRunner
	initKc()
}

type kcRunner struct {
	kcReadOnly bool
	kcVersion  string
	kc         kc.Kc
}

func (r *runner) KcGet(api string) CmdRunner {
	r.initKc()
	if r.err == nil {
		o, e := r.kc.Get(api)
		if e == nil {
			r.write(o)
		}
		r.error(e, "KcGet api="+api)
	}
	return r
}

func (r *runner) KcApiResources() CmdRunner {
	r.initKc()
	apiList := []string{"/api/" + r.kcVersion, "/apis/apps/" + r.kcVersion}
	otherApiListYq := `.groups[].preferredVersion.groupVersion | select(. != "apps/%s")`
	otherApis := r.KcGet("/apis").Yq(fmt.Sprintf(otherApiListYq, r.kcVersion)).Sed("s;^;/apis/;g").List()
	if r.err != nil {
		return r
	}
	slices.Sort(otherApis)
	apiList = append(apiList, otherApis...)
	apiRLYq := `[.resources[] | select(.singularName != "") | del(.storageVersionHash) | del (.singularName) | .groupVersion = "%s" | .available = "true"]`
	apiRLHeaderYq := `{ "kind": "APIResourceList", "groupVersion": "%s", "resources": . }`
	apiYamlList := ""
	for _, api := range apiList {
		r.KcGet(api)
		gv := strings.TrimPrefix(api, "/api/")
		gv = strings.TrimPrefix(gv, "/apis/")
		if r.err == nil {
			apiYamlList += r.Yq(fmt.Sprintf(apiRLYq, gv)).String() + "\n"
		} else {
			logger.Error("error getting api resource", zap.String("api", api), zap.Error(r.err))
			r.IgnoreError()
			badApiYq := `. + {"groupVersion": "%s", "available": "false"}`
			apiYamlList += r.Echo("[]").Yq(fmt.Sprintf(badApiYq, gv)).String() + "\n"
		}
	}
	return r.Echo(apiYamlList).Yq(fmt.Sprintf(apiRLHeaderYq, r.kcVersion))
}

func (r *runner) KcNs() CmdRunner {
	r.initKc()
	r.KcGet("/api/" + r.kcVersion + "/namespaces").Yq(`del(.metadata) | with(.items[].metadata; del(.uid) | del(.resourceVersion) | del(.creationTimestamp) | del(.annotations["kubectl.kubernetes.io/last-applied-configuration"]) | del(.managedFields))`)
	return r
}

func (r *runner) initKc() {
	if r.kc == nil {
		r.kc = kc.NewKc()
		r.kcReadOnly = false
		r.kcVersion = r.KcGet("/api").Yq(".versions[-1]").String()
		logger.Debug("kcVersion=" + r.kcVersion)
	}
}
