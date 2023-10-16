package log

import (
	"fmt"
	"io"
	"os"

	"github.com/go-logr/zapr"
	"github.com/mikefarah/yq/v4/pkg/yqlib"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/op/go-logging.v1"
	"k8s.io/klog/v2"
)

var logger *zap.Logger
var loggerCfg zap.Config

func SilenceKcLogs() {
	zc := zap.NewProductionConfig()
	zc.OutputPaths = []string{os.DevNull}
	z, err := zc.Build()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	klog.SetLogger(zapr.NewLogger(z))
}

func SilenceYqLogs() {
	bke := logging.NewLogBackend(io.Discard, "", 0)
	bkel := logging.AddModuleLevel(bke)
	yqlib.GetLogger().SetBackend(bkel)
}

func Logger() *zap.Logger {
	if logger == nil {
		if conf := os.Getenv("LOGGER_ENV"); conf == "prod" {
			loggerCfg = zap.NewProductionConfig()
		} else {
			loggerCfg = zap.NewDevelopmentConfig()
		}
		if conf := os.Getenv("LOGGER_LEVEL"); len(conf) > 0 {
			l, e := zapcore.ParseLevel(conf)
			if e != nil {
				l = zapcore.InfoLevel
			}
			loggerCfg.Level.SetLevel(l)
		}
		loggerCfg.DisableCaller = true
		var err error
		if logger, err = loggerCfg.Build(); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	}
	return logger
}

func SetLoggerLevelInfo() {
	loggerCfg.Level.SetLevel(zapcore.InfoLevel)
}

func SetLoggerLevelDebug() {
	loggerCfg.Level.SetLevel(zapcore.DebugLevel)
}

func ResetLoggerLevel(log *zap.Logger, level zapcore.Level) *zap.Logger {
	return log.WithOptions(zap.WrapCore(
		func(zapcore.Core) zapcore.Core {
			sink, _, err := zap.Open(loggerCfg.OutputPaths...)
			if err != nil {
				logger.Error(err.Error())
			}
			return zapcore.NewCore(
				zapcore.NewConsoleEncoder(loggerCfg.EncoderConfig),
				sink,
				level)
		}))
}
