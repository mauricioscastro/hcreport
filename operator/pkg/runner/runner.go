package runner

import (
	"bytes"
	// "github.com/mauricioscastro/hcreport/pkg/util/log"
)

// var logger = log.Logger().Named("hcr.runner")

type cmd func(args ...string) Runner

type Runner interface {
	Append(output string) Runner
	AppendCmd(c cmd, args ...string) Runner
	Kc(args ...string) Runner
	Out() string
	Err() error
	write(data string)
}

type runner struct {
	out    bytes.Buffer
	err    error
	append bool
}

func NewRunner() Runner {
	r := runner{}
	return &r
}

func (r *runner) Append(output string) Runner {
	if r.err == nil {
		r.append = true
		r.write(output)
	}
	return r
}

func (r *runner) AppendCmd(c cmd, args ...string) Runner {
	if r.err == nil {
		r.append = true
		return c(args...)
	}
	return r
}

func (r *runner) Kc(args ...string) Runner {
	if r.err == nil {
		// r.write(args[1])
	}
	return r
}

func (r *runner) write(data string) {
	if !r.append {
		r.out.Reset()
	}
	r.out.WriteString(data)
	r.append = false
}

func (r *runner) Out() string {
	return r.out.String()
}

func (r *runner) Err() error {
	return r.err
}
