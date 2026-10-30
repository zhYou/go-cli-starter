// Package logging 构造调用级 zap 日志，不接管外部输出流。
package logging

import (
	"fmt"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"io"
)

type writerSink struct{ io.Writer }

func (writerSink) Sync() error { return nil }
func New(out io.Writer, level string) (*zap.Logger, error) {
	var severity zapcore.Level
	switch level {
	case "debug":
		severity = zapcore.DebugLevel
	case "info":
		severity = zapcore.InfoLevel
	case "warn":
		severity = zapcore.WarnLevel
	case "error":
		severity = zapcore.ErrorLevel
	default:
		return nil, fmt.Errorf("日志级别 %q 无效", level)
	}
	if out == nil {
		out = io.Discard
	}
	cfg := zap.NewProductionEncoderConfig()
	cfg.TimeKey = ""
	cfg.EncodeLevel = zapcore.CapitalLevelEncoder
	return zap.New(zapcore.NewCore(zapcore.NewConsoleEncoder(cfg), zapcore.Lock(writerSink{out}), severity)), nil
}
