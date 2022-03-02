package logger

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func NewLogger(level string, encoding string, caller string) (*zap.Logger, error) {
	lvl := zap.NewAtomicLevelAt(zapcore.InfoLevel)
	development := false
	switch level {
	case "debug":
		lvl = zap.NewAtomicLevelAt(zapcore.DebugLevel)
		development = true
	case "info":
		lvl = zap.NewAtomicLevelAt(zapcore.InfoLevel)
	case "warn":
		lvl = zap.NewAtomicLevelAt(zapcore.WarnLevel)
	case "error":
		lvl = zap.NewAtomicLevelAt(zapcore.ErrorLevel)
	case "fatal":
		lvl = zap.NewAtomicLevelAt(zapcore.FatalLevel)
	case "panic":
		lvl = zap.NewAtomicLevelAt(zapcore.PanicLevel)
	}
	encodeCaller := zapcore.FullCallerEncoder
	disableCaller := false
	switch caller {
	case "short":
		encodeCaller = zapcore.ShortCallerEncoder
	case "disable":
		disableCaller = true
	}
	zapEncoderConfig := zapcore.EncoderConfig{
		MessageKey:    "msg",
		LevelKey:      "lvl",
		TimeKey:       "ts",
		NameKey:       "logger",
		CallerKey:     "caller",
		StacktraceKey: "stacktrace",
		LineEnding:    zapcore.DefaultLineEnding,
		EncodeLevel:   zapcore.CapitalLevelEncoder,
		EncodeTime:    zapcore.ISO8601TimeEncoder,
		//EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeDuration: zapcore.StringDurationEncoder,
		EncodeCaller:   encodeCaller,
	}

	zapConfig := zap.Config{
		Level:             lvl,
		Development:       development,
		DisableCaller:     disableCaller,
		DisableStacktrace: true,
		Sampling: &zap.SamplingConfig{
			Initial:    100,
			Thereafter: 100,
		},
		Encoding:         encoding,
		EncoderConfig:    zapEncoderConfig,
		OutputPaths:      []string{"stderr"},
		ErrorOutputPaths: []string{"stderr"},
	}

	return zapConfig.Build()
}
