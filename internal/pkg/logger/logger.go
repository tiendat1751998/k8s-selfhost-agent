// Package logger provides structured JSON logging using zap.
package logger

import (
	"context"
	"os"
	"sync"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type contextKey struct{}

var (
	global *zap.Logger
	once   sync.Once
)

// Init initializes the global logger with the specified level.
// Valid levels: "debug", "info", "warn", "error".
func Init(level string) {
	once.Do(func() {
		global = newLogger(level)
	})
}

// Get returns the global logger instance.
// If Init has not been called, it initializes with "info" level.
func Get() *zap.Logger {
	if global == nil {
		Init("info")
	}
	return global
}

// WithContext returns a logger from the context, or the global logger.
func WithContext(ctx context.Context) *zap.Logger {
	if l, ok := ctx.Value(contextKey{}).(*zap.Logger); ok {
		return l
	}
	return Get()
}

// ToContext stores a logger in the context.
func ToContext(ctx context.Context, l *zap.Logger) context.Context {
	return context.WithValue(ctx, contextKey{}, l)
}

// With returns a child logger with the given fields attached.
func With(fields ...zap.Field) *zap.Logger {
	return Get().With(fields...)
}

// Sync flushes any buffered log entries. Should be called before program exit.
func Sync() {
	if global != nil {
		_ = global.Sync()
	}
}

func newLogger(level string) *zap.Logger {
	lvl := parseLevel(level)

	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "timestamp",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		FunctionKey:    zapcore.OmitKey,
		MessageKey:     "message",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.MillisDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(encoderConfig),
		zapcore.AddSync(os.Stdout),
		lvl,
	)

	return zap.New(core, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))
}

func parseLevel(level string) zapcore.Level {
	switch level {
	case "debug":
		return zapcore.DebugLevel
	case "info":
		return zapcore.InfoLevel
	case "warn":
		return zapcore.WarnLevel
	case "error":
		return zapcore.ErrorLevel
	default:
		return zapcore.InfoLevel
	}
}
