package kc

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	"github.com/spf13/cobra"

	"github.com/mauricioscastro/hcreport/pkg/util/log"
	"github.com/mauricioscastro/hcreport/pkg/wrapper/yq"
	"github.com/rwtodd/Go.Sed/sed"
	"go.uber.org/zap"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/genericiooptions"
	"k8s.io/component-base/cli"

	kubecmd "k8s.io/kubectl/pkg/cmd"
	cmdUtil "k8s.io/kubectl/pkg/cmd/util"
)

const kcDefaultArgs = "--insecure-skip-tls-verify"

var (
	stdioLock sync.Mutex
	logger    = log.Logger().Named("hcr.kcw")
)

type KcWrapper interface {
	Run(command string, stdin ...string) (string, error)
	RunSed(command string, e ...string) (string, error)
	RunYq(command string, e ...string) (string, error)
	UnSynced() KcWrapper
	Synced() KcWrapper
}

type kcWrapper struct {
	out  bytes.Buffer
	err  bytes.Buffer
	in   bytes.Buffer
	cmd  *cobra.Command
	sync bool
}

func NewKcWrapper() KcWrapper {
	kcw := kcWrapper{}
	kcw.cmd = kubecmd.NewDefaultKubectlCommandWithArgs(kubecmd.KubectlOptions{
		PluginHandler: nil,
		Arguments:     nil,
		ConfigFlags:   genericclioptions.NewConfigFlags(true).WithDeprecatedPasswordFlag().WithDiscoveryBurst(300).WithDiscoveryQPS(50.0),
		IOStreams:     genericiooptions.IOStreams{In: &kcw.in, Out: &kcw.out, ErrOut: &kcw.err},
	})
	kcw.cmd.SetErr(&kcw.err)
	kcw.cmd.SetOut(&kcw.err)
	kcw.cmd.SetIn(&kcw.in)
	kcw.sync = true
	cmdUtil.BehaviorOnFatal(func(msg string, code int) {
		if len(msg) > 0 {
			fmt.Fprint(&kcw.err, msg)
		}
	})
	return &kcw
}

func (kcw *kcWrapper) Run(command string, stdin ...string) (string, error) {
	argString := kcDefaultArgs + " " + command
	kcw.cmd.SetArgs(strings.Split(argString, " "))
	kcw.out.Reset()
	kcw.err.Reset()
	kcw.in.Reset()
	feedStdin() // in case auth is requested, provoke auth error
	deployment := strings.Join(stdin, "\n---\n")
	fmt.Fprint(&kcw.in, deployment)
	logger.Debug("", zap.String("deployment", deployment))
	var err error
	if kcw.sync {
		logger.Debug("run synced", zap.String("arg", argString))
		err = kcw.runSynced()
	} else {
		logger.Debug("run", zap.String("arg", argString)) //"run: " + argString)
		err = cli.RunNoErrOutput(kcw.cmd)
	}
	return strings.TrimSuffix(kcw.out.String(), "\n"), err
}

func (kcw *kcWrapper) RunSed(command string, expr ...string) (string, error) {
	out, err := kcw.Run(command)
	if err == nil {
		var engine *sed.Engine
		for _, e := range expr {
			if engine, err = sed.New(strings.NewReader(e)); err != nil {
				break
			}
			if out, err = engine.RunString(out); err != nil {
				break
			}
		}
	}
	return out, err
}

func (kcw *kcWrapper) RunYq(command string, expr ...string) (string, error) {
	out, err := kcw.Run(command + " -o yaml")
	if err == nil {
		yqw := yq.NewYqWrapper()
		for _, e := range expr {
			if out, err = yqw.EvalAll(e, out); err != nil {
				break
			}
		}
	}
	return out, err
}

func (kcw *kcWrapper) runSynced() error {
	// redirecting in cases where not connected to server
	// for problems even before cmd runs
	// cmd out and err should go to the buffers
	// should not happen since this is supposed to be
	// used from inside the cluster
	oOut := os.Stdout
	oErr := os.Stderr
	rErr, wErr, err := os.Pipe()
	if err != nil {
		return err
	} else {
		stdioLock.Lock()
		os.Stdout, err = os.Open(os.DevNull)
		if err == nil {
			os.Stderr = wErr
			err = cli.RunNoErrOutput(kcw.cmd)
		}
	}
	os.Stdout = oOut
	os.Stderr = oErr
	stdioLock.Unlock()
	wErr.Close()
	io.Copy(&kcw.err, rErr)
	if len(kcw.err.Bytes()) > 0 {
		if err != nil {
			fmt.Fprintf(&kcw.err, "\n%s", err)
		}
		err = fmt.Errorf("%s", kcw.err.String())
	}
	return err
}

func feedStdin() {
	ri, wi, err := os.Pipe()
	if err == nil {
		os.Stdin = ri
		wi.Write([]byte("0\r0\r"))
		wi.Close()
	}
}

func (kcw *kcWrapper) UnSynced() KcWrapper {
	kcw.sync = false
	return kcw
}

func (kcw *kcWrapper) Synced() KcWrapper {
	kcw.sync = true
	return kcw
}
