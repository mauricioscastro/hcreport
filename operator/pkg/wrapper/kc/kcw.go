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
	// "github.com/mauricioscastro/hcreport/pkg/wrapper/yq"
	// "github.com/rwtodd/Go.Sed/sed"
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

func SetLoggerLevel(level string) {
	logger = log.ResetLoggerLevel(logger, level)
}

type KcWrapper interface {
	Run(args []string, stdin string) (string, error)
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

func (kcw *kcWrapper) Run(args []string, stdin string) (string, error) {
	kcw.cmd.SetArgs(append(args, strings.Split(kcDefaultArgs, " ")...))
	kcw.out.Reset()
	kcw.cmd.SetOut(&kcw.out)
	kcw.err.Reset()
	kcw.cmd.SetErr(&kcw.err)
	kcw.in.Reset()
	feedStdin() // in case auth is requested, provoke auth error
	kcw.in.WriteString(stdin)
	var err error
	if kcw.sync {
		logger.Debug("run synced", zap.String("arg", strings.Join(args, " ")))
		err = kcw.runSynced()
	} else {
		logger.Debug("run", zap.String("arg", strings.Join(args, " ")))
		err = cli.RunNoErrOutput(kcw.cmd)
	}
	return strings.TrimSuffix(kcw.out.String(), "\n"), err
}

func (kcw *kcWrapper) runSynced() error {
	oOut := os.Stdout
	oErr := os.Stderr
	rErr, wErr, err := os.Pipe()
	if err != nil {
		return err
	} else {
		stdioLock.Lock()
		defer stdioLock.Unlock()
		os.Stdout, err = os.Open(os.DevNull)
		if err == nil {
			os.Stderr = wErr
			err = cli.RunNoErrOutput(kcw.cmd)
		}
	}
	os.Stdout = oOut
	os.Stderr = oErr
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
