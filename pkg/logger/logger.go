package logger

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

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

type managedWriter struct {
	mu     sync.Mutex
	writer io.WriteCloser
	closed bool
}

func (w *managedWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return 0, os.ErrClosed
	}
	return w.writer.Write(p)
}

func (w *managedWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return nil
	}
	w.closed = true
	return w.writer.Close()
}

var globalLogger = struct {
	sync.RWMutex
	log    zerolog.Logger
	writer io.Closer
}{
	log: zerolog.New(os.Stdout).With().Timestamp().Logger(),
}

func Init(cfg Config) error {
	level, err := zerolog.ParseLevel(strings.ToLower(cfg.Level))
	if err != nil {
		level = zerolog.InfoLevel
	}
	// Process-wide on purpose: Grove owns its process, and this is the only
	// place log.level is applied. Removing it disables level configuration.
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
		fileWriter := &managedWriter{writer: file}
		writers = append(writers, fileWriter)
		newLogger := zerolog.New(io.MultiWriter(writers...)).With().Timestamp().Str("service", cfg.Service).Logger()
		if err := replace(newLogger, fileWriter); err != nil {
			_ = Close()
			return err
		}
		return nil
	}
	if len(writers) == 0 {
		writers = append(writers, os.Stdout)
	}
	newLogger := zerolog.New(io.MultiWriter(writers...)).With().Timestamp().Str("service", cfg.Service).Logger()
	if err := replace(newLogger, nil); err != nil {
		_ = Close()
		return err
	}
	return nil
}

func Close() error {
	log := zerolog.New(os.Stdout).With().Timestamp().Logger()
	return replace(log, nil)
}

func replace(log zerolog.Logger, writer io.Closer) error {
	globalLogger.Lock()
	previousWriter := globalLogger.writer
	globalLogger.log = log
	globalLogger.writer = writer
	globalLogger.Unlock()
	if previousWriter != nil {
		return previousWriter.Close()
	}
	return nil
}

func current() zerolog.Logger {
	globalLogger.RLock()
	log := globalLogger.log
	globalLogger.RUnlock()
	return log
}

func WithContext(ctx context.Context, log zerolog.Logger) context.Context {
	return context.WithValue(ctx, loggerKey, log)
}

func FromContext(ctx context.Context) zerolog.Logger {
	if ctx != nil {
		if log, ok := ctx.Value(loggerKey).(zerolog.Logger); ok {
			return log
		}
	}
	return current()
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
	return current()
}

func InitForTest(log zerolog.Logger) {
	globalLogger.Lock()
	globalLogger.log = log
	globalLogger.Unlock()
}

func Debug() *zerolog.Event {
	log := current()
	return log.Debug()
}

func Info() *zerolog.Event {
	log := current()
	return log.Info()
}

func Warn() *zerolog.Event {
	log := current()
	return log.Warn()
}

func Error() *zerolog.Event {
	log := current()
	return log.Error()
}

func Fatal() *zerolog.Event {
	log := current()
	return log.Fatal()
}
