// Package logger provides structured logging for AIRA using go.uber.org/zap.
package logger

import (
	"os"
	"sync"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	global *zap.Logger
	once   sync.Once
)

// Init initialises the global logger.  level must be one of: debug, info, warn, error.
// If pretty is true a human-friendly console encoder is used; otherwise JSON is written.
func Init(level string, pretty bool) {
	once.Do(func() {
		lvl := zapcore.InfoLevel
		if err := lvl.Set(level); err != nil {
			lvl = zapcore.InfoLevel
		}

		var enc zapcore.Encoder
		cfg := zap.NewProductionEncoderConfig()
		cfg.TimeKey = "ts"
		cfg.EncodeTime = zapcore.ISO8601TimeEncoder

		if pretty {
			cfg.EncodeLevel = zapcore.CapitalColorLevelEncoder
			enc = zapcore.NewConsoleEncoder(cfg)
		} else {
			cfg.EncodeLevel = zapcore.LowercaseLevelEncoder
			enc = zapcore.NewJSONEncoder(cfg)
		}

		core := zapcore.NewCore(enc, zapcore.AddSync(os.Stderr), lvl)
		global = zap.New(core, zap.AddCaller(), zap.AddCallerSkip(1))
	})
}

// L returns the global logger, initialising a default one if needed.
func L() *zap.Logger {
	if global == nil {
		Init("info", true)
	}
	return global
}

// Sugar returns the sugared global logger.
func Sugar() *zap.SugaredLogger { return L().Sugar() }

// Sync flushes any buffered log entries.
func Sync() { _ = L().Sync() }

// Debug logs at DEBUG level.
func Debug(msg string, fields ...zap.Field) { L().Debug(msg, fields...) }

// Info logs at INFO level.
func Info(msg string, fields ...zap.Field) { L().Info(msg, fields...) }

// Warn logs at WARN level.
func Warn(msg string, fields ...zap.Field) { L().Warn(msg, fields...) }

// Error logs at ERROR level.
func Error(msg string, fields ...zap.Field) { L().Error(msg, fields...) }

// Fatal logs at FATAL level then calls os.Exit(1).
func Fatal(msg string, fields ...zap.Field) { L().Fatal(msg, fields...) }
