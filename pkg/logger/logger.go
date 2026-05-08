package logger

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/rs/zerolog"
)

type Config struct {
	Level   string
	Path    string
	Service string
	Console bool
}

type contextKey string

const (
	loggerKey contextKey = "logger"
	ridKey    contextKey = "request_id"
)

var appLogger = zerolog.New(os.Stdout).With().Timestamp().Logger()

func Init(cfg Config) error {
	level, err := zerolog.ParseLevel(strings.ToLower(cfg.Level))
	if err != nil {
		level = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(level)

	var writers []io.Writer
	if cfg.Console || cfg.Path == "" {
		writers = append(writers, zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: "2006-01-02 15:04:05"})
	}
	if cfg.Path != "" {
		if err := os.MkdirAll(cfg.Path, 0o750); err != nil {
			return err
		}
		logFile := filepath.Join(cfg.Path, cfg.Service+".log")
		file, err := os.OpenFile(filepath.Clean(logFile), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
		if err != nil {
			return err
		}
		writers = append(writers, file)
	}
	if len(writers) == 0 {
		writers = append(writers, os.Stdout)
	}
	appLogger = zerolog.New(io.MultiWriter(writers...)).With().Timestamp().Str("service", cfg.Service).Logger()
	return nil
}

func WithContext(ctx context.Context, log zerolog.Logger) context.Context {
	return context.WithValue(ctx, loggerKey, log)
}

func FromContext(ctx context.Context) zerolog.Logger {
	if log, ok := ctx.Value(loggerKey).(zerolog.Logger); ok {
		return log
	}
	return appLogger
}

func WithRID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, ridKey, requestID)
}

func RIDFromContext(ctx context.Context) string {
	if requestID, ok := ctx.Value(ridKey).(string); ok {
		return requestID
	}
	return ""
}

func Logger() zerolog.Logger {
	return appLogger
}

func InitForTest(log zerolog.Logger) {
	appLogger = log
}

func Debug() *zerolog.Event {
	return appLogger.Debug()
}

func Info() *zerolog.Event {
	return appLogger.Info()
}

func Warn() *zerolog.Event {
	return appLogger.Warn()
}

func Error() *zerolog.Event {
	return appLogger.Error()
}

func Fatal() *zerolog.Event {
	return appLogger.Fatal()
}
