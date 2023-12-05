package log

import (
	"fmt"
	"io"
	"strings"
	"time"

	zaplogfmt "github.com/jsternberg/zap-logfmt"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func NewRootLogger(format string, level string, w io.Writer) (*zap.Logger, error) {
	cfg := zap.NewProductionEncoderConfig()
	cfg.EncodeTime = func(ts time.Time, encoder zapcore.PrimitiveArrayEncoder) {
		encoder.AppendString(ts.UTC().Format("2006-01-02T15:04:05.000000Z07:00"))
	}
	cfg.LevelKey = "lvl"

	var enc zapcore.Encoder
	switch format {
	case "json":
		enc = zapcore.NewJSONEncoder(cfg)
	case "auto", "console":
		enc = zapcore.NewConsoleEncoder(cfg)
	case "logfmt":
		enc = zaplogfmt.NewEncoder(cfg)
	default:
		return nil, fmt.Errorf("unrecognized log format %q", format)
	}

	var lvl zapcore.Level
	switch strings.ToLower(level) {
	case "panic":
		lvl = zap.PanicLevel
	case "fatal":
		lvl = zap.FatalLevel
	case "error":
		lvl = zap.ErrorLevel
	case "warn", "warning":
		lvl = zap.WarnLevel
	case "info":
		lvl = zap.InfoLevel
	case "debug":
		lvl = zap.DebugLevel
	default:
		return nil, fmt.Errorf("unsupported log level: %s", level)
	}

	return zap.New(zapcore.NewCore(
		enc,
		zapcore.AddSync(w),
		lvl,
	)), nil
}
