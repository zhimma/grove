package logger

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/rs/zerolog"
)

func TestInitWritesFileAndCloseIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	if err := Init(Config{Level: "info", Path: dir, Service: "api"}); err != nil {
		t.Fatalf("init logger: %v", err)
	}
	Info().Msg("before close")
	if err := Close(); err != nil {
		t.Fatalf("close logger: %v", err)
	}
	if err := Close(); err != nil {
		t.Fatalf("close logger again: %v", err)
	}

	raw, err := os.ReadFile(filepath.Join(dir, "api.log"))
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	if !strings.Contains(string(raw), "before close") {
		t.Fatalf("missing log entry: %s", raw)
	}
}

func TestInitRotatesTheFileAtMaxSize(t *testing.T) {
	t.Cleanup(func() { _ = Close() })
	dir := t.TempDir()
	if err := Init(Config{Level: "info", Path: dir, Service: "api", MaxSizeMB: 1}); err != nil {
		t.Fatalf("init logger: %v", err)
	}
	line := strings.Repeat("x", 1024)
	for range 1100 {
		Info().Msg(line)
	}
	if err := Close(); err != nil {
		t.Fatalf("close logger: %v", err)
	}

	files, err := filepath.Glob(filepath.Join(dir, "api*.log"))
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(files) < 2 {
		t.Fatalf("expected a rotated file next to api.log, got %v", files)
	}
}

func TestInitFailsWhenTheLogDirectoryIsNotWritable(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions")
	}
	t.Cleanup(func() { _ = Close() })
	dir := filepath.Join(t.TempDir(), "logs")
	if err := os.Mkdir(dir, 0o500); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := Init(Config{Level: "info", Path: dir, Service: "api"}); err == nil {
		t.Fatal("expected init to fail on a read-only log directory")
	}
}

func TestLoggerAccessIsSafeDuringReinitialization(t *testing.T) {
	t.Cleanup(func() { _ = Close() })
	dir := t.TempDir()
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			_ = Init(Config{Level: "disabled", Path: dir, Service: "race"})
		}()
		go func() {
			defer wg.Done()
			globalLog := Logger()
			globalLog.Info().Msg("race")
			contextLog := FromContext(t.Context())
			contextLog.Info().Msg("race")
		}()
	}
	wg.Wait()
}

func TestInitForTestReplacesGlobalLogger(t *testing.T) {
	previousLevel := zerolog.GlobalLevel()
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	t.Cleanup(func() { zerolog.SetGlobalLevel(previousLevel) })
	previous := Logger()
	t.Cleanup(func() { InitForTest(previous) })

	var output strings.Builder
	InitForTest(zerolog.New(&output))
	Info().Msg("test logger")
	if !strings.Contains(output.String(), "test logger") {
		t.Fatalf("unexpected output: %q", output.String())
	}
}
