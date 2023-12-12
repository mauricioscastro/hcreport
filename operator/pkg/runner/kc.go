package runner

import (
	"strings"
	"sync"
	"time"

	"github.com/panjf2000/ants/v2"

	"github.com/mauricioscastro/hcreport/pkg/kc"
	"github.com/mauricioscastro/hcreport/pkg/yjq"
)

type KcCmdRunner interface {
	PipeCmdRunner
	KcVersion() string
	KcGet(api string) CmdRunner
	KcGetAsync(apis []string, separator string, poolSize int) CmdRunner
	KcApiResources() CmdRunner
	KcNs() CmdRunner
	initKc()
	workerPool(size int) *ants.Pool
}

// kube client get resource
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

func (r *runner) KcGetAsync(apis []string, separator string, poolSize int) CmdRunner {
	if r.err != nil {
		return r
	}
	pool := r.workerPool(poolSize)
	if pool == nil {
		return r
	}
	defer pool.ReleaseTimeout(5 * time.Second)
	var wg sync.WaitGroup
	kcList := make([]kc.Kc, len(apis))
	for i, api := range apis {
		_kc := kc.NewKc()
		kcList[i] = _kc
		pool.Submit(func() {
			_kc.GetAsync(api, &wg)
		})
	}
	wg.Wait()
	var sb strings.Builder
	for i, v := range kcList {
		sb.WriteString(v.Response())
		if len(separator) > 0 && i+1 < len(kcList) {
			sb.WriteString(separator)
		}
	}
	return r.Echo(sb.String())
}

func (r *runner) KcApiResources() CmdRunner {
	if r.err != nil {
		return r
	}
	wr := NewCmdRunner()
	apiList :=
		wr.KcGet("/apis").
			Yq(`.groups[].preferredVersion.groupVersion`).
			Sed("s;^;/apis/;g").
			Append().Echo("\n/api/" + wr.KcVersion()).
			List()
	if r.err = wr.Err(); r.err != nil {
		return r
	}
	pool := r.workerPool(-1)
	if pool == nil {
		return r
	}
	defer pool.ReleaseTimeout(5 * time.Second)
	var wg sync.WaitGroup
	kcList := make([]kc.Kc, len(apiList))
	for i, api := range apiList {
		_kc := kc.NewKc()
		kcList[i] = _kc
		pool.Submit(func() {
			_kc.GetTransformAsync(api, &wg, func(kcApi string, kcResp string, kcErr error) (string, error) {
				if kcErr == nil {
					expr := `[.resources[] += {"groupVersion": .groupVersion} | .resources[] += {"available": true} | .resources[] | select(.singularName != "") | del(.storageVersionHash)]`
					kcResp, kcErr = yjq.YqEval(expr, kcResp)
					if kcErr == nil && kcResp != "[]" {
						return kcResp, nil
					}
				}
				kcResp = "- groupVersion: " + strings.TrimPrefix(kcApi, "/apis/") + "\n  available: false"
				return kcResp, nil
			})
		})
	}
	wg.Wait()
	var sb strings.Builder
	for i, _kc := range kcList {
		sb.WriteString(_kc.Response())
		if i+1 < len(kcList) {
			sb.WriteString("\n")
		}
	}
	return r.Echo("kind: APIResourceList\nresources:\n").Append().Echo(sb.String())
}

func (r *runner) KcNs() CmdRunner {
	if r.err != nil {
		return r
	}
	wr := NewCmdRunner()
	return r.Copy(wr.KcGet("/api/" + wr.KcVersion() + "/namespaces").
		Yq(`del(.metadata) | with(.items[].metadata; del(.uid) | del(.resourceVersion) | del(.creationTimestamp) | del(.annotations["kubectl.kubernetes.io/last-applied-configuration"]) | del(.managedFields))`))
}

func (r *runner) KcVersion() string {
	r.initKc()
	return r.kc.Version()
}

func (r *runner) initKc() {
	if r.kc == nil {
		r.kc = kc.NewKc()
		logger.Debug("kcVersion=" + r.kc.Version())
	}
}

func (r *runner) workerPool(size int) *ants.Pool {
	pool, err := ants.NewPool(size)
	if err != nil {
		r.error(err)
		return nil
	}
	return pool
}
